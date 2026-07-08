package models

import "time"

const (
	UserPlanStatusPending   = "pending"
	UserPlanStatusTrial     = "trial"
	UserPlanStatusActive    = "active"
	UserPlanStatusPastDue   = "past_due"
	UserPlanStatusCancelled = "cancelled"
	UserPlanStatusExpired   = "expired"
)

// UserPlan ใช้แทนข้อมูลจากตาราง user_plan
type UserPlan struct {
	ID     int64 `json:"id"`
	UserID int   `json:"user_id"`
	PlanID int   `json:"plan_id"`

	Status string `json:"status"`

	StartedAt   time.Time  `json:"started_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`

	AutoRenew bool `json:"auto_renew"`

	PaymentProvider        *string `json:"payment_provider,omitempty"`
	ProviderSubscriptionID *string `json:"provider_subscription_id,omitempty"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`

	// Generated Column ของ MySQL
	// ใช้สำหรับ Unique Index ภายในฐานข้อมูล
	CurrentPlanKey *int8 `json:"-"`
}

// UserPlanResponse คือข้อมูลแพ็กเกจของผู้ใช้สำหรับส่งกลับ Client
type UserPlanResponse struct {
	ID     int64 `json:"id"`
	UserID int   `json:"user_id"`
	PlanID int   `json:"plan_id"`

	Status string `json:"status"`

	StartedAt   time.Time  `json:"started_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`

	AutoRenew bool `json:"auto_renew"`

	PaymentProvider        *string `json:"payment_provider,omitempty"`
	ProviderSubscriptionID *string `json:"provider_subscription_id,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CurrentUserPlan ใช้รับผลลัพธ์จากการ JOIN user_plan กับ plan
type CurrentUserPlan struct {
	UserPlanID int64 `json:"user_plan_id"`
	UserID     int   `json:"user_id"`
	PlanID     int   `json:"plan_id"`

	Status string `json:"status"`

	StartedAt   time.Time  `json:"started_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`

	AutoRenew bool `json:"auto_renew"`

	PlanName    string  `json:"plan_name"`
	PlanCode    string  `json:"plan_code"`
	Description *string `json:"description,omitempty"`

	StorageLimitBytes uint64 `json:"storage_limit_bytes"`
	MaxFileSizeBytes  uint64 `json:"max_file_size_bytes"`

	MaxFiles             *uint32 `json:"max_files,omitempty"`
	MaxShareUsersPerFile *uint32 `json:"max_share_users_per_file,omitempty"`

	Price        string `json:"price"`
	BillingCycle string `json:"billing_cycle"`
}

type UserStoragePlanResponse struct {
	UserPlan CurrentUserPlan `json:"user_plan"`

	UsedStorageBytes      uint64  `json:"used_storage_bytes"`
	RemainingStorageBytes uint64  `json:"remaining_storage_bytes"`
	StorageUsagePercent   float64 `json:"storage_usage_percent"`
}

type SelectUserPlanRequest struct {
	PlanCode  string `json:"plan_code"`
	AutoRenew bool   `json:"auto_renew"`
}

func NewUserPlanResponse(userPlan *UserPlan) UserPlanResponse {
	return UserPlanResponse{
		ID:                     userPlan.ID,
		UserID:                 userPlan.UserID,
		PlanID:                 userPlan.PlanID,
		Status:                 userPlan.Status,
		StartedAt:              userPlan.StartedAt,
		ExpiresAt:              userPlan.ExpiresAt,
		CancelledAt:            userPlan.CancelledAt,
		AutoRenew:              userPlan.AutoRenew,
		PaymentProvider:        userPlan.PaymentProvider,
		ProviderSubscriptionID: userPlan.ProviderSubscriptionID,
		CreatedAt:              userPlan.CreatedAt,
		UpdatedAt:              userPlan.UpdatedAt,
	}
}
