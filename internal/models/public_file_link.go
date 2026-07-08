package models

import "time"

type PublicFileLink struct {
	ID         int64      `json:"id"`
	FileID     int64      `json:"file_id"`
	UserID     int        `json:"user_id"`
	TokenHash  string     `json:"-"`
	Permission string     `json:"permission"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

type CreatePublicFileLinkRequest struct {
	FileID    int64      `json:"file_id"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type PublicFileLinkResponse struct {
	ID         int64      `json:"id"`
	FileID     int64      `json:"file_id"`
	Permission string     `json:"permission"`
	URL        string     `json:"url"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func NewPublicFileLinkResponse(
	link *PublicFileLink,
	url string,
) PublicFileLinkResponse {
	return PublicFileLinkResponse{
		ID:         link.ID,
		FileID:     link.FileID,
		Permission: link.Permission,
		URL:        url,
		ExpiresAt:  link.ExpiresAt,
		CreatedAt:  link.CreatedAt,
		UpdatedAt:  link.UpdatedAt,
	}
}
