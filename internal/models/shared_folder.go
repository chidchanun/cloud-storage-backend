package models

import (
	"errors"
	"strings"
	"time"
)

const (
	SharedPermissionViewer = "viewer"
	SharedPermissionEditor = "editor"
)

type SharedFolder struct {
	ID               int64      `json:"id"`
	FolderID         int64      `json:"folder_id"`
	SharedByUserID   int        `json:"shared_by_user_id"`
	SharedWithUserID int        `json:"shared_with_user_id"`
	Permission       string     `json:"permission"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

type SharedFolderResponse struct {
	ID               int64      `json:"id"`
	FolderID         int64      `json:"folder_id"`
	SharedByUserID   int        `json:"shared_by_user_id"`
	SharedWithUserID int        `json:"shared_with_user_id"`
	Permission       string     `json:"permission"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type SharedFolderUpdatePermissionRequest struct {
	ID         int64      `json:"id"`
	FolderID   int64      `json:"folder_id"`
	Email      string     `json:"email"`
	Permission string     `json:"permission"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

type SharedFolderAddUserRequest struct {
	FolderID   int64      `json:"folder_id"`
	Email      string     `json:"email"`
	Permission string     `json:"permission"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

type SharedFolderListItem struct {
	ID         int64      `json:"id"`
	FolderID   int64      `json:"folder_id"`
	FolderName string     `json:"folder_name"`
	Permission string     `json:"permission"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

type SharedFolderUserPermission struct {
	ID          int64   `json:"id"`
	FolderID    int64   `json:"folder_id"`
	UserID      int     `json:"user_id"`
	FirstName   string  `json:"first_name"`
	LastName    string  `json:"last_name"`
	Email       string  `json:"email"`
	PicturePath *string `json:"picture_path,omitempty"`
	Permission  string  `json:"permission"`

	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type SharedFolderRemoveUser struct {
	Email string `json:"email"`
}

func (r *SharedFolderUpdatePermissionRequest) Validate() error {
	r.Permission = strings.ToLower(strings.TrimSpace(r.Permission))

	switch r.Permission {
	case SharedPermissionViewer, SharedPermissionEditor:
		return nil
	default:
		return errors.New("permission ต้องเป็น viewer หรือ editor")
	}
}

func NewSharedFolderResponse(
	sharedFolder *SharedFolder,
) SharedFolderResponse {
	if sharedFolder == nil {
		return SharedFolderResponse{}
	}

	return SharedFolderResponse{
		ID:               sharedFolder.ID,
		FolderID:         sharedFolder.FolderID,
		SharedByUserID:   sharedFolder.SharedByUserID,
		SharedWithUserID: sharedFolder.SharedWithUserID,
		Permission:       sharedFolder.Permission,
		ExpiresAt:        sharedFolder.ExpiresAt,
		CreatedAt:        sharedFolder.CreatedAt,
		UpdatedAt:        sharedFolder.UpdatedAt,
	}
}
