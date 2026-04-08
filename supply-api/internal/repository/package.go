package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"lijiaoqiao/supply-api/internal/domain"
)

// PackageRepository 套餐仓储
type PackageRepository struct {
	pool *pgxpool.Pool
}

// NewPackageRepository 创建套餐仓储
func NewPackageRepository(pool *pgxpool.Pool) *PackageRepository {
	return &PackageRepository{pool: pool}
}

// Create 创建套餐
func (r *PackageRepository) Create(ctx context.Context, pkg *domain.Package, requestID, traceID string) error {
	query := `
		INSERT INTO supply_packages (
			supply_account_id, user_id, platform, model,
			total_quota, available_quota, sold_quota, reserved_quota,
			price_per_1m_input, price_per_1m_output, min_purchase,
			start_at, end_at, valid_days,
			status, max_concurrent, rate_limit_rpm,
			total_orders, total_revenue, rating, rating_count,
			quota_unit, price_unit, currency_code, version,
			created_ip, updated_ip, audit_trace_id,
			request_id
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21,
			$22, $23, $24, $25, $26, $27, $28
		)
		RETURNING id, created_at, updated_at
	`

	var startAt, endAt *time.Time
	if !pkg.StartAt.IsZero() {
		startAt = &pkg.StartAt
	}
	if !pkg.EndAt.IsZero() {
		endAt = &pkg.EndAt
	}

	err := r.pool.QueryRow(ctx, query,
		pkg.SupplierID, pkg.AccountID, pkg.Platform, pkg.Model,
		pkg.TotalQuota, pkg.AvailableQuota, pkg.SoldQuota, pkg.ReservedQuota,
		pkg.PricePer1MInput, pkg.PricePer1MOutput, pkg.MinPurchase,
		startAt, endAt, pkg.ValidDays,
		pkg.Status, pkg.MaxConcurrent, pkg.RateLimitRPM,
		pkg.TotalOrders, pkg.TotalRevenue, pkg.Rating, pkg.RatingCount,
		"token", "per_1m_tokens", "USD", 0,
		nil, nil, traceID,
		requestID,
	).Scan(&pkg.ID, &pkg.CreatedAt, &pkg.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create package: %w", err)
	}
	return nil
}

// GetByID 获取套餐
func (r *PackageRepository) GetByID(ctx context.Context, supplierID, id int64) (*domain.Package, error) {
	query := `
		SELECT id, supply_account_id, user_id, platform, model,
			total_quota, available_quota, sold_quota, reserved_quota,
			price_per_1m_input, price_per_1m_output, min_purchase,
			start_at, end_at, valid_days,
			status, max_concurrent, rate_limit_rpm,
			total_orders, total_revenue, rating, rating_count,
			quota_unit, price_unit, currency_code, version,
			created_at, updated_at
		FROM supply_packages
		WHERE id = $1 AND user_id = $2
	`

	pkg := &domain.Package{}
	var startAt, endAt *time.Time
	err := r.pool.QueryRow(ctx, query, id, supplierID).Scan(
		&pkg.ID, &pkg.SupplierID, &pkg.AccountID, &pkg.Platform, &pkg.Model,
		&pkg.TotalQuota, &pkg.AvailableQuota, &pkg.SoldQuota, &pkg.ReservedQuota,
		&pkg.PricePer1MInput, &pkg.PricePer1MOutput, &pkg.MinPurchase,
		&startAt, &endAt, &pkg.ValidDays,
		&pkg.Status, &pkg.MaxConcurrent, &pkg.RateLimitRPM,
		&pkg.TotalOrders, &pkg.TotalRevenue, &pkg.Rating, &pkg.RatingCount,
		&pkg.QuotaUnit, &pkg.PriceUnit, &pkg.CurrencyCode, &pkg.Version,
		&pkg.CreatedAt, &pkg.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get package: %w", err)
	}

	if startAt != nil {
		pkg.StartAt = *startAt
	}
	if endAt != nil {
		pkg.EndAt = *endAt
	}

	return pkg, nil
}

