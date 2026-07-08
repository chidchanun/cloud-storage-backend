package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"cloud-storage-backend/internal/middleware"
	"cloud-storage-backend/internal/models"
	"cloud-storage-backend/internal/repository"
	"cloud-storage-backend/internal/response"
)

type UserHandler struct {
	userRepo      *repository.UserRepository
	maxUploadSize int64
}

func NewUserHandler(
	userRepo *repository.UserRepository,
	maxUploadSize int64,
) *UserHandler {
	if maxUploadSize <= 0 {
		maxUploadSize = maxProfilePictureSize
	}

	return &UserHandler{
		userRepo:      userRepo,
		maxUploadSize: maxUploadSize,
	}
}

func (h *UserHandler) UpdateUserProfile(
	w http.ResponseWriter,
	r *http.Request,
) {
	// PATCH เหมาะกับการอัปเดตเฉพาะบางฟิลด์
	if r.Method != http.MethodPatch {
		response.Error(
			w,
			http.StatusMethodNotAllowed,
			"ไม่อนุญาตให้ใช้ Method นี้",
		)
		return
	}

	// ดึงข้อมูลผู้ใช้จาก Access Token
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	// ตรวจสอบว่าผู้ใช้ยังมีอยู่และยังไม่ถูกลบ
	_, err := h.userRepo.FindByID(
		r.Context(),
		claims.UserID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบผู้ใช้งาน",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหลดข้อมูลผู้ใช้ได้",
		)
		return
	}

	var req models.UpdateUserProfileRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"ข้อมูลที่ส่งมาไม่ถูกต้อง",
		)
		return
	}

	// ต้องส่งมาอย่างน้อยหนึ่งฟิลด์
	if req.FirstName == nil &&
		req.LastName == nil &&
		req.Email == nil &&
		req.Phone == nil &&
		req.PicturePath == nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"กรุณาระบุข้อมูลที่ต้องการอัปเดต",
		)
		return
	}

	// ตรวจสอบและปรับ first_name
	if req.FirstName != nil {
		value := strings.TrimSpace(*req.FirstName)

		if value == "" {
			response.Error(
				w,
				http.StatusBadRequest,
				"กรุณาระบุชื่อ",
			)
			return
		}

		if len(value) > 100 {
			response.Error(
				w,
				http.StatusBadRequest,
				"ชื่อต้องมีความยาวไม่เกิน 100 ตัวอักษร",
			)
			return
		}

		req.FirstName = &value
	}

	// ตรวจสอบและปรับ last_name
	if req.LastName != nil {
		value := strings.TrimSpace(*req.LastName)

		if value == "" {
			response.Error(
				w,
				http.StatusBadRequest,
				"กรุณาระบุนามสกุล",
			)
			return
		}

		if len(value) > 100 {
			response.Error(
				w,
				http.StatusBadRequest,
				"นามสกุลต้องมีความยาวไม่เกิน 100 ตัวอักษร",
			)
			return
		}

		req.LastName = &value
	}

	// ตรวจสอบ Email
	if req.Email != nil {
		value := strings.ToLower(
			strings.TrimSpace(*req.Email),
		)

		if value == "" {
			response.Error(
				w,
				http.StatusBadRequest,
				"กรุณาระบุอีเมล",
			)
			return
		}

		address, err := mail.ParseAddress(value)
		if err != nil || address.Address != value {
			response.Error(
				w,
				http.StatusBadRequest,
				"รูปแบบอีเมลไม่ถูกต้อง",
			)
			return
		}

		req.Email = &value
	}

	// อัปเดตข้อมูล
	if req.Phone != nil {
		value := strings.TrimSpace(*req.Phone)

		if value == "" {
			response.Error(
				w,
				http.StatusBadRequest,
				"phone is required",
			)
			return
		}

		if len(value) < 9 || len(value) > 20 {
			response.Error(
				w,
				http.StatusBadRequest,
				"phone must be 9-20 characters",
			)
			return
		}

		req.Phone = &value
	}

	updated, err := h.userRepo.UpdateUserProfile(
		r.Context(),
		claims.UserID,
		&req,
	)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถอัปเดตข้อมูลผู้ใช้ได้",
		)
		return
	}

	if !updated {
		response.Error(
			w,
			http.StatusNotFound,
			"ไม่พบผู้ใช้งานหรือไม่มีข้อมูลถูกเปลี่ยนแปลง",
		)
		return
	}

	// โหลดข้อมูลล่าสุดหลังอัปเดต
	updatedUser, err := h.userRepo.FindByID(
		r.Context(),
		claims.UserID,
	)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"อัปเดตสำเร็จ แต่ไม่สามารถโหลดข้อมูลล่าสุดได้",
		)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]any{
			"message": "อัปเดตข้อมูลผู้ใช้สำเร็จ",
			"user":    models.NewUserResponse(updatedUser),
		},
	)
}
