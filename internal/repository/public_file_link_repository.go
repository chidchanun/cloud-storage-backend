package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cloud-storage-backend/internal/models"
)

type PublicFileLinkRepository struct {
	db *sql.DB
}

func NewPublicFileLinkRepository(db *sql.DB) *PublicFileLinkRepository {
	return &PublicFileLinkRepository{
		db: db,
	}
}

// Create stores only a SHA-256 hash of the share token.
// The raw token is returned to the user once and is never persisted.
func (r *PublicFileLinkRepository) Create(
	ctx context.Context,
	link *models.PublicFileLink,
) error {
	if link == nil {
		return errors.New("public file link must not be nil")
	}

	query := `
		INSERT INTO public_file_link (
			file_id,
			user_id,
			token_hash,
			permission,
			expires_at
		)
		VALUES (?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		link.FileID,
		link.UserID,
		link.TokenHash,
		link.Permission,
		link.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("create public file link: %w", err)
	}

	linkID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get public file link ID: %w", err)
	}

	link.ID = linkID

	queryTimestamp := `
		SELECT
			created_at,
			updated_at
		FROM public_file_link
		WHERE id = ?
		LIMIT 1
	`

	err = r.db.QueryRowContext(
		ctx,
		queryTimestamp,
		link.ID,
	).Scan(
		&link.CreatedAt,
		&link.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("get public file link timestamps: %w", err)
	}

	return nil
}

func (r *PublicFileLinkRepository) FindActiveByTokenHash(
	ctx context.Context,
	tokenHash string,
) (*models.PublicFileLink, *models.UserFile, error) {
	query := `
		SELECT
			pfl.id,
			pfl.file_id,
			pfl.user_id,
			pfl.token_hash,
			pfl.permission,
			pfl.expires_at,
			pfl.created_at,
			pfl.updated_at,
			pfl.deleted_at,
			uf.id,
			uf.user_id,
			uf.folder_id,
			uf.original_name,
			uf.stored_name,
			uf.storage_path,
			uf.mime_type,
			uf.size_bytes,
			uf.checksum_sha256,
			uf.created_at,
			uf.updated_at,
			uf.deleted_at
		FROM public_file_link AS pfl
		INNER JOIN user_file AS uf
			ON uf.id = pfl.file_id
		WHERE pfl.token_hash = ?
			AND pfl.deleted_at IS NULL
			AND uf.deleted_at IS NULL
			AND (
				pfl.expires_at IS NULL
				OR pfl.expires_at > NOW()
			)
		LIMIT 1
	`

	var link models.PublicFileLink
	var file models.UserFile
	var linkExpiresAt sql.NullTime
	var linkDeletedAt sql.NullTime
	var folderID sql.NullInt64
	var checksum sql.NullString
	var fileDeletedAt sql.NullTime

	err := r.db.QueryRowContext(
		ctx,
		query,
		tokenHash,
	).Scan(
		&link.ID,
		&link.FileID,
		&link.UserID,
		&link.TokenHash,
		&link.Permission,
		&linkExpiresAt,
		&link.CreatedAt,
		&link.UpdatedAt,
		&linkDeletedAt,
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
		&fileDeletedAt,
	)
	if err != nil {
		return nil, nil, err
	}

	if linkExpiresAt.Valid {
		value := linkExpiresAt.Time
		link.ExpiresAt = &value
	}

	if linkDeletedAt.Valid {
		value := linkDeletedAt.Time
		link.DeletedAt = &value
	}

	if folderID.Valid {
		value := folderID.Int64
		file.FolderID = &value
	}

	if checksum.Valid {
		value := checksum.String
		file.ChecksumSHA256 = &value
	}

	if fileDeletedAt.Valid {
		value := fileDeletedAt.Time
		file.DeletedAt = &value
	}

	return &link, &file, nil
}
