package repository

import (
	"context"
	"database/sql"
	"strings"

	"cloud-storage-backend/internal/models"
)

// FileRepository ใช้จัดการข้อมูลในตาราง user_file
type FileRepository struct {
	db *sql.DB
}

// NewFileRepository สร้าง FileRepository
func NewFileRepository(db *sql.DB) *FileRepository {
	return &FileRepository{
		db: db,
	}
}

// Create บันทึก metadata ของไฟล์ลงฐานข้อมูล
func (r *FileRepository) Create(
	ctx context.Context,
	file *models.UserFile,
) error {
	query := `
		INSERT INTO user_file (
			user_id,
			folder_id,
			original_name,
			stored_name,
			storage_path,
			mime_type,
			size_bytes,
			checksum_sha256
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		file.UserID,
		file.FolderID,
		file.OriginalName,
		file.StoredName,
		file.StoragePath,
		file.MimeType,
		file.SizeBytes,
		file.ChecksumSHA256,
	)
	if err != nil {
		return err
	}

	fileID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	file.ID = fileID

	queryTimestamp := `
		SELECT created_at, updated_at
		FROM user_file
		WHERE id = ?
		  AND user_id = ?
		LIMIT 1
	`

	return r.db.QueryRowContext(
		ctx,
		queryTimestamp,
		file.ID,
		file.UserID,
	).Scan(
		&file.CreatedAt,
		&file.UpdatedAt,
	)
}

// FindAllByUserID ดึงรายการไฟล์ทั้งหมดของ user
func (r *FileRepository) FindAllByFolderID(
	ctx context.Context,
	userID int,
	folderID *int64,
) ([]models.UserFile, error) {
	query := `
		SELECT
			id,
			user_id,
			folder_id,
			original_name,
			stored_name,
			storage_path,
			mime_type,
			size_bytes,
			checksum_sha256,
			created_at,
			updated_at,
			deleted_at
		FROM user_file
		WHERE user_id = ?
		  AND folder_id <=> ?
		  AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
		userID,
		folderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make([]models.UserFile, 0)

	for rows.Next() {
		var file models.UserFile
		var folderIDValue sql.NullInt64
		var checksum sql.NullString
		var deletedAt sql.NullTime

		if err := rows.Scan(
			&file.ID,
			&file.UserID,
			&folderIDValue,
			&file.OriginalName,
			&file.StoredName,
			&file.StoragePath,
			&file.MimeType,
			&file.SizeBytes,
			&checksum,
			&file.CreatedAt,
			&file.UpdatedAt,
			&deletedAt,
		); err != nil {
			return nil, err
		}

		if folderIDValue.Valid {
			value := folderIDValue.Int64
			file.FolderID = &value
		}

		if checksum.Valid {
			value := checksum.String
			file.ChecksumSHA256 = &value
		}

		if deletedAt.Valid {
			value := deletedAt.Time
			file.DeletedAt = &value
		}

		files = append(files, file)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return files, nil
}

// FindByIDAndUserID ดึงไฟล์จาก ID และตรวจว่าเป็นไฟล์ของ user คนนี้
func (r *FileRepository) FindByIDAndUserID(
	ctx context.Context,
	fileID int64,
	userID int,
) (*models.UserFile, error) {
	query := `
		SELECT
			id,
			user_id,
			folder_id,
			original_name,
			stored_name,
			storage_path,
			mime_type,
			size_bytes,
			checksum_sha256,
			created_at,
			updated_at,
			deleted_at
		FROM user_file
		WHERE id = ?
		  AND user_id = ?
		  AND deleted_at IS NULL
		LIMIT 1
	`

	var file models.UserFile

	var checksum sql.NullString
	var deletedAt sql.NullTime
	var folderID sql.NullInt64

	err := r.db.QueryRowContext(
		ctx,
		query,
		fileID,
		userID,
	).Scan(
		&file.ID,
		&file.UserID,
		&folderID,
		&file.OriginalName,
		&file.StoredName,
		&file.StoragePath,
		&file.MimeType,
		&file.SizeBytes,
		&checksum,
		&file.CreatedAt,
		&file.UpdatedAt,
		&deletedAt,
	)

	if err != nil {
		return nil, err
	}

	if folderID.Valid {
		value := folderID.Int64
		file.FolderID = &value
	}

	if checksum.Valid {
		checksumValue := checksum.String
		file.ChecksumSHA256 = &checksumValue
	}

	if deletedAt.Valid {
		deletedAtValue := deletedAt.Time
		file.DeletedAt = &deletedAtValue
	}

	return &file, nil
}

// SearchByFileName ค้นหาไฟล์จากชื่อเดิมของไฟล์
// ค้นหาเฉพาะไฟล์ของผู้ใช้ และไฟล์ที่ยังไม่ถูกลบ
func (r *FileRepository) SearchByFileName(
	ctx context.Context,
	userID int,
	keyword string,
) ([]models.UserFile, error) {
	keyword = strings.TrimSpace(keyword)

	// ป้องกัน keyword ว่างแล้วดึงไฟล์ทั้งหมดโดยไม่ตั้งใจ
	if keyword == "" {
		return []models.UserFile{}, nil
	}

	query := `
		SELECT
			id,
			user_id,
			folder_id,
			original_name,
			stored_name,
			storage_path,
			mime_type,
			size_bytes,
			checksum_sha256,
			created_at,
			updated_at,
			deleted_at
		FROM user_file
		WHERE user_id = ?
		  AND deleted_at IS NULL
		  AND LOCATE(
				LOWER(?),
				LOWER(original_name)
		  ) > 0
		ORDER BY
			LOCATE(
				LOWER(?),
				LOWER(original_name)
			) ASC,
			updated_at DESC
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
		userID,
		keyword,
		keyword,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make([]models.UserFile, 0)

	for rows.Next() {
		var file models.UserFile

		var folderID sql.NullInt64
		var checksumSHA256 sql.NullString
		var deletedAt sql.NullTime

		if err := rows.Scan(
			&file.ID,
			&file.UserID,
			&folderID,
			&file.OriginalName,
			&file.StoredName,
			&file.StoragePath,
			&file.MimeType,
			&file.SizeBytes,
			&checksumSHA256,
			&file.CreatedAt,
			&file.UpdatedAt,
			&deletedAt,
		); err != nil {
			return nil, err
		}

		if folderID.Valid {
			value := folderID.Int64
			file.FolderID = &value
		}

		if checksumSHA256.Valid {
			value := checksumSHA256.String
			file.ChecksumSHA256 = &value
		}

		if deletedAt.Valid {
			value := deletedAt.Time
			file.DeletedAt = &value
		}

		files = append(files, file)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return files, nil
}

func (r *FileRepository) SearchByFileNameInFolder(
	ctx context.Context,
	userID int,
	folderID *int64,
	keyword string,
) ([]models.UserFile, error) {
	keyword = strings.TrimSpace(keyword)

	if keyword == "" {
		return []models.UserFile{}, nil
	}

	query := `
		SELECT
			id,
			user_id,
			folder_id,
			original_name,
			stored_name,
			storage_path,
			mime_type,
			size_bytes,
			checksum_sha256,
			created_at,
			updated_at,
			deleted_at
		FROM user_file
		WHERE user_id = ?
		  AND folder_id <=> ?
		  AND deleted_at IS NULL
		  AND LOCATE(
				LOWER(?),
				LOWER(original_name)
		  ) > 0
		ORDER BY
			LOCATE(
				LOWER(?),
				LOWER(original_name)
			) ASC,
			updated_at DESC
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
		userID,
		folderID,
		keyword,
		keyword,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make([]models.UserFile, 0)

	for rows.Next() {
		var file models.UserFile

		var folderIDValue sql.NullInt64
		var checksumSHA256 sql.NullString
		var deletedAt sql.NullTime

		if err := rows.Scan(
			&file.ID,
			&file.UserID,
			&folderIDValue,
			&file.OriginalName,
			&file.StoredName,
			&file.StoragePath,
			&file.MimeType,
			&file.SizeBytes,
			&checksumSHA256,
			&file.CreatedAt,
			&file.UpdatedAt,
			&deletedAt,
		); err != nil {
			return nil, err
		}

		if folderIDValue.Valid {
			value := folderIDValue.Int64
			file.FolderID = &value
		}

		if checksumSHA256.Valid {
			value := checksumSHA256.String
			file.ChecksumSHA256 = &value
		}

		if deletedAt.Valid {
			value := deletedAt.Time
			file.DeletedAt = &value
		}

		files = append(files, file)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return files, nil
}

func (r *FileRepository) FindByID(
	ctx context.Context,
	fileID int64,
) (*models.UserFile, error) {
	query := `
		SELECT
			id,
			user_id,
			folder_id,
			original_name,
			stored_name,
			storage_path,
			mime_type,
			size_bytes,
			checksum_sha256,
			created_at,
			updated_at,
			deleted_at
		FROM user_file
		WHERE id = ?
		  AND deleted_at IS NULL
		LIMIT 1
	`

	var file models.UserFile
	var checksum sql.NullString
	var deletedAt sql.NullTime
	var folderID sql.NullInt64

	err := r.db.QueryRowContext(
		ctx,
		query,
		fileID,
	).Scan(
		&file.ID,
		&file.UserID,
		&folderID,
		&file.OriginalName,
		&file.StoredName,
		&file.StoragePath,
		&file.MimeType,
		&file.SizeBytes,
		&checksum,
		&file.CreatedAt,
		&file.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return nil, err
	}

	if folderID.Valid {
		value := folderID.Int64
		file.FolderID = &value
	}

	if checksum.Valid {
		value := checksum.String
		file.ChecksumSHA256 = &value
	}

	if deletedAt.Valid {
		value := deletedAt.Time
		file.DeletedAt = &value
	}

	return &file, nil
}

func (r *FileRepository) SoftDelete(
	ctx context.Context,
	fileID int64,
	userID int,
) (bool, error) {
	query := `
		UPDATE user_file
		SET
			deleted_at = NOW(),
			updated_at = NOW()
		WHERE id = ?
			AND user_id = ?
			AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		fileID,
		userID,
	)

	if err != nil {
		return false, err
	}

	// ตรวจว่ามี record ถูกแก้ไขจริงหรือไม่
	rowAffected, err := result.RowsAffected()

	if err != nil {
		return false, err
	}

	return rowAffected > 0, nil
}

// Restore ยกเลิกการ Soft Delete โดยตั้ง deleted_at กลับเป็น NULL
func (r *FileRepository) Restore(
	ctx context.Context,
	fileID int64,
	userID int,
) (bool, error) {
	query := `
		UPDATE user_file
		SET
			deleted_at = NULL,
			updated_at = NOW()
		WHERE id = ?
		  AND user_id = ?
		  AND deleted_at IS NOT NULL
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		fileID,
		userID,
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

// TrashList รายการไฟล์ขยะ ที่ยังไม่ถูกลบ
func (r *FileRepository) FindAllTrashByUser(
	ctx context.Context,
	userID int,
) ([]models.UserFile, error) {
	query := `
		SELECT
			id,
			user_id,
			original_name,
			stored_name,
			storage_path,
			mime_type,
			size_bytes,
			checksum_sha256,
			created_at,
			updated_at,
			deleted_at
		FROM user_file
		WHERE user_id = ?
		  AND deleted_at IS NOT NULL
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make([]models.UserFile, 0)

	for rows.Next() {
		var file models.UserFile

		// ใช้ NullString และ NullTime เพราะ field ใน database สามารถเป็น NULL
		var checksum sql.NullString
		var deletedAt sql.NullTime

		if err := rows.Scan(
			&file.ID,
			&file.UserID,
			&file.OriginalName,
			&file.StoredName,
			&file.StoragePath,
			&file.MimeType,
			&file.SizeBytes,
			&checksum,
			&file.CreatedAt,
			&file.UpdatedAt,
			&deletedAt,
		); err != nil {
			return nil, err
		}

		if checksum.Valid {
			checksumValue := checksum.String
			file.ChecksumSHA256 = &checksumValue
		}

		if deletedAt.Valid {
			deletedAtValue := deletedAt.Time
			file.DeletedAt = &deletedAtValue
		}
		files = append(files, file)
	}

	// ตรวจ error ที่อาจเกิดขึ้นระหว่างวนอ่านข้อมูล
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return files, nil

}

// FindTrashByIDAndUserID ดึงไฟล์ในถังขยะตาม ID
// พร้อมตรวจสอบว่าเป็นไฟล์ของผู้ใช้คนนั้น
func (r *FileRepository) FindTrashByIDAndUserID(
	ctx context.Context,
	fileID int64,
	userID int,
) (*models.UserFile, error) {
	query := `
		SELECT
			id,
			user_id,
			original_name,
			stored_name,
			storage_path,
			mime_type,
			size_bytes,
			checksum_sha256,
			created_at,
			updated_at,
			deleted_at
		FROM user_file
		WHERE id = ?
		  AND user_id = ?
		  AND deleted_at IS NOT NULL
		LIMIT 1
	`

	var file models.UserFile
	var checksum sql.NullString
	var deletedAt sql.NullTime

	err := r.db.QueryRowContext(
		ctx,
		query,
		fileID,
		userID,
	).Scan(
		&file.ID,
		&file.UserID,
		&file.OriginalName,
		&file.StoredName,
		&file.StoragePath,
		&file.MimeType,
		&file.SizeBytes,
		&checksum,
		&file.CreatedAt,
		&file.UpdatedAt,
		&deletedAt,
	)

	if err != nil {
		return nil, err
	}

	if checksum.Valid {
		checksumValue := checksum.String
		file.ChecksumSHA256 = &checksumValue
	}

	if deletedAt.Valid {
		deletedAtValue := deletedAt.Time
		file.DeletedAt = &deletedAtValue
	}

	return &file, nil
}

func (r *FileRepository) RenameFile(
	ctx context.Context,
	fileID int64,
	userID int,
	fileName string,
) (bool, error) {
	query := `
		UPDATE user_file
		SET 
			updated_at = NOW(),
			original_name = ?
		WHERE id = ?
			AND user_id = ?
			AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		fileName,
		fileID,
		userID,
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

// MoveFile เปลี่ยนตำแหน่ง Virtual Folder ของไฟล์
func (r *FileRepository) MoveFile(
	ctx context.Context,
	fileID int64,
	userID int,
	folderID *int64,
) error {
	query := `
		UPDATE user_file
		SET
			folder_id = ?,
			updated_at = NOW()
		WHERE id = ?
		  AND user_id = ?
		  AND deleted_at IS NULL
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		folderID,
		fileID,
		userID,
	)

	return err
}

func (r *FileRepository) Delete(
	ctx context.Context,
	fileID int64,
	userID int,
) (bool, error) {
	query := `
		DELETE FROM user_file
		WHERE id = ?
			AND user_id = ?
			AND deleted_at IS NOT NULL
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		fileID,
		userID,
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
