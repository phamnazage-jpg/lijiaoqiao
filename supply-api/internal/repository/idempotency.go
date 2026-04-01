package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IdempotencyStatus 幂等记录状态
type IdempotencyStatus string

const (
	IdempotencyStatusProcessing IdempotencyStatus = "processing"
	IdempotencyStatusSucceeded IdempotencyStatus = "succeeded"
	IdempotencyStatusFailed    IdempotencyStatus = "failed"
)

// IdempotencyRecord 幂等记录
type IdempotencyRecord struct {
	ID             int64             `json:"id"`
	TenantID       int64             `json:"tenant_id"`
	OperatorID     int64             `json:"operator_id"`
	APIPath        string            `json:"api_path"`
	IdempotencyKey string            `json:"idempotency_key"`
	RequestID      string            `json:"request_id"`
	PayloadHash    string            `json:"payload_hash"` // SHA256 of request body
	ResponseCode   int               `json:"response_code"`
	ResponseBody   json.RawMessage   `json:"response_body"`
	Status         IdempotencyStatus `json:"status"`
	ExpiresAt      time.Time         `json:"expires_at"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// IdempotencyRepository 幂等记录仓储
type IdempotencyRepository struct {
	pool *pgxpool.Pool
}

// NewIdempotencyRepository 创建幂等记录仓储
func NewIdempotencyRepository(pool *pgxpool.Pool) *IdempotencyRepository {
	return &IdempotencyRepository{pool: pool}
}

// GetByKey 根据幂等键获取记录
func (r *IdempotencyRepository) GetByKey(ctx context.Context, tenantID, operatorID int64, apiPath, idempotencyKey string) (*IdempotencyRecord, error) {
	query := `
		SELECT id, tenant_id, operator_id, api_path, idempotency_key,
			request_id, payload_hash, response_code, response_body,
			status, expires_at, created_at, updated_at
		FROM supply_idempotency_records
		WHERE tenant_id = $1 AND operator_id = $2 AND api_path = $3 AND idempotency_key = $4
		AND expires_at > $5
		FOR UPDATE
	`

	record := &IdempotencyRecord{}
	err := r.pool.QueryRow(ctx, query, tenantID, operatorID, apiPath, idempotencyKey, time.Now()).Scan(
		&record.ID, &record.TenantID, &record.OperatorID, &record.APIPath, &record.IdempotencyKey,
		&record.RequestID, &record.PayloadHash, &record.ResponseCode, &record.ResponseBody,
		&record.Status, &record.ExpiresAt, &record.CreatedAt, &record.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // 不存在或已过期
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get idempotency record: %w", err)
	}

	return record, nil
}

// Create 创建幂等记录
func (r *IdempotencyRepository) Create(ctx context.Context, record *IdempotencyRecord) error {
	query := `
		INSERT INTO supply_idempotency_records (
			tenant_id, operator_id, api_path, idempotency_key,
			request_id, payload_hash, status, expires_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
		RETURNING id, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		record.TenantID, record.OperatorID, record.APIPath, record.IdempotencyKey,
		record.RequestID, record.PayloadHash, record.Status, record.ExpiresAt,
	).Scan(&record.ID, &record.CreatedAt, &record.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create idempotency record: %w", err)
	}
	return nil
}

// UpdateSuccess 更新为成功状态
func (r *IdempotencyRepository) UpdateSuccess(ctx context.Context, id int64, responseCode int, responseBody json.RawMessage) error {
	query := `
		UPDATE supply_idempotency_records SET
			response_code = $1,
			response_body = $2,
			status = $3,
			updated_at = $4
		WHERE id = $5
	`

	_, err := r.pool.Exec(ctx, query, responseCode, responseBody, IdempotencyStatusSucceeded, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update idempotency record to success: %w", err)
	}
	return nil
}

// UpdateFailed 更新为失败状态
func (r *IdempotencyRepository) UpdateFailed(ctx context.Context, id int64, responseCode int, responseBody json.RawMessage) error {
	query := `
		UPDATE supply_idempotency_records SET
			response_code = $1,
			response_body = $2,
			status = $3,
			updated_at = $4
		WHERE id = $5
	`

	_, err := r.pool.Exec(ctx, query, responseCode, responseBody, IdempotencyStatusFailed, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update idempotency record to failed: %w", err)
	}
	return nil
}

