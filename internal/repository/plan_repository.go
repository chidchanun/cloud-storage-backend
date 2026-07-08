package repository

import (
	"context"
	"database/sql"

	"cloud-storage-backend/internal/models"
)

type PlanRepository struct {
	db *sql.DB
}

func NewPlanRepository(db *sql.DB) *PlanRepository {
	return &PlanRepository{db: db}
}

func (r *PlanRepository) FindAllActive(ctx context.Context) ([]models.Plan, error) {
	query := `
		SELECT
			id,
			plan_name,
			plan_code,
			description,
			storage_limit_bytes,
			max_file_size_bytes,
			max_files,
			max_share_users_per_file,
			price,
			billing_cycle,
			is_active,
			created_at,
			updated_at,
			deleted_at
		FROM plan
		WHERE is_active = 1
			AND deleted_at IS NULL
		ORDER BY
			CASE billing_cycle
				WHEN 'free' THEN 1
				WHEN 'monthly' THEN 2
				WHEN 'yearly' THEN 3
				ELSE 4
			END,
			price ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	plans := make([]models.Plan, 0)
	for rows.Next() {
		plan, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}

		plans = append(plans, plan)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return plans, nil
}

func (r *PlanRepository) FindActiveByCode(ctx context.Context, planCode string) (*models.Plan, error) {
	query := `
		SELECT
			id,
			plan_name,
			plan_code,
			description,
			storage_limit_bytes,
			max_file_size_bytes,
			max_files,
			max_share_users_per_file,
			price,
			billing_cycle,
			is_active,
			created_at,
			updated_at,
			deleted_at
		FROM plan
		WHERE plan_code = ?
			AND is_active = 1
			AND deleted_at IS NULL
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, planCode)
	plan, err := scanPlan(row)
	if err != nil {
		return nil, err
	}

	return &plan, nil
}

type planScanner interface {
	Scan(dest ...any) error
}

func scanPlan(scanner planScanner) (models.Plan, error) {
	var plan models.Plan
	var description sql.NullString
	var maxFiles sql.NullInt64
	var maxShareUsersPerFile sql.NullInt64
	var deletedAt sql.NullTime

	err := scanner.Scan(
		&plan.ID,
		&plan.PlanName,
		&plan.PlanCode,
		&description,
		&plan.StorageLimitBytes,
		&plan.MaxFileSizeBytes,
		&maxFiles,
		&maxShareUsersPerFile,
		&plan.Price,
		&plan.BillingCycle,
		&plan.IsActive,
		&plan.CreatedAt,
		&plan.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return models.Plan{}, err
	}

	if description.Valid {
		value := description.String
		plan.Description = &value
	}

	if maxFiles.Valid {
		value := uint32(maxFiles.Int64)
		plan.MaxFiles = &value
	}

	if maxShareUsersPerFile.Valid {
		value := uint32(maxShareUsersPerFile.Int64)
		plan.MaxShareUsersPerFile = &value
	}

	if deletedAt.Valid {
		value := deletedAt.Time
		plan.DeletedAt = &value
	}

	return plan, nil
}
