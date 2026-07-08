package repository

import (
	"context"
	"database/sql"
	"time"

	"cloud-storage-backend/internal/models"
)

type UserPlanRepository struct {
	db *sql.DB
}

func NewUserPlanRepository(db *sql.DB) *UserPlanRepository {
	return &UserPlanRepository{db: db}
}

func (r *UserPlanRepository) FindCurrentByUserID(
	ctx context.Context,
	userID int,
) (*models.CurrentUserPlan, error) {
	query := `
		SELECT
			up.id,
			up.user_id,
			up.plan_id,
			up.status,
			up.started_at,
			up.expires_at,
			up.cancelled_at,
			up.auto_renew,
			p.plan_name,
			p.plan_code,
			p.description,
			p.storage_limit_bytes,
			p.max_file_size_bytes,
			p.max_files,
			p.max_share_users_per_file,
			p.price,
			p.billing_cycle
		FROM user_plan AS up
		INNER JOIN plan AS p
			ON p.id = up.plan_id
		WHERE up.user_id = ?
			AND up.deleted_at IS NULL
			AND up.status IN ('trial', 'active', 'past_due')
			AND p.deleted_at IS NULL
		LIMIT 1
	`

	return scanCurrentUserPlan(r.db.QueryRowContext(ctx, query, userID))
}

func (r *UserPlanRepository) AssignCurrentPlan(
	ctx context.Context,
	userID int,
	planID int,
	status string,
	expiresAt *time.Time,
	autoRenew bool,
) (*models.UserPlan, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(
		ctx,
		`
			UPDATE user_plan
			SET
				status = 'cancelled',
				cancelled_at = NOW(),
				deleted_at = NOW(),
				updated_at = NOW()
			WHERE user_id = ?
				AND deleted_at IS NULL
				AND status IN ('pending', 'trial', 'active', 'past_due')
		`,
		userID,
	)
	if err != nil {
		return nil, err
	}

	result, err := tx.ExecContext(
		ctx,
		`
			INSERT INTO user_plan (
				user_id,
				plan_id,
				status,
				expires_at,
				auto_renew
			)
			VALUES (?, ?, ?, ?, ?)
		`,
		userID,
		planID,
		status,
		expiresAt,
		autoRenew,
	)
	if err != nil {
		return nil, err
	}

	userPlanID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	userPlan, err := r.findByIDTx(ctx, tx, userPlanID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return userPlan, nil
}

func (r *UserPlanRepository) UsedStorageBytes(ctx context.Context, userID int) (uint64, error) {
	query := `
		SELECT COALESCE(SUM(size_bytes), 0)
		FROM user_file
		WHERE user_id = ?
			AND deleted_at IS NULL
	`

	var usedBytes uint64
	if err := r.db.QueryRowContext(ctx, query, userID).Scan(&usedBytes); err != nil {
		return 0, err
	}

	return usedBytes, nil
}

func (r *UserPlanRepository) UserFileCount(ctx context.Context, userID int) (uint64, error) {
	query := `
		SELECT COUNT(*)
		FROM user_file
		WHERE user_id = ?
			AND deleted_at IS NULL
	`

	var count uint64
	if err := r.db.QueryRowContext(ctx, query, userID).Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

func (r *UserPlanRepository) findByIDTx(
	ctx context.Context,
	tx *sql.Tx,
	userPlanID int64,
) (*models.UserPlan, error) {
	query := `
		SELECT
			id,
			user_id,
			plan_id,
			status,
			started_at,
			expires_at,
			cancelled_at,
			auto_renew,
			payment_provider,
			provider_subscription_id,
			created_at,
			updated_at,
			deleted_at,
			current_plan_key
		FROM user_plan
		WHERE id = ?
		LIMIT 1
	`

	return scanUserPlan(tx.QueryRowContext(ctx, query, userPlanID))
}

type userPlanScanner interface {
	Scan(dest ...any) error
}

func scanUserPlan(scanner userPlanScanner) (*models.UserPlan, error) {
	var userPlan models.UserPlan
	var expiresAt sql.NullTime
	var cancelledAt sql.NullTime
	var paymentProvider sql.NullString
	var providerSubscriptionID sql.NullString
	var deletedAt sql.NullTime
	var currentPlanKey sql.NullInt64

	err := scanner.Scan(
		&userPlan.ID,
		&userPlan.UserID,
		&userPlan.PlanID,
		&userPlan.Status,
		&userPlan.StartedAt,
		&expiresAt,
		&cancelledAt,
		&userPlan.AutoRenew,
		&paymentProvider,
		&providerSubscriptionID,
		&userPlan.CreatedAt,
		&userPlan.UpdatedAt,
		&deletedAt,
		&currentPlanKey,
	)
	if err != nil {
		return nil, err
	}

	if expiresAt.Valid {
		value := expiresAt.Time
		userPlan.ExpiresAt = &value
	}

	if cancelledAt.Valid {
		value := cancelledAt.Time
		userPlan.CancelledAt = &value
	}

	if paymentProvider.Valid {
		value := paymentProvider.String
		userPlan.PaymentProvider = &value
	}

	if providerSubscriptionID.Valid {
		value := providerSubscriptionID.String
		userPlan.ProviderSubscriptionID = &value
	}

	if deletedAt.Valid {
		value := deletedAt.Time
		userPlan.DeletedAt = &value
	}

	if currentPlanKey.Valid {
		value := int8(currentPlanKey.Int64)
		userPlan.CurrentPlanKey = &value
	}

	return &userPlan, nil
}

func scanCurrentUserPlan(scanner userPlanScanner) (*models.CurrentUserPlan, error) {
	var userPlan models.CurrentUserPlan
	var expiresAt sql.NullTime
	var cancelledAt sql.NullTime
	var description sql.NullString
	var maxFiles sql.NullInt64
	var maxShareUsersPerFile sql.NullInt64

	err := scanner.Scan(
		&userPlan.UserPlanID,
		&userPlan.UserID,
		&userPlan.PlanID,
		&userPlan.Status,
		&userPlan.StartedAt,
		&expiresAt,
		&cancelledAt,
		&userPlan.AutoRenew,
		&userPlan.PlanName,
		&userPlan.PlanCode,
		&description,
		&userPlan.StorageLimitBytes,
		&userPlan.MaxFileSizeBytes,
		&maxFiles,
		&maxShareUsersPerFile,
		&userPlan.Price,
		&userPlan.BillingCycle,
	)
	if err != nil {
		return nil, err
	}

	if expiresAt.Valid {
		value := expiresAt.Time
		userPlan.ExpiresAt = &value
	}

	if cancelledAt.Valid {
		value := cancelledAt.Time
		userPlan.CancelledAt = &value
	}

	if description.Valid {
		value := description.String
		userPlan.Description = &value
	}

	if maxFiles.Valid {
		value := uint32(maxFiles.Int64)
		userPlan.MaxFiles = &value
	}

	if maxShareUsersPerFile.Valid {
		value := uint32(maxShareUsersPerFile.Int64)
		userPlan.MaxShareUsersPerFile = &value
	}

	return &userPlan, nil
}
