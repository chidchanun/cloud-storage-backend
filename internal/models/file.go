package models

import "time"

type UserFile struct {
	ID             int64      `json:"id"`
	UserID         int        `json:"user_id"`
	FolderID       *int64     `json:"folder_id"`
	OriginalName   string     `json:"original_name"`
	StoredName     string     `json:"stored_name"`
	StoragePath    string     `json:"-"`
	MimeType       string     `json:"mime_type"`
	SizeBytes      uint64     `json:"size_bytes"`
	ChecksumSHA256 *string    `json:"checksum_sha256,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

type RenameFileRequest struct {
	OriginalName string `json:"original_name"`
}

// MoveFileRequest คือข้อมูลสำหรับย้ายไฟล์ไปยังโฟลเดอร์อื่น
type MoveFileRequest struct {
	// nil หมายถึงย้ายกลับไปหน้า Root
	FolderID *int64 `json:"folder_id"`
}

// UserFileResponse คือข้อมูลไฟล์ที่ปลอดภัยสำหรับส่งกลับไปยัง Client
type StartChunkUploadRequest struct {
	OriginalName string `json:"original_name"`
	SizeBytes    uint64 `json:"size_bytes"`
	FolderID     *int64 `json:"folder_id"`
}

type StartChunkUploadResponse struct {
	UploadID  string `json:"upload_id"`
	ChunkSize int64  `json:"chunk_size"`
}

type UserFileResponse struct {
	ID             int64      `json:"id"`
	FolderID       *int64     `json:"folder_id"`
	OriginalName   string     `json:"original_name"`
	MimeType       string     `json:"mime_type"`
	SizeBytes      uint64     `json:"size_bytes"`
	ChecksumSHA256 *string    `json:"checksum_sha256,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

// NewUserFileResponse แปลงข้อมูล UserFile เป็นข้อมูลสำหรับส่งกลับ Client
func NewUserFileResponse(file *UserFile) UserFileResponse {
	return UserFileResponse{
		ID:             file.ID,
		FolderID:       file.FolderID,
		OriginalName:   file.OriginalName,
		MimeType:       file.MimeType,
		SizeBytes:      file.SizeBytes,
		ChecksumSHA256: file.ChecksumSHA256,
		CreatedAt:      file.CreatedAt,
		UpdatedAt:      file.UpdatedAt,
		DeletedAt:      file.DeletedAt,
	}
}
