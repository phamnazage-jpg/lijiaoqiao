package repository

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"lijiaoqiao/supply-api/internal/domain"
)

// AccountRepository 账号仓储
type AccountRepository struct {
	pool *pgxpool.Pool
}

// NewAccountRepository 创建账号仓储
func NewAccountRepository(pool *pgxpool.Pool) *AccountRepository {
	return &AccountRepository{pool: pool}
}

// Create 创建账号
func (r *AccountRepository) Create(ctx context.Context, account *domain.Account, requestID, idempotencyKey, traceID string) error {
	query := `
		INSERT INTO supply_accounts (
			user_id, platform, account_type, account_name,
			encrypted_credentials, key_id,
			status, risk_level, total_quota, available_quota, frozen_quota,
			is_verified, verified_at, last_check_at,
			tos_compliant, tos_check_result,
			total_requests, total_tokens, total_cost, success_rate,
			risk_score, risk_reason, is_frozen, frozen_reason,
			credential_cipher_algo, credential_kms_key_alias, credential_key_version,
			quota_unit, currency_code, version,
			created_ip, updated_ip, audit_trace_id,
			request_id, idempotency_key
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12, $13, $14,
			$15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25, $26, $27,
			$28, $29, $30, $31, $32, $33, $34, $35
		)
		RETURNING id, created_at, updated_at
	`

	var createdIP, updatedIP *netip.Addr
	if account.CreatedIP != nil {
		createdIP = account.CreatedIP
	}
	if account.UpdatedIP != nil {
		updatedIP = account.UpdatedIP
	}

	err := r.pool.QueryRow(ctx, query,
		account.SupplierID, account.Provider, account.AccountType, account.Alias,
		account.CredentialHash, account.KeyID,
		account.Status, account.RiskLevel, account.TotalQuota, account.AvailableQuota, account.FrozenQuota,
		account.IsVerified, account.VerifiedAt, account.LastCheckAt,
		account.TosCompliant, account.TosCheckResult,
		account.TotalRequests, account.TotalTokens, account.TotalCost, account.SuccessRate,
		account.RiskScore, account.RiskReason, account.IsFrozen, account.FrozenReason,
		"AES-256-GCM", "kms/supply/default", 1,
		"token", "USD", 0,
		createdIP, updatedIP, traceID,
		requestID, idempotencyKey,
	).Scan(&account.ID, &account.CreatedAt, &account.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}
	return nil
}

