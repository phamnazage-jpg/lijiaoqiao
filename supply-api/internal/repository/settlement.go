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

// SettlementRepository 结算仓储
type SettlementRepository struct {
	pool *pgxpool.Pool
}

// NewSettlementRepository 创建结算仓储
func NewSettlementRepository(pool *pgxpool.Pool) *SettlementRepository {
	return &SettlementRepository{pool: pool}
}

// Create 创建结算单
func (r *SettlementRepository) Create(ctx context.Context, s *domain.Settlement, requestID, idempotencyKey, traceID string) error {
	query := `
		INSERT INTO supply_settlements (
			settlement_no, user_id, total_amount, fee_amount, net_amount,
			status, payment_method, payment_account,
			period_start, period_end, total_orders, total_usage_records,
			currency_code, amount_unit, version,
			request_id, idempotency_key, audit_trace_id
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
		)
		RETURNING id, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		s.SettlementNo, s.SupplierID, s.TotalAmount, s.FeeAmount, s.NetAmount,
		s.Status, s.PaymentMethod, s.PaymentAccount,
		s.PeriodStart, s.PeriodEnd, s.TotalOrders, s.TotalUsageRecords,
		"USD", "minor", 0,
		requestID, idempotencyKey, traceID,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create settlement: %w", err)
	}
	return nil
}

// GetByID 获取结算单
func (r *SettlementRepository) GetByID(ctx context.Context, supplierID, id int64) (*domain.Settlement, error) {
	query := `
		SELECT id, settlement_no, user_id, total_amount, fee_amount, net_amount,
			status, payment_method, payment_account,
			period_start, period_end, total_orders, total_usage_records,
			payment_transaction_id, paid_at,
			version, created_at, updated_at
		FROM supply_settlements
		WHERE id = $1 AND user_id = $2
	`

	s := &domain.Settlement{}
	var paidAt pgx.NullTime
	err := r.pool.QueryRow(ctx, query, id, supplierID).Scan(
		&s.ID, &s.SettlementNo, &s.SupplierID, &s.TotalAmount, &s.FeeAmount, &s.NetAmount,
		&s.Status, &s.PaymentMethod, &s.PaymentAccount,
		&s.PeriodStart, &s.PeriodEnd, &s.TotalOrders, &s.TotalUsageRecords,
		&s.PaymentTransactionID, &paidAt,
		&s.Version, &s.CreatedAt, &s.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get settlement: %w", err)
	}

	if paidAt.Valid {
		s.PaidAt = &paidAt.Time
	}

	return s, nil
}

// Update 更新结算单（乐观锁）
func (r *SettlementRepository) Update(ctx context.Context, s *domain.Settlement, expectedVersion int) error {
	query := `
		UPDATE supply_settlements SET
			status = $1, payment_method = $2, payment_account = $3,
			payment_transaction_id = $4, paid_at = $5,
			total_orders = $6, total_usage_records = $7,
			version = $8, updated_at = $9
		WHERE id = $10 AND user_id = $11 AND version = $12
	`

	s.UpdatedAt = time.Now()
	newVersion := expectedVersion + 1

	cmdTag, err := r.pool.Exec(ctx, query,
		s.Status, s.PaymentMethod, s.PaymentAccount,
		s.PaymentTransactionID, s.PaidAt,
		s.TotalOrders, s.TotalUsageRecords,
		newVersion, s.UpdatedAt,
		s.ID, s.SupplierID, expectedVersion,
	)

	if err != nil {
		return fmt.Errorf("failed to update settlement: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return ErrConcurrencyConflict
	}

	s.Version = newVersion
	return nil
}

// GetForUpdate 获取结算单并加行锁
func (r *SettlementRepository) GetForUpdate(ctx context.Context, tx pgxpool.Tx, supplierID, id int64) (*domain.Settlement, error) {
	query := `
		SELECT id, settlement_no, user_id, total_amount, fee_amount, net_amount,
			status, payment_method, payment_account, version,
			created_at, updated_at
		FROM supply_settlements
		WHERE id = $1 AND user_id = $2
		FOR UPDATE
	`

	s := &domain.Settlement{}
	err := tx.QueryRow(ctx, query, id, supplierID).Scan(
		&s.ID, &s.SettlementNo, &s.SupplierID, &s.TotalAmount, &s.FeeAmount, &s.NetAmount,
		&s.Status, &s.PaymentMethod, &s.PaymentAccount, &s.Version,
		&s.CreatedAt, &s.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get settlement for update: %w", err)
	}

	return s, nil
}

// GetProcessing 获取处理中的结算单（用于单一性约束）
func (r *SettlementRepository) GetProcessing(ctx context.Context, tx pgxpool.Tx, supplierID int64) (*domain.Settlement, error) {
	query := `
		SELECT id, settlement_no, user_id, total_amount, fee_amount, net_amount,
			status, payment_method, payment_account, version,
			created_at, updated_at
		FROM supply_settlements
		WHERE user_id = $1 AND status = 'processing'
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`

	s := &domain.Settlement{}
	err := tx.QueryRow(ctx, query, supplierID).Scan(
		&s.ID, &s.SettlementNo, &s.SupplierID, &s.TotalAmount, &s.FeeAmount, &s.NetAmount,
		&s.Status, &s.PaymentMethod, &s.PaymentAccount, &s.Version,
		&s.CreatedAt, &s.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // 没有处理中的单据
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get processing settlement: %w", err)
	}

	return s, nil
}

// List 列出结算单
func (r *SettlementRepository) List(ctx context.Context, supplierID int64) ([]*domain.Settlement, error) {
	query := `
		SELECT id, settlement_no, user_id, total_amount, fee_amount, net_amount,
			status, payment_method,
			period_start, period_end, total_orders,
			version, created_at, updated_at
		FROM supply_settlements
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, supplierID)
	if err != nil {
		return nil, fmt.Errorf("failed to list settlements: %w", err)
	}
	defer rows.Close()

	var settlements []*domain.Settlement
	for rows.Next() {
		s := &domain.Settlement{}
		err := rows.Scan(
			&s.ID, &s.SettlementNo, &s.SupplierID, &s.TotalAmount, &s.FeeAmount, &s.NetAmount,
			&s.Status, &s.PaymentMethod,
			&s.PeriodStart, &s.PeriodEnd, &s.TotalOrders,
			&s.Version, &s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan settlement: %w", err)
		}
		settlements = append(settlements, s)
	}

	return settlements, nil
}

// CreateInTx 在事务中创建结算单
func (r *SettlementRepository) CreateInTx(ctx context.Context, tx pgxpool.Tx, s *domain.Settlement, requestID, idempotencyKey, traceID string) error {
	query := `
		INSERT INTO supply_settlements (
			settlement_no, user_id, total_amount, fee_amount, net_amount,
			status, payment_method, payment_account,
			period_start, period_end, total_orders, total_usage_records,
			currency_code, amount_unit, version,
			request_id, idempotency_key, audit_trace_id
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
		)
		RETURNING id, created_at, updated_at
	`

	err := tx.QueryRow(ctx, query,
		s.SettlementNo, s.SupplierID, s.TotalAmount, s.FeeAmount, s.NetAmount,
		s.Status, s.PaymentMethod, s.PaymentAccount,
		s.PeriodStart, s.PeriodEnd, s.TotalOrders, s.TotalUsageRecords,
		"USD", "minor", 0,
		requestID, idempotencyKey, traceID,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create settlement in tx: %w", err)
	}
	return nil
}
