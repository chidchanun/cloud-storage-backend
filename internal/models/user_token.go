package models

import "time"

type UserToken struct {
	ID        int
	UserID    int
	TokenHash string
	ExpiredAt time.Time
	CreatedAt time.Time
	RevokedAt *time.Time
}