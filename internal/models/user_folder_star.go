package models

import "time"

type UserFolderStar struct {
	ID        int64     `json:"id"`
	UserID    int       `json:"user_id"`
	FolderID  int64     `json:"folder_id"`
	CreatedAt time.Time `json:"created_at"`
}

type UserFolderStarResponse struct {
	ID        int64     `json:"id"`
	UserID    int       `json:"user_id"`
	FolderID  int64     `json:"folder_id"`
	CreatedAt time.Time `json:"created_at"`
}

type StarredFolderListItem struct {
	StarID int64 `json:"star_id"`

	FolderID   int64  `json:"folder_id"`
	ParentID   *int64 `json:"parent_id"`
	FolderName string `json:"folder_name"`

	FolderCreatedAt time.Time `json:"folder_created_at"`
	FolderUpdatedAt time.Time `json:"folder_updated_at"`
	StarredAt       time.Time `json:"starred_at"`
}

func NewUserFolderStar(
	userFolderStar *UserFolderStar,
) UserFolderStarResponse {
	return UserFolderStarResponse{
		ID:        userFolderStar.ID,
		UserID:    userFolderStar.UserID,
		FolderID:  userFolderStar.FolderID,
		CreatedAt: userFolderStar.CreatedAt,
	}
}
