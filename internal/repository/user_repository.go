package repository

import (
	"context"
	"database/sql"
	"time"

	"cloud-storage-backend/internal/models"
)

// UserRepository ใช้จัดการ query ที่เกี่ยวกับ user
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository สร้าง UserRepository
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

// FindByEmail ใช้ค้นหา user จาก email
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	var picturePath sql.NullString
	var phone sql.NullString
	var passwordHash sql.NullString
	var emailVerifiedAt sql.NullTime

	query := `
		SELECT
			id,
			first_name,
			last_name,
			email,
			email_verified_at,
			picture_path,
			phone,
			password_hash
		FROM ` + "`user`" + `
		WHERE email = ?
		AND deleted_at IS NULL
		LIMIT 1
	`

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&emailVerifiedAt,
		&picturePath,
		&phone,
		&passwordHash,
	)

	if err != nil {
		return nil, err
	}

	if picturePath.Valid {
		path := picturePath.String
		user.PicturePath = &path
	}

	if phone.Valid {
		value := phone.String
		user.Phone = &value
	}

	if passwordHash.Valid {
		value := passwordHash.String
		user.PasswordHash = &value
	}

	if emailVerifiedAt.Valid {
		value := emailVerifiedAt.Time
		user.EmailVerifiedAt = &value
	}

	return &user, nil
}

// Create ใช้เพิ่ม user ใหม่ลง database
func (r *UserRepository) FindByID(ctx context.Context, userID int) (*models.User, error) {
	var user models.User
	var picturePath sql.NullString
	var phone sql.NullString
	var passwordHash sql.NullString
	var emailVerifiedAt sql.NullTime

	query := `
		SELECT
			id,
			first_name,
			last_name,
			email,
			email_verified_at,
			picture_path,
			phone,
			password_hash
		FROM ` + "`user`" + `
		WHERE id = ?
		AND deleted_at IS NULL
		LIMIT 1
	`

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&emailVerifiedAt,
		&picturePath,
		&phone,
		&passwordHash,
	)

	if err != nil {
		return nil, err
	}

	if picturePath.Valid {
		path := picturePath.String
		user.PicturePath = &path
	}

	if phone.Valid {
		value := phone.String
		user.Phone = &value
	}

	if passwordHash.Valid {
		value := passwordHash.String
		user.PasswordHash = &value
	}

	if emailVerifiedAt.Valid {
		value := emailVerifiedAt.Time
		user.EmailVerifiedAt = &value
	}

	return &user, nil
}

func (r *UserRepository) SearchByEmailOrName(
	ctx context.Context,
	keyword string,
	excludeUserID int,
	limit int,
) ([]models.User, error) {
	if limit <= 0 || limit > 20 {
		limit = 8
	}

	query := `
		SELECT
			id,
			first_name,
			last_name,
			email,
			email_verified_at,
			picture_path,
			phone,
			password_hash
		FROM ` + "`user`" + `
		WHERE deleted_at IS NULL
		  AND id <> ?
		  AND (
		    email LIKE ?
		    OR first_name LIKE ?
		    OR last_name LIKE ?
		  )
		ORDER BY email ASC
		LIMIT ?
	`

	searchValue := "%" + keyword + "%"

	rows, err := r.db.QueryContext(
		ctx,
		query,
		excludeUserID,
		searchValue,
		searchValue,
		searchValue,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]models.User, 0)

	for rows.Next() {
		var user models.User
		var picturePath sql.NullString
		var phone sql.NullString
		var passwordHash sql.NullString
		var emailVerifiedAt sql.NullTime

		if err := rows.Scan(
			&user.ID,
			&user.FirstName,
			&user.LastName,
			&user.Email,
			&emailVerifiedAt,
			&picturePath,
			&phone,
			&passwordHash,
		); err != nil {
			return nil, err
		}

		if picturePath.Valid {
			value := picturePath.String
			user.PicturePath = &value
		}

		if phone.Valid {
			value := phone.String
			user.Phone = &value
		}

		if passwordHash.Valid {
			value := passwordHash.String
			user.PasswordHash = &value
		}

		if emailVerifiedAt.Valid {
			value := emailVerifiedAt.Time
			user.EmailVerifiedAt = &value
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO ` + "`user`" + ` (
			first_name,
			last_name,
			email,
			phone,
			password_hash,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		user.FirstName,
		user.LastName,
		user.Email,
		user.Phone,
		user.PasswordHash,
	)

	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	user.ID = int(id)

	return nil
}

func (r *UserRepository) UpdateUserProfile(
	ctx context.Context,
	userID int,
	user *models.UpdateUserProfileRequest,
) (bool, error) {
	query := `
		UPDATE user
		SET
			first_name = COALESCE(?, first_name),
			last_name = COALESCE(?, last_name),
			email = COALESCE(?, email),
			phone = COALESCE(?, phone),
			picture_path = COALESCE(?, picture_path),
			updated_at = NOW()
		WHERE id = ?
			AND deleted_at IS NULL
	`

	optionalString := func(value *string) any {
		if value == nil {
			return nil
		}

		return *value
	}

	result, err := r.db.ExecContext(
		ctx,
		query,
		optionalString(user.FirstName),
		optionalString(user.LastName),
		optionalString(user.Email),
		optionalString(user.Phone),
		optionalString(user.PicturePath),
		userID,
	)
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	if rowsAffected == 0 {
		return false, nil
	}

	return true, nil
}

func (r *UserRepository) MarkEmailVerified(
	ctx context.Context,
	userID int,
) error {
	query := `
		UPDATE ` + "`user`" + `
		SET email_verified_at = COALESCE(email_verified_at, ?),
		    updated_at = NOW()
		WHERE id = ?
		  AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		time.Now().UTC(),
		userID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *UserRepository) SetPasswordHash(
	ctx context.Context,
	userID int,
	passwordHash string,
) (*models.User, error) {
	query := `
		UPDATE ` + "`user`" + `
		SET password_hash = ?,
		    updated_at = NOW()
		WHERE id = ?
		  AND password_hash IS NULL
		  AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, passwordHash, userID)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, sql.ErrNoRows
	}

	return r.FindByID(ctx, userID)
}

func (r *UserRepository) UpdatePicturePath(
	ctx context.Context,
	userID int,
	picturePath string,
) (*models.User, error) {
	query := `
		UPDATE ` + "`user`" + `
		SET picture_path = ?,
		    updated_at = NOW()
		WHERE id = ?
		  AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, picturePath, userID)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, sql.ErrNoRows
	}

	return r.FindByID(ctx, userID)
}
