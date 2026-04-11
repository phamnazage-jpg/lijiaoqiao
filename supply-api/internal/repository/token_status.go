package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TokenStatus Token状态
type TokenStatus string

const (
	TokenStatusActive  TokenStatus = "active"
	TokenStatusRevoked TokenStatus = "revoked"
	TokenStatusExpired TokenStatus = "expired"
)

// TokenStatusRecord Token状态记录
type TokenStatusRecord struct {
	ID                  int64      `json:"id"`
	TokenID             string     `json:"token_id"`
	SubjectID           int64      `json:"subject_id"`
	TenantID            int64      `json:"tenant_id"`
	Role                string     `json:"role"`
	Status              TokenStatus `json:"status"`
	IssuedAt            time.Time  `json:"issued_at"`
	ExpiresAt           time.Time  `json:"expires_at"`
	RevokedAt           *time.Time `json:"revoked_at,omitempty"`
	RevokedReason       *string    `json:"revoked_reason,omitempty"`
	RevokedBy           *int64     `json:"revoked_by,omitempty"`
	LastVerifiedAt      *time.Time `json:"last_verified_at,omitempty"`
	VerificationCount   int64      `json:"verification_count"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// TokenStatusRepository Token状态仓储
type TokenStatusRepository struct {
	pool *pgxpool.Pool
}

// NewTokenStatusRepository 创建Token状态仓储
func NewTokenStatusRepository(pool *pgxpool.Pool) *TokenStatusRepository {
	return &TokenStatusRepository{pool: pool}
}

// Create 创建Token状态记录
func (r *TokenStatusRepository) Create(ctx context.Context, record *TokenStatusRecord) error {
	query := `
		INSERT INTO token_status_registry (
			token_id, subject_id, tenant_id, role, status,
			issued_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		record.TokenID, record.SubjectID, record.TenantID, record.Role,
		record.Status, record.IssuedAt, record.ExpiresAt,
	).Scan(&record.ID, &record.CreatedAt, &record.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create token status record: %w", err)
	}
	return nil
}

// GetByTokenID 根据TokenID获取状态
func (r *TokenStatusRepository) GetByTokenID(ctx context.Context, tokenID string) (*TokenStatusRecord, error) {
	query := `
		SELECT id, token_id, subject_id, tenant_id, role, status,
			issued_at, expires_at, revoked_at, revoked_reason, revoked_by,
			last_verified_at, verification_count, created_at, updated_at
		FROM token_status_registry
		WHERE token_id = $1
	`

	record := &TokenStatusRecord{}
	err := r.pool.QueryRow(ctx, query, tokenID).Scan(
		&record.ID, &record.TokenID, &record.SubjectID, &record.TenantID, &record.Role,
		&record.Status, &record.IssuedAt, &record.ExpiresAt, &record.RevokedAt,
		&record.RevokedReason, &record.RevokedBy, &record.LastVerifiedAt,
		&record.VerificationCount, &record.CreatedAt, &record.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get token status record: %w", err)
	}

	return record, nil
}

// GetStatus 获取Token状态字符串（用于TokenStatusBackend接口）
func (r *TokenStatusRepository) GetStatus(ctx context.Context, tokenID string) (string, error) {
	record, err := r.GetByTokenID(ctx, tokenID)
	if err != nil {
		return "", err
	}
	if record == nil {
		return "active", nil // 不存在的token默认为active（未发行）
	}

	// 检查是否过期
	if record.Status == TokenStatusActive && record.ExpiresAt.Before(time.Now()) {
		return string(TokenStatusExpired), nil
	}

	return string(record.Status), nil
}

// Revoke 吊销Token（用于TokenRevocationBackend接口）
func (r *TokenStatusRepository) Revoke(ctx context.Context, tokenID string, reason string) error {
	query := `
		UPDATE token_status_registry SET
			status = 'revoked',
			revoked_at = CURRENT_TIMESTAMP,
			revoked_reason = $2,
			updated_at = CURRENT_TIMESTAMP
		WHERE token_id = $1 AND status = 'active'
	`

	result, err := r.pool.Exec(ctx, query, tokenID, reason)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("token not found or already revoked")
	}

	return nil
}

// RevokeBySubjectID 根据SubjectID吊销所有Token
func (r *TokenStatusRepository) RevokeBySubjectID(ctx context.Context, subjectID int64, reason string) (int64, error) {
	query := `
		UPDATE token_status_registry SET
			status = 'revoked',
			revoked_at = CURRENT_TIMESTAMP,
			revoked_reason = $2,
			updated_at = CURRENT_TIMESTAMP
		WHERE subject_id = $1 AND status = 'active'
	`

	result, err := r.pool.Exec(ctx, query, subjectID, reason)
	if err != nil {
		return 0, fmt.Errorf("failed to revoke tokens by subject_id: %w", err)
	}

	return result.RowsAffected(), nil
}

// UpdateVerificationCount 更新验证计数
func (r *TokenStatusRepository) UpdateVerificationCount(ctx context.Context, tokenID string) error {
	query := `
		UPDATE token_status_registry SET
			last_verified_at = CURRENT_TIMESTAMP,
			verification_count = verification_count + 1,
			updated_at = CURRENT_TIMESTAMP
		WHERE token_id = $1
	`

	_, err := r.pool.Exec(ctx, query, tokenID)
	if err != nil {
		return fmt.Errorf("failed to update verification count: %w", err)
	}

	return nil
}

// DeleteExpired 删除过期记录（定时清理）
func (r *TokenStatusRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	query := `DELETE FROM token_status_registry WHERE status = 'expired' AND expires_at < $1`

	cmdTag, err := r.pool.Exec(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired token records: %w", err)
	}

	return cmdTag.RowsAffected(), nil
}

// DeleteBySubjectID 删除用户的所有Token记录（登出时清理）
func (r *TokenStatusRepository) DeleteBySubjectID(ctx context.Context, subjectID int64) (int64, error) {
	query := `DELETE FROM token_status_registry WHERE subject_id = $1`

	cmdTag, err := r.pool.Exec(ctx, query, subjectID)
	if err != nil {
		return 0, fmt.Errorf("failed to delete token records by subject_id: %w", err)
	}

	return cmdTag.RowsAffected(), nil
}

// ListActiveBySubjectID 列出用户的所有活跃Token
func (r *TokenStatusRepository) ListActiveBySubjectID(ctx context.Context, subjectID int64) ([]*TokenStatusRecord, error) {
	query := `
		SELECT id, token_id, subject_id, tenant_id, role, status,
			issued_at, expires_at, revoked_at, revoked_reason, revoked_by,
			last_verified_at, verification_count, created_at, updated_at
		FROM token_status_registry
		WHERE subject_id = $1 AND status = 'active' AND expires_at > CURRENT_TIMESTAMP
		ORDER BY issued_at DESC
	`

	rows, err := r.pool.Query(ctx, query, subjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list active tokens: %w", err)
	}
	defer rows.Close()

	var records []*TokenStatusRecord
	for rows.Next() {
		record := &TokenStatusRecord{}
		err := rows.Scan(
			&record.ID, &record.TokenID, &record.SubjectID, &record.TenantID, &record.Role,
			&record.Status, &record.IssuedAt, &record.ExpiresAt, &record.RevokedAt,
			&record.RevokedReason, &record.RevokedBy, &record.LastVerifiedAt,
			&record.VerificationCount, &record.CreatedAt, &record.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan token record: %w", err)
		}
		records = append(records, record)
	}

	return records, nil
}
