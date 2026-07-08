package models

import "time"

type SharedFile struct {
	ID               int64      `json:"id"`
	FileID           int64      `json:"file_id"`
	SharedByUserID   int        `json:"shared_by_user_id"`
	SharedWithUserID int        `json:"shared_with_user_id"`
	Permission       string     `json:"permission"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

type SharedFileResponse struct {
	ID               int64      `json:"id"`
	FileID           int64      `json:"file_id"`
	SharedByUserID   int        `json:"shared_by_user_id"`
	SharedWithUserID int        `json:"shared_with_user_id"`
	Permission       string     `json:"permission"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type SharedFileAddUserRequest struct {
	FileID     int64      `json:"file_id"`
	Email      string     `json:"email"`
	Permission string     `json:"permission"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

type SharedFileUpdatePermissionRequest struct {
	ID         int64      `json:"id"`
	FileID     int64      `json:"file_id"`
	Email      string     `json:"email"`
	Permission string     `json:"permission"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

type SharedFileListItem struct {
	ID         int64      `json:"id"`
	FileID     int64      `json:"file_id"`
	FileName   string     `json:"file_name"`
	Permission string     `json:"permission"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

type SharedFileUserPermission struct {
	ID int64 `json:"id"`

	FileID int64 `json:"file_id"`

	UserID int `json:"user_id"`

	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`

	PicturePath *string `json:"picture_path,omitempty"`

	Permission string `json:"permission"`

	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SharedFileRemoveUser struct {
	Email string `json:"email"`
}

func NewSharedFileResponse(sharedFile *SharedFile) SharedFileResponse {
	return SharedFileResponse{
		ID:               sharedFile.ID,
		FileID:           sharedFile.FileID,
		SharedByUserID:   sharedFile.SharedByUserID,
		SharedWithUserID: sharedFile.SharedWithUserID,
		Permission:       sharedFile.Permission,
		ExpiresAt:        sharedFile.ExpiresAt,
		CreatedAt:        sharedFile.CreatedAt,
		UpdatedAt:        sharedFile.UpdatedAt,
	}
}
