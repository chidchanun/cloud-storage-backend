package repository

import (
	"context"
	"database/sql"

	"cloud-storage-backend/internal/models"
)

type UserFolderStarRepository struct {
	db *sql.DB
}

func NewUserFolderStarRepository(db *sql.DB) *UserFolderStarRepository {
	return &UserFolderStarRepository{
		db: db,
	}
}

// StarFolder adds a star only when the folder belongs to the current user.
func (r *UserFolderStarRepository) StarFolder(
	ctx context.Context,
	userID int,
	folderID int64,
) (*models.UserFolderStar, error) {
	query := `
		INSERT IGNORE INTO user_folder_star (
			user_id,
			folder_id
		)
		SELECT
			?,
			uf.id
		FROM user_folder AS uf
		WHERE uf.id = ?
		  AND uf.user_id = ?
		  AND uf.deleted_at IS NULL
	`

	if _, err := r.db.ExecContext(ctx, query, userID, folderID, userID); err != nil {
		return nil, err
	}

	return r.FindByUserIDAndFolderID(ctx, userID, folderID)
}

func (r *UserFolderStarRepository) UnstarFolder(
	ctx context.Context,
	userID int,
	folderID int64,
) (bool, error) {
	query := `
		DELETE FROM user_folder_star
		WHERE user_id = ?
		  AND folder_id = ?
	`

	result, err := r.db.ExecContext(ctx, query, userID, folderID)
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rowsAffected > 0, nil
}

func (r *UserFolderStarRepository) FindByUserIDAndFolderID(
	ctx context.Context,
	userID int,
	folderID int64,
) (*models.UserFolderStar, error) {
	query := `
		SELECT
			star.id,
			star.user_id,
			star.folder_id,
			star.created_at
		FROM user_folder_star AS star
		INNER JOIN user_folder AS uf
			ON uf.id = star.folder_id
		WHERE star.user_id = ?
		  AND star.folder_id = ?
		  AND uf.user_id = ?
		  AND uf.deleted_at IS NULL
		LIMIT 1
	`

	var star models.UserFolderStar

	err := r.db.QueryRowContext(ctx, query, userID, folderID, userID).Scan(
		&star.ID,
		&star.UserID,
		&star.FolderID,
		&star.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &star, nil
}

func (r *UserFolderStarRepository) IsFolderStarred(
	ctx context.Context,
	userID int,
	folderID int64,
) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM user_folder_star AS star
			INNER JOIN user_folder AS uf
				ON uf.id = star.folder_id
			WHERE star.user_id = ?
			  AND star.folder_id = ?
			  AND uf.user_id = ?
			  AND uf.deleted_at IS NULL
		)
	`

	var isStarred bool
	err := r.db.QueryRowContext(ctx, query, userID, folderID, userID).Scan(&isStarred)
	if err != nil {
		return false, err
	}

	return isStarred, nil
}

func (r *UserFolderStarRepository) FindAllByUserID(
	ctx context.Context,
	userID int,
) ([]models.StarredFolderListItem, error) {
	query := `
		SELECT
			star.id AS star_id,
			uf.id AS folder_id,
			uf.parent_id,
			uf.folder_name,
			uf.created_at AS folder_created_at,
			uf.updated_at AS folder_updated_at,
			star.created_at AS starred_at
		FROM user_folder_star AS star
		INNER JOIN user_folder AS uf
			ON uf.id = star.folder_id
		WHERE star.user_id = ?
		  AND uf.user_id = ?
		  AND uf.deleted_at IS NULL
		ORDER BY star.created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.StarredFolderListItem, 0)

	for rows.Next() {
		var item models.StarredFolderListItem
		var parentID sql.NullInt64

		if err := rows.Scan(
			&item.StarID,
			&item.FolderID,
			&parentID,
			&item.FolderName,
			&item.FolderCreatedAt,
			&item.FolderUpdatedAt,
			&item.StarredAt,
		); err != nil {
			return nil, err
		}

		if parentID.Valid {
			value := parentID.Int64
			item.ParentID = &value
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}