// GetByID 获取账号
func (r *AccountRepository) GetByID(ctx context.Context, supplierID, id int64) (*domain.Account, error) {
	query := `
		SELECT id, user_id, platform, account_type, account_name,
			encrypted_credentials, key_id,
			status, risk_level, total_quota, available_quota, frozen_quota,
			is_verified, verified_at, last_check_at,
			tos_compliant, tos_check_result,
			total_requests, total_tokens, total_cost, success_rate,
			risk_score, risk_reason, is_frozen, frozen_reason,
			credential_cipher_algo, credential_kms_key_alias, credential_key_version,
			quota_unit, currency_code, version,
			created_ip, updated_ip, audit_trace_id,
			created_at, updated_at
		FROM supply_accounts
		WHERE id = $1 AND user_id = $2
	`

	account := &domain.Account{}
	var createdIP, updatedIP netip.Addr
	var credentialFingerprint *string

	err := r.pool.QueryRow(ctx, query, id, supplierID).Scan(
		&account.ID, &account.SupplierID, &account.Provider, &account.AccountType, &account.Alias,
		&account.CredentialHash, &account.KeyID,
		&account.Status, &account.RiskLevel, &account.TotalQuota, &account.AvailableQuota, &account.FrozenQuota,
		&account.IsVerified, &account.VerifiedAt, &account.LastCheckAt,
		&account.TosCompliant, &account.TosCheckResult,
		&account.TotalRequests, &account.TotalTokens, &account.TotalCost, &account.SuccessRate,
		&account.RiskScore, &account.RiskReason, &account.IsFrozen, &account.FrozenReason,
		&account.CredentialCipherAlgo, &account.CredentialKMSKeyAlias, &account.CredentialKeyVersion,
		&account.QuotaUnit, &account.CurrencyCode, &account.Version,
		&createdIP, &updatedIP, &account.AuditTraceID,
		&account.CreatedAt, &account.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	account.CreatedIP = &createdIP
	account.UpdatedIP = &updatedIP
	_ = credentialFingerprint // 未使用但字段存在

	return account, nil
}

// Update 更新账号（乐观锁）
func (r *AccountRepository) Update(ctx context.Context, account *domain.Account, expectedVersion int) error {
	query := `
		UPDATE supply_accounts SET
			platform = $1, account_type = $2, account_name = $3,
			status = $4, risk_level = $5, total_quota = $6, available_quota = $7,
			frozen_quota = $8, is_verified = $9, verified_at = $10, last_check_at = $11,
			tos_compliant = $12, tos_check_result = $13,
			total_requests = $14, total_tokens = $15, total_cost = $16, success_rate = $17,
			risk_score = $18, risk_reason = $19, is_frozen = $20, frozen_reason = $21,
			version = $22, updated_at = $23
		WHERE id = $24 AND user_id = $25 AND version = $26
	`

	account.UpdatedAt = time.Now()
	newVersion := expectedVersion + 1

	cmdTag, err := r.pool.Exec(ctx, query,
		account.Provider, account.AccountType, account.Alias,
		account.Status, account.RiskLevel, account.TotalQuota, account.AvailableQuota,
		account.FrozenQuota, account.IsVerified, account.VerifiedAt, account.LastCheckAt,
		account.TosCompliant, account.TosCheckResult,
		account.TotalRequests, account.TotalTokens, account.TotalCost, account.SuccessRate,
		account.RiskScore, account.RiskReason, account.IsFrozen, account.FrozenReason,
		newVersion, account.UpdatedAt,
		account.ID, account.SupplierID, expectedVersion,
	)

	if err != nil {
		return fmt.Errorf("failed to update account: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return ErrConcurrencyConflict
	}

	account.Version = newVersion
	return nil
}

// UpdateWithPessimisticLock 更新账号（悲观锁，用于提现等关键操作）
func (r *AccountRepository) UpdateWithPessimisticLock(ctx context.Context, tx pgxpool.Tx, account *domain.Account, expectedVersion int) error {
	query := `
		UPDATE supply_accounts SET
			available_quota = $1, frozen_quota = $2,
			version = $3, updated_at = $4
		WHERE id = $5 AND version = $6
		RETURNING version
	`

	account.UpdatedAt = time.Now()
	newVersion := expectedVersion + 1

	err := tx.QueryRow(ctx, query,
		account.AvailableQuota, account.FrozenQuota,
		newVersion, account.UpdatedAt,
		account.ID, expectedVersion,
	).Scan(&account.Version)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrConcurrencyConflict
	}
	if err != nil {
		return fmt.Errorf("failed to update account with lock: %w", err)
	}

	return nil
}

// GetForUpdate 获取账号并加行锁（用于事务内）
func (r *AccountRepository) GetForUpdate(ctx context.Context, tx pgxpool.Tx, supplierID, id int64) (*domain.Account, error) {
	query := `
		SELECT id, user_id, platform, account_type, account_name,
			encrypted_credentials, key_id,
			status, risk_level, total_quota, available_quota, frozen_quota,
			is_verified, verified_at, last_check_at,
			tos_compliant, tos_check_result,
			total_requests, total_tokens, total_cost, success_rate,
			risk_score, risk_reason, is_frozen, frozen_reason,
			version,
			created_at, updated_at
		FROM supply_accounts
		WHERE id = $1 AND user_id = $2
		FOR UPDATE
	`

	account := &domain.Account{}
	err := tx.QueryRow(ctx, query, id, supplierID).Scan(
		&account.ID, &account.SupplierID, &account.Provider, &account.AccountType, &account.Alias,
		&account.CredentialHash, &account.KeyID,
		&account.Status, &account.RiskLevel, &account.TotalQuota, &account.AvailableQuota, &account.FrozenQuota,
		&account.IsVerified, &account.VerifiedAt, &account.LastCheckAt,
		&account.TosCompliant, &account.TosCheckResult,
		&account.TotalRequests, &account.TotalTokens, &account.TotalCost, &account.SuccessRate,
		&account.RiskScore, &account.RiskReason, &account.IsFrozen, &account.FrozenReason,
		&account.Version,
		&account.CreatedAt, &account.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get account for update: %w", err)
	}

	return account, nil
}

// List 列出账号
func (r *AccountRepository) List(ctx context.Context, supplierID int64) ([]*domain.Account, error) {
	query := `
		SELECT id, user_id, platform, account_type, account_name,
			status, risk_level, total_quota, available_quota, frozen_quota,
			is_verified, verified_at, last_check_at,
			tos_compliant, success_rate,
			risk_score, is_frozen,
			version, created_at, updated_at
		FROM supply_accounts
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, supplierID)
	if err != nil {
		return nil, fmt.Errorf("failed to list accounts: %w", err)
	}
	defer rows.Close()

	accounts := make([]*domain.Account, 0)
	for rows.Next() {
		account := &domain.Account{}
		err := rows.Scan(
			&account.ID, &account.SupplierID, &account.Provider, &account.AccountType, &account.Alias,
			&account.Status, &account.RiskLevel, &account.TotalQuota, &account.AvailableQuota, &account.FrozenQuota,
			&account.IsVerified, &account.VerifiedAt, &account.LastCheckAt,
			&account.TosCompliant, &account.SuccessRate,
			&account.RiskScore, &account.IsFrozen,
			&account.Version, &account.CreatedAt, &account.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan account: %w", err)
		}
		accounts = append(accounts, account)
	}

	return accounts, nil
}

// GetWithdrawableBalance 获取可提现余额
func (r *AccountRepository) GetWithdrawableBalance(ctx context.Context, supplierID int64) (float64, error) {
	query := `
		SELECT COALESCE(SUM(available_quota), 0)
		FROM supply_accounts
		WHERE user_id = $1 AND status = 'active'
	`

	var balance float64
	err := r.pool.QueryRow(ctx, query, supplierID).Scan(&balance)
	if err != nil {
		return 0, fmt.Errorf("failed to get withdrawable balance: %w", err)
	}
	return balance, nil
}
