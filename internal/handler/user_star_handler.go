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

type UserFileStarHandler struct {
	userFileStarRepo *repository.UserFileStarRepository
}

func NewUserFileStarHandler(
	userFileStarRepo *repository.UserFileStarRepository,
) *UserFileStarHandler {
	return &UserFileStarHandler{
		userFileStarRepo: userFileStarRepo,
	}
}

// StarFile กดไฟล์เป็นสำคัญ
// POST /api/files/{id}/star
func (h *UserFileStarHandler) StarFile(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		response.Error(
			w,
			http.StatusMethodNotAllowed,
			"ไม่อนุญาตให้ใช้ Method อื่น",
		)
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่ระบบก่อนใช้งาน",
		)

		return
	}

	fileID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

	if err != nil || fileID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"รหัสไฟล์ไม่ถูกต้อง",
		)

		return
	}

	userFileStar, err := h.userFileStarRepo.StarFile(
		r.Context(),
		claims.UserID,
		fileID,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบไฟล์หรือคุณไม่มีสิทธิ์เข้าถึงไฟล์นี้",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถกดไฟล์เป็นสำคัญได้",
		)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message":    "กดไฟล์เป็นสำคัญสำเร็จ",
			"is_starred": true,
			"star":       models.NewUserFileStar(userFileStar),
		},
	)
}

// UnstarFile ยกเลิกไฟล์สำคัญ
// DELETE /api/files/{id}/star
func (h *UserFileStarHandler) UnstarFile(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodDelete {
		response.Error(
			w,
			http.StatusMethodNotAllowed,
			"ไม่อนุญาตให้ใช้ Method อื่น",
		)
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่่ระบบก่อนใช้งาน",
		)

		return
	}

	fileID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || fileID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"รหัสไฟล์ไม่ถูกต้อง",
		)

		return
	}

	deleted, err := h.userFileStarRepo.UnStarFile(
		r.Context(),
		claims.UserID,
		fileID,
	)

	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถยกเลิกไฟล์สำคัญได้",
		)
		return
	}

	if !deleted {
		response.JSON(
			w,
			http.StatusOK,
			map[string]interface{}{
				"message":    "ไฟล์นี้ไม่ได้ถูกกดเป็นสำคัญอยู่แล้ว",
				"file_id":    fileID,
				"is_starred": false,
			},
		)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message":    "ยกเลิกไฟล์สำคัญสำเร็จ",
			"file_id":    fileID,
			"is_starred": false,
		},
	)
}

// CheckFileStar ตรวจว่าไฟล์นี้ถูกกดสำคัญหรือยัง
// GET /api/files/{id}/star
func (h *UserFileStarHandler) CheckFileStar(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims, ok := middleware.UserFromContext(r.Context())

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่่ระบบก่อนใช้งาน",
		)
		return
	}

	fileID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || fileID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"รหัสไฟล์ไม่ถูกต้อง",
		)

		return
	}

	isStarred, err := h.userFileStarRepo.IsFileStarred(
		r.Context(),
		claims.UserID,
		fileID,
	)

	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถตรวจสอบสถานะไฟล์สำคัญได้",
		)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message":    "ตรวจสอบสถานะไฟล์สำคัญสำเร็จ",
			"file_id":    fileID,
			"is_starred": isStarred,
		},
	)
}

// ListStarredFiles ดึงรายการไฟล์สำคัญทั้งหมดของผู้ใช้
// GET /api/files/starred
func (h *UserFileStarHandler) ListStarredFiles(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	items, err := h.userFileStarRepo.FindAllByUserID(
		r.Context(),
		claims.UserID,
	)

	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหลดรายการไฟล์สำคัญได้",
		)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message": "โหลดรายการไฟล์สำคัญสำเร็จ",
			"files":   items,
			"total":   len(items),
		},
	)
}