// Update 更新套餐（乐观锁）
func (r *PackageRepository) Update(ctx context.Context, pkg *domain.Package, expectedVersion int) error {
	query := `
		UPDATE supply_packages SET
			platform = $1, model = $2,
			total_quota = $3, available_quota = $4, sold_quota = $5, reserved_quota = $6,
			price_per_1m_input = $7, price_per_1m_output = $8,
			start_at = $9, end_at = $10, valid_days = $11,
			status = $12, max_concurrent = $13, rate_limit_rpm = $14,
			total_orders = $15, total_revenue = $16,
			rating = $17, rating_count = $18,
			version = $19, updated_at = $20
		WHERE id = $21 AND user_id = $22 AND version = $23
	`

	pkg.UpdatedAt = time.Now()
	newVersion := expectedVersion + 1

	cmdTag, err := r.pool.Exec(ctx, query,
		pkg.Platform, pkg.Model,
		pkg.TotalQuota, pkg.AvailableQuota, pkg.SoldQuota, pkg.ReservedQuota,
		pkg.PricePer1MInput, pkg.PricePer1MOutput,
		pkg.StartAt, pkg.EndAt, pkg.ValidDays,
		pkg.Status, pkg.MaxConcurrent, pkg.RateLimitRPM,
		pkg.TotalOrders, pkg.TotalRevenue,
		pkg.Rating, pkg.RatingCount,
		newVersion, pkg.UpdatedAt,
		pkg.ID, pkg.SupplierID, expectedVersion,
	)

	if err != nil {
		return fmt.Errorf("failed to update package: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return ErrConcurrencyConflict
	}

	pkg.Version = newVersion
	return nil
}

// GetForUpdate 获取套餐并加行锁
func (r *PackageRepository) GetForUpdate(ctx context.Context, tx pgxpool.Tx, supplierID, id int64) (*domain.Package, error) {
	query := `
		SELECT id, supply_account_id, user_id, platform, model,
			total_quota, available_quota, sold_quota, reserved_quota,
			price_per_1m_input, price_per_1m_output,
			status, version,
			created_at, updated_at
		FROM supply_packages
		WHERE id = $1 AND user_id = $2
		FOR UPDATE
	`

	pkg := &domain.Package{}
	err := tx.QueryRow(ctx, query, id, supplierID).Scan(
		&pkg.ID, &pkg.SupplierID, &pkg.AccountID, &pkg.Platform, &pkg.Model,
		&pkg.TotalQuota, &pkg.AvailableQuota, &pkg.SoldQuota, &pkg.ReservedQuota,
		&pkg.PricePer1MInput, &pkg.PricePer1MOutput,
		&pkg.Status, &pkg.Version,
		&pkg.CreatedAt, &pkg.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get package for update: %w", err)
	}

	return pkg, nil
}

// List 列出套餐
func (r *PackageRepository) List(ctx context.Context, supplierID int64) ([]*domain.Package, error) {
	query := `
		SELECT id, supply_account_id, user_id, platform, model,
			total_quota, available_quota, sold_quota,
			price_per_1m_input, price_per_1m_output,
			status, max_concurrent, rate_limit_rpm,
			valid_days, total_orders, total_revenue,
			version, created_at, updated_at
		FROM supply_packages
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, supplierID)
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}
	defer rows.Close()

	packages := make([]*domain.Package, 0)
	for rows.Next() {
		pkg := &domain.Package{}
		err := rows.Scan(
			&pkg.ID, &pkg.SupplierID, &pkg.AccountID, &pkg.Platform, &pkg.Model,
			&pkg.TotalQuota, &pkg.AvailableQuota, &pkg.SoldQuota,
			&pkg.PricePer1MInput, &pkg.PricePer1MOutput,
			&pkg.Status, &pkg.MaxConcurrent, &pkg.RateLimitRPM,
			&pkg.ValidDays, &pkg.TotalOrders, &pkg.TotalRevenue,
			&pkg.Version, &pkg.CreatedAt, &pkg.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan package: %w", err)
		}
		packages = append(packages, pkg)
	}

	return packages, nil
}

// UpdateQuota 扣减配额
func (r *PackageRepository) UpdateQuota(ctx context.Context, tx pgxpool.Tx, packageID, supplierID int64, usedQuota float64) error {
	query := `
		UPDATE supply_packages SET
			available_quota = available_quota - $1,
			sold_quota = sold_quota + $1,
			updated_at = $2
		WHERE id = $3 AND user_id = $4 AND available_quota >= $1
		RETURNING id
	`

	var id int64
	err := tx.QueryRow(ctx, query, usedQuota, time.Now(), packageID, supplierID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("insufficient quota or package not found")
	}
	if err != nil {
		return fmt.Errorf("failed to update quota: %w", err)
	}
	return nil
}
