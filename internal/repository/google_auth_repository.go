package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"cloud-storage-backend/internal/models"
)

var (
	// ErrInvalidGoogleIdentity หมายถึงข้อมูลจาก Google ไม่ครบ
	ErrInvalidGoogleIdentity = errors.New(
		"invalid Google identity",
	)

	// ErrGoogleEmailNotVerified หมายถึง Google ยังไม่ยืนยัน email
	ErrGoogleEmailNotVerified = errors.New(
		"Google email is not verified",
	)

	// ErrGoogleAccountConflict หมายถึง user นี้ผูก Google
	// บัญชีอื่นอยู่แล้ว
	ErrGoogleAccountConflict = errors.New(
		"user is already linked to another Google account",
	)
)

// rowQuerier ทำให้ helper ใช้ได้ทั้ง *sql.DB และ *sql.Tx
type rowQuerier interface {
	QueryRowContext(
		ctx context.Context,
		query string,
		args ...any,
	) *sql.Row
}

// GoogleAuthRepository จัดการ user และ Google provider
type GoogleAuthRepository struct {
	db *sql.DB
}

// NewGoogleAuthRepository สร้าง GoogleAuthRepository
func NewGoogleAuthRepository(
	db *sql.DB,
) *GoogleAuthRepository {
	return &GoogleAuthRepository{
		db: db,
	}
}

// FindUserByGoogleSubject ค้นหา user จาก Google sub
func (r *GoogleAuthRepository) FindUserByGoogleSubject(
	ctx context.Context,
	subject string,
) (*models.User, error) {
	subject = strings.TrimSpace(subject)

	if subject == "" {
		return nil, ErrInvalidGoogleIdentity
	}

	return findUserByGoogleSubject(
		ctx,
		r.db,
		subject,
	)
}

// FindOrCreateGoogleUser ใช้หา user เดิมหรือสร้าง user ใหม่
//
// ลำดับ:
//  1. ค้นหาจาก Google sub
//  2. ถ้าไม่พบ ให้ค้นหาจาก email
//  3. ถ้าพบ email ให้ผูก Google กับ user เดิม
//  4. ถ้าไม่พบ email ให้สร้าง user และ provider ใหม่
func (r *GoogleAuthRepository) FindOrCreateGoogleUser(
	ctx context.Context,
	identity *models.GoogleIdentity,
) (*models.User, error) {
	if identity == nil {
		return nil, ErrInvalidGoogleIdentity
	}

	subject := strings.TrimSpace(identity.Subject)
	email := strings.ToLower(
		strings.TrimSpace(identity.Email),
	)

	if subject == "" || email == "" {
		return nil, ErrInvalidGoogleIdentity
	}

	if !identity.EmailVerified {
		return nil, ErrGoogleEmailNotVerified
	}

	// เริ่ม Transaction เพื่อให้การสร้าง user และ provider
	// สำเร็จหรือยกเลิกพร้อมกัน
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf(
			"begin Google login transaction: %w",
			err,
		)
	}

	// ถ้า function ออกจากการทำงานก่อน Commit
	// ให้ยกเลิก Transaction อัตโนมัติ
	defer func() {
		_ = tx.Rollback()
	}()

	// ขั้นแรกค้นหาจาก Google sub
	user, err := findUserByGoogleSubject(
		ctx,
		tx,
		subject,
	)

	if err == nil {
		// อัปเดต email ล่าสุดที่ Google ส่งมา
		if err := updateProviderEmail(
			ctx,
			tx,
			subject,
			email,
		); err != nil {
			return nil, err
		}

		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf(
				"commit existing Google user: %w",
				err,
			)
		}

		return user, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf(
			"find user by Google subject: %w",
			err,
		)
	}

	// ยังไม่มี Google sub นี้ในระบบ
	// ลองค้นหา user เดิมจาก email
	user, err = findUserByEmailForGoogle(
		ctx,
		tx,
		email,
	)

	if err == nil {
		// ตรวจว่า user เดิมผูก Google บัญชีอื่นอยู่หรือไม่
		linkedSubject, findProviderErr :=
			findGoogleSubjectByUserID(
				ctx,
				tx,
				user.ID,
			)

		switch {
		case findProviderErr == nil:
			// User มี Google provider อยู่แล้ว
			// แต่ sub ไม่ตรงกับบัญชีที่กำลัง Login
			if linkedSubject != subject {
				return nil, ErrGoogleAccountConflict
			}

		case errors.Is(findProviderErr, sql.ErrNoRows):
			// ยังไม่มี Google provider
			// ผูก Google account กับ user เดิม
			if err := insertGoogleProvider(
				ctx,
				tx,
				user.ID,
				subject,
				email,
			); err != nil {
				return nil, err
			}

		default:
			return nil, fmt.Errorf(
				"find linked Google provider: %w",
				findProviderErr,
			)
		}

		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf(
				"commit linked Google user: %w",
				err,
			)
		}

		return user, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf(
			"find user by Google email: %w",
			err,
		)
	}

	// ไม่พบทั้ง Google sub และ email
	// จึงสร้าง user ใหม่
	firstName, lastName := googleIdentityNames(
		identity,
		email,
	)

	user = &models.User{
		FirstName: firstName,
		LastName:  lastName,
		Email:     email,

		// ยังไม่กำหนดรหัสผ่านและเบอร์โทร
		Phone:        nil,
		PasswordHash: nil,

		// ยังไม่บันทึก Google picture URL ลง picture_path
		// เพราะ picture_path เดิมอาจใช้สำหรับ path ภายใน server
		PicturePath: nil,
	}

	if err := insertGoogleUser(
		ctx,
		tx,
		user,
	); err != nil {
		return nil, err
	}

	if err := insertGoogleProvider(
		ctx,
		tx,
		user.ID,
		subject,
		email,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf(
			"commit new Google user: %w",
			err,
		)
	}

	return user, nil
}

