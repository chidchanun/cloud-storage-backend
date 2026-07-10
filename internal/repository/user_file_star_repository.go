package repository

import (
	"context"
	"database/sql"

	"cloud-storage-backend/internal/models"
)

type UserFileStarRepository struct {
	db *sql.DB
}

func NewUserFileStarRepository(db *sql.DB) *UserFileStarRepository {
	return &UserFileStarRepository{
		db: db,
	}
}

// StarFile กดไฟล์เป็นสำคัญ
//
// ใช้ INSERT ... SELECT เพื่อป้องกันไม่ให้ user กดดาวไฟล์ของคนอื่น
func (r *UserFileStarRepository) StarFile(
	ctx context.Context,
	userID int,
	fileID int64,
) (*models.UserFileStar, error) {
	query := `
		INSERT IGNORE INTO user_file_star (
			user_id,
			file_id
		)
		SELECT
			?,
			uf.id
		FROM user_file AS uf
		WHERE uf.id = ?
		  AND uf.user_id = ?
		  AND uf.deleted_at IS NULL
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		userID,
		fileID,
		userID,
	)
	if err != nil {
		return nil, err
	}

	return r.FindByUserIDAndFileID(
		ctx,
		userID,
		fileID,
	)
}

// UnstarFile ยกเลิกไฟล์สำคัญ
func (r *UserFileStarRepository) UnStarFile(
	ctx context.Context,
	userID int,
	fileID int64,
) (bool, error) {
	query := `
		DELETE FROM user_file_star
		WHERE user_id = ?
		  AND file_id = ?
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		userID,
		fileID,
	)
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, nil
	}

	return rowsAffected > 0, nil
}

// FindByUserIDAndFileID ค้นหารายการดาวของไฟล์
func (r *UserFileStarRepository) FindByUserIDAndFileID(
	ctx context.Context,
	userID int,
	fileID int64,
) (*models.UserFileStar, error) {
	query := `
		SELECT
			star.id,
			star.user_id,
			star.file_id,
			star.created_at
		FROM user_file_star AS star
		INNER JOIN user_file AS uf
			ON uf.id = star.file_id
		WHERE star.user_id = ?
		  AND star.file_id = ?
		  AND uf.user_id = ?
		  AND uf.deleted_at IS NULL
		LIMIT 1
	`

	var star models.UserFileStar

	err := r.db.QueryRowContext(
		ctx,
		query,
		userID,
		fileID,
		userID,
	).Scan(
		&star.ID,
		&star.UserID,
		&star.FileID,
		&star.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &star, nil
}

// IsFileStarred ตรวจว่าไฟล์นี้ถูกกดสำคัญแล้วหรือยัง
func (r *UserFileStarRepository) IsFileStarred(
	ctx context.Context,
	userID int,
	fileID int64,
) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM user_file_star AS star
			INNER JOIN user_file AS uf
				ON uf.id = star.file_id
			WHERE star.user_id = ?
			  AND star.file_id = ?
			  AND uf.user_id = ?
			  AND uf.deleted_at IS NULL
		)
	`

	var isStarred bool
	err := r.db.QueryRowContext(
		ctx,
		query,
		userID,
		fileID,
		userID,
	).Scan(
		&isStarred,
	)

	if err != nil {
		return false, err
	}

	return isStarred, nil
}

// FindAllByUserID ดึงรายการไฟล์สำคัญทั้งหมดของ user
func (r *UserFileStarRepository) FindAllByUserID(
	ctx context.Context,
	userID int,
) ([]models.StarredFileListItem, error) {
	query := `
		SELECT
			star.id AS star_id,
			uf.id AS file_id,
			uf.folder_id,
			uf.original_name,
			uf.mime_type,
			uf.size_bytes,
			uf.created_at AS file_created_at,
			uf.updated_at AS file_updated_at,
			star.created_at AS starred_at
		FROM user_file_star AS star
		INNER JOIN user_file AS uf
			ON uf.id = star.file_id
		WHERE star.user_id = ?
		  AND uf.user_id = ?
		  AND uf.deleted_at IS NULL
		ORDER BY star.created_at DESC
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
		userID,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.StarredFileListItem, 0)

	for rows.Next() {
		var item models.StarredFileListItem

		var folderID sql.NullInt64

		if err := rows.Scan(
			&item.StarID,
			&item.FileID,
			&folderID,
			&item.OriginalName,
			&item.MimeType,
			&item.SizeBytes,
			&item.FileCreatedAt,
			&item.FileUpdatedAt,
			&item.StarredAt,
		); err != nil {
			return nil, err
		}

		if folderID.Valid {
			value := folderID.Int64
			item.FolderID = &value
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}