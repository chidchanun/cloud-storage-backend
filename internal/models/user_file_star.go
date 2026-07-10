package models

import "time"

type UserFileStar struct {
	ID        int64     `json:"id"`
	UserID    int       `json:"user_id"`
	FileID    int64     `json:"file_id"`
	CreatedAt time.Time `json:"created_at"`
}

type UserFileStarResponse struct {
	ID        int64     `json:"id"`
	UserID    int       `json:"user_id"`
	FileID    int64     `json:"file_id"`
	CreatedAt time.Time `json:"created_at"`
}

type StarredFileListItem struct {
	StarID int64 `json:"star_id"`
	FileID int64 `json:"file_id"`

	FolderID     *int64 `json:"folder_id"`
	OriginalName string `json:"original_name"`
	MimeType     string `json:"mime_type"`
	SizeBytes    uint64 `json:"size_bytes"`

	FileCreatedAt time.Time `json:"file_created_at"`
	FileUpdatedAt time.Time `json:"file_updated_at"`
	StarredAt     time.Time `json:"starred_at"`
}

func NewUserFileStar(
	userFileStar *UserFileStar,
) UserFileStarResponse {
	return UserFileStarResponse{
		ID:        userFileStar.ID,
		UserID:    userFileStar.UserID,
		FileID:    userFileStar.FileID,
		CreatedAt: userFileStar.CreatedAt,
	}
}
