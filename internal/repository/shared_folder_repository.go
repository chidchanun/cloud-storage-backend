package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	// "time"

	"cloud-storage-backend/internal/models"
)

type SharedFolderRepository struct {
	db *sql.DB
}

func NewSharedFolderRespository(db *sql.DB) *SharedFolderRepository {
	return &SharedFolderRepository{
		db: db,
	}
}

func NewSharedFolderRepository(db *sql.DB) *SharedFolderRepository {
	return NewSharedFolderRespository(db)
}

func (r *SharedFolderRepository) Create(
	ctx context.Context,
	sharedFolder *models.SharedFolder,
) error {
	if sharedFolder == nil {
		return errors.New("แชร์โฟลเดอร์ห้ามเป็นค่าว่าง")
	}

	query := `
		INSERT INTO shared_folder (
			folder_id,
			shared_by_user_id,
			shared_with_user_id,
			permission,
			expires_at
		)
		VALUES (?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		sharedFolder.FolderID,
		sharedFolder.SharedByUserID,
		sharedFolder.SharedWithUserID,
		sharedFolder.Permission,
		sharedFolder.ExpiresAt,
	)

	if err != nil {
		return fmt.Errorf("create shared folder: %w", err)
	}

	// อ่าน ID ที่ฐานข้อมูลสร้างจาก AUTO_INCREMENT
	sharedFolderID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get shared folder ID: %w", err)
	}

	sharedFolder.ID = sharedFolderID

	queryTimestamp := `
		SELECT
			created_at,
			updated_at
		FROM shared_folder
		WHERE id = ?
		LIMIT 1
	`

	// อ่าน timestamp ที่ฐานข้อมูลสร้างกลับมาใส่ใน model
	err = r.db.QueryRowContext(
		ctx,
		queryTimestamp,
		sharedFolderID,
	).Scan(
		&sharedFolder.CreatedAt,
		&sharedFolder.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("get shared folder timestamps: %w", err)
	}

	return nil
}

func (r *SharedFolderRepository) FindAllUserPermissionInFolder(
	ctx context.Context,
	folderID int64,
	ownerUserID int,
) ([]models.SharedFolderUserPermission, error) {
	query := `
		SELECT
			sf.id,
			sf.folder_id,
			sf.shared_with_user_id,
			u.first_name,
			u.last_name,
			u.email,
			u.picture_path,
			sf.permission,
			sf.expires_at,
			sf.created_at,
			sf.updated_at
		FROM shared_folder AS sf
		INNER JOIN user_folder AS uf
			ON uf.id = sf.folder_id
		INNER JOIN user AS u
			ON u.id = sf.shared_with_user_id
		WHERE sf.folder_id = ?
			AND uf.user_id = ?
			AND uf.deleted_at IS NULL
			AND sf.deleted_at IS NULL
			AND u.deleted_at IS NULL
		ORDER BY sf.created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, folderID, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissions := make([]models.SharedFolderUserPermission, 0)

	for rows.Next() {
		var item models.SharedFolderUserPermission
		var picturePath sql.NullString
		var expiresAt sql.NullTime

		if err := rows.Scan(
			&item.ID,
			&item.FolderID,
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

		permissions = append(permissions, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return permissions, nil
}

func (r *SharedFolderRepository) FindSharedFoldersByUserID(
	ctx context.Context,
	sharedWithUserID int,
) ([]models.SharedFolderListItem, error) {
	query := `
		SELECT
			sf.id,
			uf.id,
			uf.folder_name,
			sf.permission,
			sf.expires_at
		FROM shared_folder AS sf
		INNER JOIN user_folder AS uf
			ON uf.id = sf.folder_id
		WHERE sf.shared_with_user_id = ?
			AND sf.deleted_at IS NULL
			AND uf.deleted_at IS NULL
			AND (
				sf.expires_at IS NULL
				OR sf.expires_at > NOW()
			)
		ORDER BY sf.created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, sharedWithUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	folders := make([]models.SharedFolderListItem, 0)

	for rows.Next() {
		var folder models.SharedFolderListItem
		var expiresAt sql.NullTime

		if err := rows.Scan(
			&folder.ID,
			&folder.FolderID,
			&folder.FolderName,
			&folder.Permission,
			&expiresAt,
		); err != nil {
			return nil, err
		}

		if expiresAt.Valid {
			value := expiresAt.Time
			folder.ExpiresAt = &value
		}

		folders = append(folders, folder)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return folders, nil
}

func (r *SharedFolderRepository) FindPermissionByFolderIDAndUserID(
	ctx context.Context,
	folderID int64,
	userID int,
) (string, error) {
	query := `
		SELECT permission
		FROM shared_folder
		WHERE folder_id = ?
			AND shared_with_user_id = ?
			AND deleted_at IS NULL
			AND (
				expires_at IS NULL
				OR expires_at > NOW()
			)
		LIMIT 1
	`

	var permission string
	err := r.db.QueryRowContext(ctx, query, folderID, userID).Scan(&permission)
	if err != nil {
		return "", err
	}

	return permission, nil
}

func (r *SharedFolderRepository) FindPermissionByFolderTreeAndUserID(
	ctx context.Context,
	folderID int64,
	userID int,
) (string, error) {
	query := `
		WITH RECURSIVE folder_tree AS (
			SELECT
				id,
				parent_id
			FROM user_folder
			WHERE id = ?
				AND deleted_at IS NULL

			UNION ALL

			SELECT
				parent.id,
				parent.parent_id
			FROM user_folder AS parent
			INNER JOIN folder_tree AS child
				ON child.parent_id = parent.id
			WHERE parent.deleted_at IS NULL
		)
		SELECT sf.permission
		FROM shared_folder AS sf
		INNER JOIN folder_tree AS tree
			ON tree.id = sf.folder_id
		WHERE sf.shared_with_user_id = ?
			AND sf.deleted_at IS NULL
			AND (
				sf.expires_at IS NULL
				OR sf.expires_at > NOW()
			)
		ORDER BY sf.created_at DESC
		LIMIT 1
	`

	var permission string
	err := r.db.QueryRowContext(ctx, query, folderID, userID).Scan(&permission)
	if err != nil {
		return "", err
	}

	return permission, nil
}

func (r *SharedFolderRepository) UpdatePermissionUserFolder(
	ctx context.Context,
	sharedFolder *models.SharedFolder,
) (bool, error) {
	query := `
		UPDATE shared_folder
		SET
			permission = ?,
			expires_at = ?,
			updated_at = NOW()
		WHERE folder_id = ?
			AND shared_by_user_id = ?
			AND shared_with_user_id = ?
			AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		sharedFolder.Permission,
		sharedFolder.ExpiresAt,
		sharedFolder.FolderID,
		sharedFolder.SharedByUserID,
		sharedFolder.SharedWithUserID,
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

func (r *SharedFolderRepository) RemoveSharedUser(
	ctx context.Context,
	sharedFolderID int64,
	sharedByUserID int,
	sharedWithUserID int,
) (bool, error) {
	query := `
		UPDATE shared_folder
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
		sharedFolderID,
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
