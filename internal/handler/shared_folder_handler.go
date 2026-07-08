package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloud-storage-backend/internal/middleware"
	"cloud-storage-backend/internal/models"
	"cloud-storage-backend/internal/repository"
	"cloud-storage-backend/internal/response"
)

type SharedFolderHandler struct {
	sharedFolderRepo *repository.SharedFolderRepository
	folderRepo       *repository.FolderRepository
	userRepo         *repository.UserRepository
}

func NewSharedFolderHandler(
	sharedFolderRepo *repository.SharedFolderRepository,
	folderRepo *repository.FolderRepository,
	userRepo *repository.UserRepository,
) *SharedFolderHandler {
	return &SharedFolderHandler{
		sharedFolderRepo: sharedFolderRepo,
		folderRepo:       folderRepo,
		userRepo:         userRepo,
	}
}

func (h *SharedFolderHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "ไม่อนุญาตให้ใช้ Method อื่น")
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "เข้าสู่ระบบก่อนใช้งาน")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

	var req models.SharedFolderAddUserRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "ข้อมูลที่ส่งมาไม่ถูกต้อง")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Permission = strings.TrimSpace(strings.ToLower(req.Permission))

	if req.FolderID <= 0 {
		response.Error(w, http.StatusBadRequest, "รหัสโฟลเดอร์ไม่ถูกต้อง")
		return
	}

	if req.Email == "" || !strings.Contains(req.Email, "@") {
		response.Error(w, http.StatusBadRequest, "อีเมลผู้ใช้ที่ต้องการแชร์ไม่ถูกต้อง")
		return
	}

	if req.Permission != models.SharedPermissionViewer && req.Permission != models.SharedPermissionEditor {
		response.Error(w, http.StatusBadRequest, "สิทธิ์ต้องเป็น viewer หรือ editor")
		return
	}

	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now()) {
		response.Error(w, http.StatusBadRequest, "วันหมดอายุต้องเป็นเวลาในอนาคต")
		return
	}

	if _, err := h.folderRepo.FindByIDAndUserID(r.Context(), req.FolderID, claims.UserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "ไม่พบโฟลเดอร์ที่ต้องการแชร์")
			return
		}

		response.Error(w, http.StatusInternalServerError, "ไม่สามารถตรวจสอบโฟลเดอร์ได้")
		return
	}

	sharedWithUser, err := h.userRepo.FindByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "ไม่พบผู้ใช้จากอีเมลนี้")
			return
		}

		response.Error(w, http.StatusInternalServerError, "ไม่สามารถตรวจสอบผู้ใช้ที่ต้องการแชร์ได้")
		return
	}

	if sharedWithUser.ID == claims.UserID {
		response.Error(w, http.StatusBadRequest, "ไม่สามารถแชร์โฟลเดอร์ให้ตัวเองได้")
		return
	}

	sharedFolder := &models.SharedFolder{
		FolderID:         req.FolderID,
		SharedByUserID:   claims.UserID,
		SharedWithUserID: sharedWithUser.ID,
		Permission:       req.Permission,
		ExpiresAt:        req.ExpiresAt,
	}

	if err := h.sharedFolderRepo.Create(r.Context(), sharedFolder); err != nil {
		response.Error(w, http.StatusInternalServerError, "ไม่สามารถแชร์โฟลเดอร์ได้")
		return
	}

	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"message":       "แชร์โฟลเดอร์สำเร็จ",
		"shared_folder": models.NewSharedFolderResponse(sharedFolder),
	})
}

func (h *SharedFolderHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "ไม่อนุญาตให้ใช้ Method อื่น")
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "เข้าสู่ระบบก่อนใช้งาน")
		return
	}

	folders, err := h.sharedFolderRepo.FindSharedFoldersByUserID(r.Context(), claims.UserID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "ไม่สามารถโหลดรายการโฟลเดอร์ที่แชร์ได้")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "โหลดรายการโฟลเดอร์ที่แชร์สำเร็จ",
		"folders": folders,
		"total":   len(folders),
	})
}

func (h *SharedFolderHandler) ListUserPermissionInFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "ไม่อนุญาตให้ใช้ Method อื่น")
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "เข้าสู่ระบบก่อนใช้งาน")
		return
	}

	folderID, err := strconv.ParseInt(r.URL.Query().Get("folder_id"), 10, 64)
	if err != nil || folderID <= 0 {
		response.Error(w, http.StatusBadRequest, "รหัสโฟลเดอร์ไม่ถูกต้อง")
		return
	}

	permissions, err := h.sharedFolderRepo.FindAllUserPermissionInFolder(
		r.Context(),
		folderID,
		claims.UserID,
	)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "ไม่สามารถโหลดรายการสิทธิ์การแชร์โฟลเดอร์ได้")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message":     "โหลดรายการสิทธิ์สำเร็จ",
		"permissions": permissions,
		"total":       len(permissions),
	})
}

