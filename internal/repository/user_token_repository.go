package repository

import (
	"context"
	"database/sql"

	"cloud-storage-backend/internal/models"
)

type UserTokenRepository struct {
	db *sql.DB
}

func NewUserTokenRepository(
	db *sql.DB,
) *UserTokenRepository {
	return &UserTokenRepository{
		db: db,
	}
}

// Upsert สร้าง session ใหม่ถ้ายังไม่มีข้อมูลของ user
// แต่ถ้ามี user_id อยู่แล้ว จะอัปเดต session เดิมแทน
func (r *UserTokenRepository) Upsert(
	ctx context.Context,
	userToken *models.UserToken,
) error {
	query := `
		INSERT INTO user_token (
			user_id,
			token_hash,
			expired_at,
			revoked_at
		)
		VALUES (?, ?, ?, NULL) AS new
		ON DUPLICATE KEY UPDATE
			token_hash = new.token_hash,
			expired_at = new.expired_at,
			revoked_at = NULL,
			updated_at = NOW()
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		userToken.UserID,
		userToken.TokenHash,
		userToken.ExpiredAt,
	)

	return err
}

// IsValid ใช้ตรวจว่า session ยังมีอยู่ ยังไม่หมดอายุ และยังไม่ถูก revoke
func (r *UserTokenRepository) IsValid(
	ctx context.Context,
	tokenHash string,
) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM user_token
			WHERE token_hash = ?
			  AND revoked_at IS NULL
			  AND expired_at > UTC_TIMESTAMP()
		)
	`

	var exists bool

	err := r.db.QueryRowContext(
		ctx,
		query,
		tokenHash,
	).Scan(&exists)

	return exists, err
}

// RevokeByTokenHash ใช้ยกเลิก session จาก token hash
func (r *UserTokenRepository) RevokeByTokenHash(
	ctx context.Context,
	tokenHash string,
) error {
	query := `
		UPDATE user_token
		SET revoked_at = NOW()
		WHERE token_hash = ?
		  AND revoked_at IS NULL
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		tokenHash,
	)

	return err
}
