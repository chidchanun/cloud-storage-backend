package repository

import (
	"context"
	"database/sql"
	"time"

	"cloud-storage-backend/internal/models"
)

type EmailVerificationRepository struct {
	db *sql.DB
}

func NewEmailVerificationRepository(db *sql.DB) *EmailVerificationRepository {
	return &EmailVerificationRepository{
		db: db,
	}
}

func (r *EmailVerificationRepository) Create(
	ctx context.Context,
	token *models.EmailVerificationToken,
) error {
	query := `
		INSERT INTO email_verification_token (
			user_id,
			token_hash,
			expires_at
		)
		VALUES (?, ?, ?)
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		token.UserID,
		token.TokenHash,
		token.ExpiresAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	token.ID = int(id)

	return nil
}

func (r *EmailVerificationRepository) FindValidByTokenHash(
	ctx context.Context,
	tokenHash string,
) (*models.EmailVerificationToken, error) {
	query := `
		SELECT
			id,
			user_id,
			token_hash,
			expires_at,
			used_at,
			created_at
		FROM email_verification_token
		WHERE token_hash = ?
		  AND used_at IS NULL
		  AND expires_at > UTC_TIMESTAMP()
		LIMIT 1
	`

	var token models.EmailVerificationToken
	var usedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&usedAt,
		&token.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if usedAt.Valid {
		token.UsedAt = &usedAt.Time
	}

	return &token, nil
}

func (r *EmailVerificationRepository) MarkUsed(
	ctx context.Context,
	tokenID int,
) error {
	query := `
		UPDATE email_verification_token
		SET used_at = ?
		WHERE id = ?
		  AND used_at IS NULL
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		time.Now().UTC(),
		tokenID,
	)

	return err
}