// findUserByGoogleSubject ค้นหา user ผ่าน provider_subject
func findUserByGoogleSubject(
	ctx context.Context,
	queryer rowQuerier,
	subject string,
) (*models.User, error) {
	query := `
		SELECT
			u.id,
			u.first_name,
			u.last_name,
			u.email,
			u.email_verified_at,
			u.picture_path,
			u.phone,
			u.password_hash
		FROM user_auth_provider AS provider
		INNER JOIN ` + "`user`" + ` AS u
			ON u.id = provider.user_id
		WHERE provider.provider = ?
		  AND provider.provider_subject = ?
		  AND u.deleted_at IS NULL
		LIMIT 1
	`

	row := queryer.QueryRowContext(
		ctx,
		query,
		models.AuthProviderGoogle,
		subject,
	)

	return scanGoogleUser(row)
}

// findUserByEmailForGoogle ค้นหา user เดิมจาก email
func findUserByEmailForGoogle(
	ctx context.Context,
	queryer rowQuerier,
	email string,
) (*models.User, error) {
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

	row := queryer.QueryRowContext(
		ctx,
		query,
		email,
	)

	return scanGoogleUser(row)
}

// scanGoogleUser แปลงผลลัพธ์จาก SQL เป็น User Model
func scanGoogleUser(
	row *sql.Row,
) (*models.User, error) {
	var user models.User

	var picturePath sql.NullString
	var phone sql.NullString
	var passwordHash sql.NullString
	var emailVerifiedAt sql.NullTime

	err := row.Scan(
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

	return &user, nil
}

// findGoogleSubjectByUserID ตรวจว่า user ผูก Google อยู่แล้วหรือไม่
func findGoogleSubjectByUserID(
	ctx context.Context,
	queryer rowQuerier,
	userID int,
) (string, error) {
	query := `
		SELECT provider_subject
		FROM user_auth_provider
		WHERE user_id = ?
		  AND provider = ?
		LIMIT 1
	`

	var subject string

	err := queryer.QueryRowContext(
		ctx,
		query,
		userID,
		models.AuthProviderGoogle,
	).Scan(&subject)

	if err != nil {
		return "", err
	}

	return subject, nil
}

// insertGoogleUser สร้าง user ใหม่จากข้อมูล Google
func insertGoogleUser(
	ctx context.Context,
	tx *sql.Tx,
	user *models.User,
) error {
	query := `
		INSERT INTO ` + "`user`" + ` (
			first_name,
			last_name,
			email,
			email_verified_at,
			picture_path,
			phone,
			password_hash,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, NOW(), NULL, NULL, NULL, NOW(), NOW())
	`

	result, err := tx.ExecContext(
		ctx,
		query,
		user.FirstName,
		user.LastName,
		user.Email,
	)
	if err != nil {
		return fmt.Errorf(
			"create Google user: %w",
			err,
		)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf(
			"get created Google user ID: %w",
			err,
		)
	}

	user.ID = int(userID)

	return nil
}

// insertGoogleProvider ผูก Google sub เข้ากับ user
func insertGoogleProvider(
	ctx context.Context,
	tx *sql.Tx,
	userID int,
	subject string,
	email string,
) error {
	query := `
		INSERT INTO user_auth_provider (
			user_id,
			provider,
			provider_subject,
			provider_email
		)
		VALUES (?, ?, ?, ?)
	`

	_, err := tx.ExecContext(
		ctx,
		query,
		userID,
		models.AuthProviderGoogle,
		subject,
		email,
	)
	if err != nil {
		return fmt.Errorf(
			"create Google auth provider: %w",
			err,
		)
	}

	return nil
}

// updateProviderEmail อัปเดต email ล่าสุดจาก Google
func updateProviderEmail(
	ctx context.Context,
	tx *sql.Tx,
	subject string,
	email string,
) error {
	query := `
		UPDATE user_auth_provider
		SET
			provider_email = ?,
			updated_at = NOW()
		WHERE provider = ?
		  AND provider_subject = ?
	`

	_, err := tx.ExecContext(
		ctx,
		query,
		email,
		models.AuthProviderGoogle,
		subject,
	)
	if err != nil {
		return fmt.Errorf(
			"update Google provider email: %w",
			err,
		)
	}

	return nil
}

// googleIdentityNames เตรียมชื่อสำหรับ user ใหม่
func googleIdentityNames(
	identity *models.GoogleIdentity,
	email string,
) (string, string) {
	firstName := strings.TrimSpace(identity.FirstName)
	lastName := strings.TrimSpace(identity.LastName)

	// บางบัญชีอาจไม่มี given_name หรือ family_name
	if firstName == "" {
		fullNameParts := strings.Fields(
			strings.TrimSpace(identity.FullName),
		)

		if len(fullNameParts) > 0 {
			firstName = fullNameParts[0]
		}

		if lastName == "" && len(fullNameParts) > 1 {
			lastName = strings.Join(
				fullNameParts[1:],
				" ",
			)
		}
	}

	// ตารางกำหนด first_name เป็น NOT NULL
	// จึงใช้ส่วนก่อน @ เป็น fallback
	if firstName == "" {
		emailParts := strings.SplitN(email, "@", 2)
		firstName = emailParts[0]
	}

	return firstName, lastName
}
