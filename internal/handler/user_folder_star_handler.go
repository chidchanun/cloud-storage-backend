package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"cloud-storage-backend/internal/middleware"
	"cloud-storage-backend/internal/models"
	"cloud-storage-backend/internal/repository"
	"cloud-storage-backend/internal/response"
)

type UserFolderStarHandler struct {
	userFolderStarRepo *repository.UserFolderStarRepository
}

func NewUserFolderStarHandler(
	userFolderStarRepo *repository.UserFolderStarRepository,
) *UserFolderStarHandler {
	return &UserFolderStarHandler{
		userFolderStarRepo: userFolderStarRepo,
	}
}

func (h *UserFolderStarHandler) StarFolder(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "ไม่อนุญาตให้ใช้ Method นี้")
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "เข้าสู่ระบบก่อนใช้งาน")
		return
	}

	folderID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || folderID <= 0 {
		response.Error(w, http.StatusBadRequest, "รหัสโฟลเดอร์ไม่ถูกต้อง")
		return
	}

	userFolderStar, err := h.userFolderStarRepo.StarFolder(
		r.Context(),
		claims.UserID,
		folderID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "ไม่พบโฟลเดอร์หรือคุณไม่มีสิทธิ์เข้าถึง")
			return
		}

		response.Error(w, http.StatusInternalServerError, "ไม่สามารถติดดาวโฟลเดอร์ได้")
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message":    "ติดดาวโฟลเดอร์สำเร็จ",
			"is_starred": true,
			"star":       models.NewUserFolderStar(userFolderStar),
		},
	)
}

func (h *UserFolderStarHandler) UnstarFolder(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodDelete {
		response.Error(w, http.StatusMethodNotAllowed, "ไม่อนุญาตให้ใช้ Method นี้")
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "เข้าสู่ระบบก่อนใช้งาน")
		return
	}

	folderID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || folderID <= 0 {
		response.Error(w, http.StatusBadRequest, "รหัสโฟลเดอร์ไม่ถูกต้อง")
		return
	}

	_, err = h.userFolderStarRepo.UnstarFolder(
		r.Context(),
		claims.UserID,
		folderID,
	)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "ไม่สามารถยกเลิกดาวโฟลเดอร์ได้")
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message":    "ยกเลิกดาวโฟลเดอร์สำเร็จ",
			"folder_id":  folderID,
			"is_starred": false,
		},
	)
}

func (h *UserFolderStarHandler) CheckFolderStar(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "เข้าสู่ระบบก่อนใช้งาน")
		return
	}

	folderID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || folderID <= 0 {
		response.Error(w, http.StatusBadRequest, "รหัสโฟลเดอร์ไม่ถูกต้อง")
		return
	}

	isStarred, err := h.userFolderStarRepo.IsFolderStarred(
		r.Context(),
		claims.UserID,
		folderID,
	)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "ไม่สามารถตรวจสอบสถานะโฟลเดอร์ได้")
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message":    "ตรวจสอบสถานะโฟลเดอร์สำเร็จ",
			"folder_id":  folderID,
			"is_starred": isStarred,
		},
	)
}

func (h *UserFolderStarHandler) ListStarredFolders(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "เข้าสู่ระบบก่อนใช้งาน")
		return
	}

	items, err := h.userFolderStarRepo.FindAllByUserID(
		r.Context(),
		claims.UserID,
	)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "ไม่สามารถโหลดรายการโฟลเดอร์โปรดได้")
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message": "โหลดรายการโฟลเดอร์โปรดสำเร็จ",
			"folders": items,
			"total":   len(items),
		},
	)
}
