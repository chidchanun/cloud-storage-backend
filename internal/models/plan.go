package models

import "time"

const (
	BillingCycleFree    = "free"
	BillingCycleMonthly = "monthly"
	BillingCycleYearly  = "yearly"
)

// Plan ใช้แทนข้อมูลจากตาราง plan
type Plan struct {
	ID          int     `json:"id"`
	PlanName    string  `json:"plan_name"`
	PlanCode    string  `json:"plan_code"`
	Description *string `json:"description,omitempty"`

	// หน่วยเป็น byte
	StorageLimitBytes uint64 `json:"storage_limit_bytes"`
	MaxFileSizeBytes  uint64 `json:"max_file_size_bytes"`

	// nil หมายถึงไม่จำกัด
	MaxFiles *uint32 `json:"max_files,omitempty"`

	// nil หมายถึงไม่จำกัด
	MaxShareUsersPerFile *uint32 `json:"max_share_users_per_file,omitempty"`

	// ใช้ string เพื่อรักษาความแม่นยำของ DECIMAL(10,2)
	Price string `json:"price"`

	BillingCycle string `json:"billing_cycle"`
	IsActive     bool   `json:"is_active"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// PlanResponse คือข้อมูลแพ็กเกจสำหรับส่งกลับ Client
type PlanResponse struct {
	ID          int     `json:"id"`
	PlanName    string  `json:"plan_name"`
	PlanCode    string  `json:"plan_code"`
	Description *string `json:"description,omitempty"`

	StorageLimitBytes uint64 `json:"storage_limit_bytes"`
	MaxFileSizeBytes  uint64 `json:"max_file_size_bytes"`

	MaxFiles             *uint32 `json:"max_files,omitempty"`
	MaxShareUsersPerFile *uint32 `json:"max_share_users_per_file,omitempty"`

	Price        string `json:"price"`
	BillingCycle string `json:"billing_cycle"`
	IsActive     bool   `json:"is_active"`
}

func NewPlanResponse(plan *Plan) PlanResponse {
	return PlanResponse{
		ID:                   plan.ID,
		PlanName:             plan.PlanName,
		PlanCode:             plan.PlanCode,
		Description:          plan.Description,
		StorageLimitBytes:    plan.StorageLimitBytes,
		MaxFileSizeBytes:     plan.MaxFileSizeBytes,
		MaxFiles:             plan.MaxFiles,
		MaxShareUsersPerFile: plan.MaxShareUsersPerFile,
		Price:                plan.Price,
		BillingCycle:         plan.BillingCycle,
		IsActive:             plan.IsActive,
	}
}