// DeleteExpired 删除过期记录（定时清理）
func (r *IdempotencyRepository) DeleteExpired(ctx context.Context) (int64, error) {
	query := `DELETE FROM supply_idempotency_records WHERE expires_at < $1`

	cmdTag, err := r.pool.Exec(ctx, query, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired idempotency records: %w", err)
	}

	return cmdTag.RowsAffected(), nil
}

// GetByRequestID 根据请求ID获取记录
func (r *IdempotencyRepository) GetByRequestID(ctx context.Context, requestID string) (*IdempotencyRecord, error) {
	query := `
		SELECT id, tenant_id, operator_id, api_path, idempotency_key,
			request_id, payload_hash, response_code, response_body,
			status, expires_at, created_at, updated_at
		FROM supply_idempotency_records
		WHERE request_id = $1
	`

	record := &IdempotencyRecord{}
	err := r.pool.QueryRow(ctx, query, requestID).Scan(
		&record.ID, &record.TenantID, &record.OperatorID, &record.APIPath, &record.IdempotencyKey,
		&record.RequestID, &record.PayloadHash, &record.ResponseCode, &record.ResponseBody,
		&record.Status, &record.ExpiresAt, &record.CreatedAt, &record.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get idempotency record by request_id: %w", err)
	}

	return record, nil
}

// CheckExists 检查幂等记录是否存在（用于竞争条件检测）
func (r *IdempotencyRepository) CheckExists(ctx context.Context, tenantID, operatorID int64, apiPath, idempotencyKey string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM supply_idempotency_records
			WHERE tenant_id = $1 AND operator_id = $2 AND api_path = $3 AND idempotency_key = $4
			AND expires_at > $5
		)
	`

	var exists bool
	err := r.pool.QueryRow(ctx, query, tenantID, operatorID, apiPath, idempotencyKey, time.Now()).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check idempotency record existence: %w", err)
	}

	return exists, nil
}

// AcquireLock 尝试获取幂等锁（用于创建记录）
func (r *IdempotencyRepository) AcquireLock(ctx context.Context, tenantID, operatorID int64, apiPath, idempotencyKey string, ttl time.Duration) (*IdempotencyRecord, error) {
	// 先尝试插入
	record := &IdempotencyRecord{
		TenantID:       tenantID,
		OperatorID:     operatorID,
		APIPath:        apiPath,
		IdempotencyKey: idempotencyKey,
		RequestID:      "", // 稍后填充
		PayloadHash:    "", // 稍后填充
		Status:         IdempotencyStatusProcessing,
		ExpiresAt:      time.Now().Add(ttl),
	}

	query := `
		INSERT INTO supply_idempotency_records (
			tenant_id, operator_id, api_path, idempotency_key,
			request_id, payload_hash, status, expires_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
		ON CONFLICT (tenant_id, operator_id, api_path, idempotency_key)
		DO UPDATE SET
			request_id = EXCLUDED.request_id,
			payload_hash = EXCLUDED.payload_hash,
			status = EXCLUDED.status,
			expires_at = EXCLUDED.expires_at,
			updated_at = now()
		WHERE supply_idempotency_records.expires_at <= $8
		RETURNING id, created_at, updated_at, status
	`

	err := r.pool.QueryRow(ctx, query,
		record.TenantID, record.OperatorID, record.APIPath, record.IdempotencyKey,
		record.RequestID, record.PayloadHash, record.Status, record.ExpiresAt,
	).Scan(&record.ID, &record.CreatedAt, &record.UpdatedAt, &record.Status)

	if err != nil {
		// 可能是重复插入
		existing, getErr := r.GetByKey(ctx, tenantID, operatorID, apiPath, idempotencyKey)
		if getErr != nil {
			return nil, fmt.Errorf("failed to acquire idempotency lock: %w (get err: %v)", err, getErr)
		}
		if existing != nil {
			return existing, nil // 返回已存在的记录
		}
		return nil, fmt.Errorf("failed to acquire idempotency lock: %w", err)
	}

	return record, nil
}
