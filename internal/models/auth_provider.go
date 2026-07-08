package models

import "time"

const (
	// AuthProviderGoogle คือชื่อ Provider ที่ใช้เก็บในฐานข้อมูล
	AuthProviderGoogle = "google"
)

// UserAuthProvider ใช้แทนข้อมูลจากตาราง user_auth_provider
type UserAuthProvider struct {
	ID              int64     `json:"id"`
	UserID          int       `json:"user_id"`
	Provider        string    `json:"provider"`
	ProviderSubject string    `json:"provider_subject"`
	ProviderEmail   *string   `json:"provider_email"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}