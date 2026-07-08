package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cloud-storage-backend/internal/models"
)

// SharedFileRepository จัดการการเข้าถึงข้อมูลการแชร์ไฟล์
// ระหว่างผู้ใช้กับฐานข้อมูล
type SharedFileRepository struct {
	db *sql.DB
}

// NewSharedFileRepository สร้าง SharedFileRepository instance ใหม่
// โดยรับ database connection ที่เปิดใช้งานแล้ว
func NewSharedFileRepository(db *sql.DB) *SharedFileRepository {
	return &SharedFileRepository{
		db: db,
	}
}

// Create บันทึกข้อมูลการแชร์ไฟล์ลงในตาราง shared_file
// และนำ ID, created_at และ updated_at ที่ฐานข้อมูลสร้าง
// กลับไปกำหนดให้ SharedFile model
func (r *SharedFileRepository) Create(
	ctx context.Context,
	sharedFile *models.SharedFile,
) error {
	// ป้องกัน panic กรณีได้รับ pointer เป็น nil
	if sharedFile == nil {
		return errors.New("shared file must not be nil")
	}

	query := `
		INSERT INTO shared_file (
			file_id,
			shared_by_user_id,
			shared_with_user_id,
			permission,
			expires_at
		)
		VALUES (?, ?, ?, ?, ?)
	`

	// เพิ่มข้อมูลการแชร์ไฟล์ลงฐานข้อมูล
	result, err := r.db.ExecContext(
		ctx,
		query,
		sharedFile.FileID,
		sharedFile.SharedByUserID,
		sharedFile.SharedWithUserID,
		sharedFile.Permission,
		sharedFile.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("create shared file: %w", err)
	}

	// อ่าน ID ที่ฐานข้อมูลสร้างจาก AUTO_INCREMENT
	sharedFileID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get shared file ID: %w", err)
	}

	sharedFile.ID = sharedFileID

	queryTimestamp := `
		SELECT
			created_at,
			updated_at
		FROM shared_file
		WHERE id = ?
		LIMIT 1
	`

	// อ่าน timestamp ที่ฐานข้อมูลสร้างกลับมาใส่ใน model
	err = r.db.QueryRowContext(
		ctx,
		queryTimestamp,
		sharedFile.ID,
	).Scan(
		&sharedFile.CreatedAt,
		&sharedFile.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("get shared file timestamps: %w", err)
	}

	return nil
}

func (r *SharedFileRepository) FindAllUserPermissionInFile(
	ctx context.Context,
	fileID int64,
	ownerUserID int,
) ([]models.SharedFileUserPermission, error) {
	query := `
		SELECT
			sf.id,
			sf.file_id,
			sf.shared_with_user_id,
			u.first_name,
			u.last_name,
			u.email,
			u.picture_path,
			sf.permission,
			sf.expires_at,
			sf.created_at,
			sf.updated_at
		FROM shared_file AS sf
		INNER JOIN user_file AS uf
			ON uf.id = sf.file_id
		INNER JOIN user AS u
			ON u.id = sf.shared_with_user_id
		WHERE sf.file_id = ?
		  AND uf.user_id = ?
		  AND uf.deleted_at IS NULL
		  AND sf.deleted_at IS NULL
		  AND u.deleted_at IS NULL
		ORDER BY sf.created_at DESC
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
		fileID,
		ownerUserID,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissions := make(
		[]models.SharedFileUserPermission,
		0,
	)

	for rows.Next() {
		var item models.SharedFileUserPermission

		var picturePath sql.NullString
		var expiresAt sql.NullTime

		if err := rows.Scan(
			&item.ID,
			&item.FileID,
			&item.UserID,
			&item.FirstName,
			&item.LastName,
			&item.Email,
			&picturePath,
			&item.Permission,
			&expiresAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if picturePath.Valid {
			value := picturePath.String
			item.PicturePath = &value
		}

		if expiresAt.Valid {
			value := expiresAt.Time
			item.ExpiresAt = &value
		}

		permissions = append(
			permissions,
			item,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return permissions, nil
}

func (r *SharedFileRepository) FindPermissionByFileIDAndUserID(
	ctx context.Context,
	fileID int64,
	userID int,
) (string, error) {
	query := `
		SELECT permission
		FROM shared_file
		WHERE file_id = ?
			AND shared_with_user_id = ?
			AND deleted_at IS NULL
			AND (
				expires_at IS NULL
				OR expires_at > NOW()
			)
		LIMIT 1
	`

	var permission string

	err := r.db.QueryRowContext(
		ctx,
		query,
		fileID,
		userID,
	).Scan(&permission)

	if err != nil {
		return "", err
	}

	return permission, nil
}

func (r *SharedFileRepository) FindSharedFilesByUserID(
	ctx context.Context,
	sharedWithUserID int,
) ([]models.SharedFileListItem, error) {
	query := `
		SELECT
			sf.id AS shared_file_id,
			uf.id AS file_id,
			uf.original_name,
			sf.permission,
			sf.expires_at
		FROM shared_file AS sf
		INNER JOIN user_file AS uf
			ON uf.id = sf.file_id
		WHERE sf.shared_with_user_id = ?
			AND sf.deleted_at IS NULL
			AND uf.deleted_at IS NULL
			AND (
				sf.expires_at IS NULL
				OR sf.expires_at > NOW()
			)
		ORDER BY sf.created_at DESC
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
		sharedWithUserID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make([]models.SharedFileListItem, 0)

	for rows.Next() {
		var file models.SharedFileListItem

		err := rows.Scan(
			&file.ID,
			&file.FileID,
			&file.FileName,
			&file.Permission,
			&file.ExpiresAt,
		)
		if err != nil {
			return nil, err
		}

		files = append(files, file)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return files, nil
}

func (r *SharedFileRepository) UpdatePermissionUserFile(
	ctx context.Context,
	sharedFile *models.SharedFile,
) (bool, error) {
	query := `
		UPDATE shared_file
		SET
			permission = ?,
			expires_at = ?,
			updated_at = NOW()
		WHERE file_id = ?
			AND shared_by_user_id = ?
			AND shared_with_user_id = ?
			AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		sharedFile.Permission,
		sharedFile.ExpiresAt,
		sharedFile.FileID,
		sharedFile.SharedByUserID,
		sharedFile.SharedWithUserID,
	)
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()

	if err != nil {
		return false, err
	}

	return rowsAffected > 0, nil

}

func (r *SharedFileRepository) RemoveSharedUser(
	ctx context.Context,
	sharedFileID int64,
	sharedByUserID int,
	sharedWithUserID int,
) (bool, error) {
	query := `
		UPDATE shared_file
		SET
			updated_at = NOW(),
			deleted_at = NOW()
		WHERE id = ?
			AND shared_by_user_id = ?
			AND shared_with_user_id = ?
			AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		sharedFileID,
		sharedByUserID,
		sharedWithUserID,
	)
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rowsAffected > 0, nil
}

// RemoveAllSharedUsers ยกเลิกการแชร์ไฟล์ให้ผู้ใช้ทั้งหมด
// โดยลบแบบ Soft Delete เฉพาะรายการแชร์ที่ยังใช้งานอยู่
func (r *SharedFileRepository) RemoveAllSharedUsers(
	ctx context.Context,
	fileID int64,
	sharedByUserID int,
) (bool, error) {
	query := `
		UPDATE shared_file
		SET
			updated_at = NOW(),
			deleted_at = NOW()
		WHERE file_id = ?
			AND shared_by_user_id = ?
			AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		fileID,
		sharedByUserID,
	)
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rowsAffected > 0, nil
}
