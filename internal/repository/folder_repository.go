package repository

import (
	"context"
	"database/sql"

	"cloud-storage-backend/internal/models"
)

// FolderRepository ใช้จัดการข้อมูลในตาราง user_folder
type FolderRepository struct {
	db *sql.DB
}

// NewFolderRepository สร้าง FolderRepository
func NewFolderRepository(db *sql.DB) *FolderRepository {
	return &FolderRepository{
		db: db,
	}
}

// Create สร้างโฟลเดอร์ใหม่
//
// parentID:
//   - nil หมายถึงสร้างโฟลเดอร์ที่หน้า Root
//   - มีค่า หมายถึงสร้างเป็นโฟลเดอร์ลูก
func (r *FolderRepository) Create(
	ctx context.Context,
	folder *models.UserFolder,
) error {
	query := `
		INSERT INTO user_folder (
			user_id,
			parent_id,
			folder_name
		)
		VALUES (?, ?, ?)
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		folder.UserID,
		folder.ParentID,
		folder.FolderName,
	)

	if err != nil {
		return err
	}

	// รับ ID ที่ MySQL สร้างจาก AUTO_INCREMENT
	folderID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	folder.ID = folderID

	// ดึงเวลาที่ MySQL สร้างกลับมา
	timestampQuery := `
		SELECT
			created_at,
			updated_at
		FROM user_folder
		WHERE id = ?
		  AND user_id = ?
		LIMIT 1
	`
	err = r.db.QueryRowContext(
		ctx,
		timestampQuery,
		folder.ID,
		folder.UserID,
	).Scan(
		&folder.CreatedAt,
		&folder.UpdatedAt,
	)

	return err
}

// FindAllByParentID ดึงรายการโฟลเดอร์ภายใต้ parent ที่กำหนด
//
// parentID:
//   - nil หมายถึงดึงโฟลเดอร์ที่หน้า Root
//   - มีค่า หมายถึงดึงโฟลเดอร์ลูกของ parent ID นั้น
func (r *FolderRepository) FindAllByParentID(
	ctx context.Context,
	userID int,
	parentID *int64,
) ([]models.UserFolder, error) {
	query := `
		SELECT
			id,
			user_id,
			parent_id,
			folder_name,
			created_at,
			updated_at,
			deleted_at
		FROM user_folder
		WHERE user_id = ?
		  AND parent_id <=> ?
		  AND deleted_at IS NULL
		ORDER BY folder_name ASC
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
		userID,
		parentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	folders := make([]models.UserFolder, 0)

	for rows.Next() {
		var folder models.UserFolder

		// parent_id และ deleted_at สามารถเป็น NULL ได้
		var parentIDValue sql.NullInt64
		var deletedAt sql.NullTime

		if err := rows.Scan(
			&folder.ID,
			&folder.UserID,
			&parentIDValue,
			&folder.FolderName,
			&folder.CreatedAt,
			&folder.UpdatedAt,
			&deletedAt,
		); err != nil {
			return nil, err
		}

		if parentIDValue.Valid {
			value := parentIDValue.Int64
			folder.ParentID = &value
		}

		if deletedAt.Valid {
			value := deletedAt.Time
			folder.DeletedAt = &value
		}

		folders = append(folders, folder)
	}

	// ตรวจ error ที่อาจเกิดระหว่างอ่าน rows
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return folders, nil
}

// FindByIDAndUserID ค้นหาโฟลเดอร์ตาม ID
// พร้อมตรวจว่าโฟลเดอร์เป็นของผู้ใช้ที่กำลัง Login
func (r *FolderRepository) FindByIDAndUserID(
	ctx context.Context,
	folderID int64,
	userID int,
) (*models.UserFolder, error) {
	query := `
		SELECT
			id,
			user_id,
			parent_id,
			folder_name,
			created_at,
			updated_at,
			deleted_at
		FROM user_folder
		WHERE id = ?
		  AND user_id = ?
		  AND deleted_at IS NULL
		LIMIT 1
	`

	var folder models.UserFolder
	var parentID sql.NullInt64
	var deletedAt sql.NullTime

	err := r.db.QueryRowContext(
		ctx,
		query,
		folderID,
		userID,
	).Scan(
		&folder.ID,
		&folder.UserID,
		&parentID,
		&folder.FolderName,
		&folder.CreatedAt,
		&folder.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return nil, err
	}

	if parentID.Valid {
		value := parentID.Int64
		folder.ParentID = &value
	}

	if deletedAt.Valid {
		value := deletedAt.Time
		folder.DeletedAt = &value
	}

	return &folder, nil
}

func (r *FolderRepository) FindByID(
	ctx context.Context,
	folderID int64,
) (*models.UserFolder, error) {
	query := `
		SELECT
			id,
			user_id,
			parent_id,
			folder_name,
			created_at,
			updated_at,
			deleted_at
		FROM user_folder
		WHERE id = ?
		  AND deleted_at IS NULL
		LIMIT 1
	`

	var folder models.UserFolder
	var parentID sql.NullInt64
	var deletedAt sql.NullTime

	err := r.db.QueryRowContext(
		ctx,
		query,
		folderID,
	).Scan(
		&folder.ID,
		&folder.UserID,
		&parentID,
		&folder.FolderName,
		&folder.CreatedAt,
		&folder.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return nil, err
	}

	if parentID.Valid {
		value := parentID.Int64
		folder.ParentID = &value
	}

	if deletedAt.Valid {
		value := deletedAt.Time
		folder.DeletedAt = &value
	}

	return &folder, nil
}

func (r *FolderRepository) Rename(
	ctx context.Context,
	userID int,
	folderID int64,
	folderName string,
) (bool, error) {
	query := `
		UPDATE user_folder
		SET
			updated_at = NOW(),
			folder_name = ?
		WHERE id = ?
			AND user_id = ?
			AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		folderName,
		folderID,
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

func (r *FolderRepository) MoveFolder(
	ctx context.Context,
	userID int,
	folderID int64,
	parentID *int64,
) error {
	query := `
		UPDATE user_folder
		SET
			updated_at = NOW(),
			parent_id = ?
		WHERE id = ?
			AND user_id = ?
			AND deleted_at IS NULL
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		parentID,
		folderID,
		userID,
	)

	return err
}

func (r *FolderRepository) SoftDelete(
	ctx context.Context,
	userID int,
	folderID int64,
) (bool, error) {
	query := `
		UPDATE user_folder
		SET
			updated_at = NOW(),
			deleted_at = NOW()
		WHERE id = ?
			AND user_id = ?
			AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		folderID,
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