func (h *SharedFolderHandler) UpdatePermissionUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		response.Error(w, http.StatusMethodNotAllowed, "ไม่อนุญาตให้ใช้ Method อื่น")
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "เข้าสู่ระบบก่อนใช้งาน")
		return
	}

	var req models.SharedFolderUpdatePermissionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "ข้อมูลที่ส่งมาไม่ถูกต้อง")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Permission = strings.TrimSpace(strings.ToLower(req.Permission))

	if req.FolderID <= 0 {
		response.Error(w, http.StatusBadRequest, "รหัสโฟลเดอร์ไม่ถูกต้อง")
		return
	}

	if req.Email == "" || !strings.Contains(req.Email, "@") {
		response.Error(w, http.StatusBadRequest, "อีเมลผู้ใช้ที่ต้องการแชร์ไม่ถูกต้อง")
		return
	}

	if req.Permission != models.SharedPermissionViewer && req.Permission != models.SharedPermissionEditor {
		response.Error(w, http.StatusBadRequest, "สิทธิ์ต้องเป็น viewer หรือ editor")
		return
	}

	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now()) {
		response.Error(w, http.StatusBadRequest, "วันหมดอายุต้องเป็นเวลาในอนาคต")
		return
	}

	sharedWithUser, err := h.userRepo.FindByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "ไม่พบผู้ใช้จากอีเมลนี้")
			return
		}

		response.Error(w, http.StatusInternalServerError, "ไม่สามารถตรวจสอบผู้ใช้ที่ต้องการแชร์ได้")
		return
	}

	updated, err := h.sharedFolderRepo.UpdatePermissionUserFolder(
		r.Context(),
		&models.SharedFolder{
			FolderID:         req.FolderID,
			SharedByUserID:   claims.UserID,
			SharedWithUserID: sharedWithUser.ID,
			Permission:       req.Permission,
			ExpiresAt:        req.ExpiresAt,
		},
	)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "ไม่สามารถอัปเดตสิทธิ์การเข้าถึงได้")
		return
	}

	if !updated {
		response.Error(w, http.StatusNotFound, "ไม่พบผู้ใช้ที่สามารถแก้ไขสิทธิ์ได้")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "อัปเดตสิทธิ์การเข้าถึงโฟลเดอร์สำเร็จ",
		"shared_folder": map[string]interface{}{
			"folder_id":           req.FolderID,
			"shared_by_user_id":   claims.UserID,
			"shared_with_user_id": sharedWithUser.ID,
			"permission":          req.Permission,
			"expires_at":          req.ExpiresAt,
		},
	})
}

func (h *SharedFolderHandler) RemoveSharedUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		response.Error(w, http.StatusMethodNotAllowed, "ไม่อนุญาตให้ใช้ Method อื่น")
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "เข้าสู่ระบบก่อนใช้งาน")
		return
	}

	sharedFolderID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || sharedFolderID <= 0 {
		response.Error(w, http.StatusBadRequest, "รหัสแชร์โฟลเดอร์ไม่ถูกต้อง")
		return
	}

	var req models.SharedFolderRemoveUser
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "ข้อมูลที่ส่งมาไม่ถูกต้อง")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		response.Error(w, http.StatusBadRequest, "อีเมลผู้ใช้ที่ต้องการลบไม่ถูกต้อง")
		return
	}

	sharedWithUser, err := h.userRepo.FindByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "ไม่พบผู้ใช้จากอีเมลนี้")
			return
		}

		response.Error(w, http.StatusInternalServerError, "ไม่สามารถตรวจสอบผู้ใช้ได้")
		return
	}

	removed, err := h.sharedFolderRepo.RemoveSharedUser(
		r.Context(),
		sharedFolderID,
		claims.UserID,
		sharedWithUser.ID,
	)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "ไม่สามารถลบสิทธิ์ผู้ใช้นี้ได้")
		return
	}

	if !removed {
		response.Error(w, http.StatusNotFound, "ไม่พบสิทธิ์ผู้ใช้ที่ต้องการลบ")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "ลบสิทธิ์ผู้ใช้สำเร็จ",
	})
}
