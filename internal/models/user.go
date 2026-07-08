package models

import "time"

// User คือ struct ที่แทนข้อมูลจากตาราง `user`

type User struct {
	ID              int        `json:"id"`
	FirstName       string     `json:"first_name"`
	LastName        string     `json:"last_name"`
	Email           string     `json:"email"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	PicturePath     *string    `json:"picture_path"`
	Phone           *string    `json:"phone"`
	PasswordHash    *string    `json:"-"`
}

type UpdateUserProfileRequest struct {
	FirstName   *string `json:"first_name"`
	LastName    *string `json:"last_name"`
	Email       *string `json:"email"`
	Phone       *string `json:"phone"`
	PicturePath *string `json:"picture_path"`
}
