package models

import "time"

type UserFolder struct {
	ID         int64      `json:"id"`
	UserID     int        `json:"user_id"`
	ParentID   *int64     `json:"parent_id"`
	FolderName string     `json:"folder_name"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

type UserFolderResponse struct {
	ID         int64      `json:"id"`
	ParentID   *int64     `json:"parent_id"`
	FolderName string     `json:"folder_name"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

// CreateFolderRequest คือข้อมูลสำหรับสร้างโฟลเดอร์
type CreateFolderRequest struct {
	FolderName string `json:"folder_name"`

	// nil หมายถึงสร้างที่ Root
	// มีค่าหมายถึงสร้างภายในโฟลเดอร์นั้น
	ParentID *int64 `json:"parent_id"`
}

type MoveFolderRequest struct {
	// nil หมายถึงย้ายกลับไปหน้า Root
	FolderID *int64 `json:"folder_id"`
}

type RenameFolderRequest struct {
	FolderName string `json:"folder_name"`
}

func NewUserFolderResponse(folder *UserFolder) UserFolderResponse {
	return UserFolderResponse{
		ID:         folder.ID,
		ParentID:   folder.ParentID,
		FolderName: folder.FolderName,
		CreatedAt:  folder.CreatedAt,
		UpdatedAt:  folder.UpdatedAt,
		DeletedAt:  folder.DeletedAt,
	}
}
