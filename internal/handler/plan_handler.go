package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"cloud-storage-backend/internal/middleware"
	"cloud-storage-backend/internal/models"
	"cloud-storage-backend/internal/repository"
	"cloud-storage-backend/internal/response"
)

type PlanHandler struct {
	planRepo     *repository.PlanRepository
	userPlanRepo *repository.UserPlanRepository
}

func NewPlanHandler(
	planRepo *repository.PlanRepository,
	userPlanRepo *repository.UserPlanRepository,
) *PlanHandler {
	return &PlanHandler{
		planRepo:     planRepo,
		userPlanRepo: userPlanRepo,
	}
}

func (h *PlanHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "ไม่อนุญาตให้ใช้ Method นี้")
		return
	}

	plans, err := h.planRepo.FindAllActive(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "ไม่สามารถโหลดรายการแพ็กเกจได้")
		return
	}

	planResponses := make([]models.PlanResponse, 0, len(plans))
	for i := range plans {
		planResponses = append(planResponses, models.NewPlanResponse(&plans[i]))
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "โหลดรายการแพ็กเกจสำเร็จ",
		"plans":   planResponses,
		"total":   len(planResponses),
	})
}

func (h *PlanHandler) Current(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "ไม่อนุญาตให้ใช้ Method นี้")
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "เข้าสู่ระบบก่อนใช้งาน")
		return
	}

	userPlan, err := h.userPlanRepo.FindCurrentByUserID(r.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "ยังไม่มีแพ็กเกจปัจจุบัน")
			return
		}

		response.Error(w, http.StatusInternalServerError, "ไม่สามารถโหลดแพ็กเกจปัจจุบันได้")
		return
	}

	usedBytes, err := h.userPlanRepo.UsedStorageBytes(r.Context(), claims.UserID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "ไม่สามารถคำนวณพื้นที่ใช้งานได้")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "โหลดแพ็กเกจปัจจุบันสำเร็จ",
		"plan":    h.buildStoragePlanResponse(*userPlan, usedBytes),
	})
}

func (h *PlanHandler) Select(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "ไม่อนุญาตให้ใช้ Method นี้")
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "เข้าสู่ระบบก่อนใช้งาน")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

	var req models.SelectUserPlanRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "ข้อมูลที่ส่งมาไม่ถูกต้อง")
		return
	}

	planCode := strings.TrimSpace(strings.ToLower(req.PlanCode))
	if planCode == "" {
		response.Error(w, http.StatusBadRequest, "กรุณาระบุ plan_code")
		return
	}

	plan, err := h.planRepo.FindActiveByCode(r.Context(), planCode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusNotFound, "ไม่พบแพ็กเกจที่เลือก")
			return
		}

		response.Error(w, http.StatusInternalServerError, "ไม่สามารถโหลดข้อมูลแพ็กเกจได้")
		return
	}

	usedBytes, err := h.userPlanRepo.UsedStorageBytes(r.Context(), claims.UserID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "ไม่สามารถตรวจสอบพื้นที่ใช้งานได้")
		return
	}

	if usedBytes > plan.StorageLimitBytes {
		response.Error(w, http.StatusConflict, "พื้นที่ที่ใช้งานอยู่เกินขนาดของแพ็กเกจนี้")
		return
	}

	expiresAt := expiresAtForPlan(plan.BillingCycle)
	userPlan, err := h.userPlanRepo.AssignCurrentPlan(
		r.Context(),
		claims.UserID,
		plan.ID,
		models.UserPlanStatusActive,
		expiresAt,
		req.AutoRenew,
	)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "ไม่สามารถเลือกแพ็กเกจได้")
		return
	}

	currentPlan, err := h.userPlanRepo.FindCurrentByUserID(r.Context(), claims.UserID)
	if err != nil {
		response.JSON(w, http.StatusCreated, map[string]interface{}{
			"message":   "เลือกแพ็กเกจสำเร็จ",
			"user_plan": models.NewUserPlanResponse(userPlan),
		})
		return
	}

	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"message": "เลือกแพ็กเกจสำเร็จ",
		"plan":    h.buildStoragePlanResponse(*currentPlan, usedBytes),
	})
}

func (h *PlanHandler) buildStoragePlanResponse(
	userPlan models.CurrentUserPlan,
	usedBytes uint64,
) models.UserStoragePlanResponse {
	remainingBytes := uint64(0)
	if userPlan.StorageLimitBytes > usedBytes {
		remainingBytes = userPlan.StorageLimitBytes - usedBytes
	}

	usagePercent := 0.0
	if userPlan.StorageLimitBytes > 0 {
		usagePercent = (float64(usedBytes) / float64(userPlan.StorageLimitBytes)) * 100
	}

	return models.UserStoragePlanResponse{
		UserPlan:              userPlan,
		UsedStorageBytes:      usedBytes,
		RemainingStorageBytes: remainingBytes,
		StorageUsagePercent:   usagePercent,
	}
}

func expiresAtForPlan(billingCycle string) *time.Time {
	now := time.Now()

	switch billingCycle {
	case models.BillingCycleMonthly:
		value := now.AddDate(0, 1, 0)
		return &value
	case models.BillingCycleYearly:
		value := now.AddDate(1, 0, 0)
		return &value
	default:
		return nil
	}
}
