package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ==================== P0-07 批量补偿策略 ====================

// BatchCompensation 批量补偿记录
type BatchCompensation struct {
	ID              int64          `json:"id"`
	BatchID         string         `json:"batch_id"`
	OperationType   string         `json:"operation_type"`
	ItemIndex       int            `json:"item_index"`
	ItemPayload     json.RawMessage `json:"item_payload"`
	FailureReason   string         `json:"failure_reason,omitempty"`
	Status          string        `json:"status"` // pending, retrying, resolved, manual_required, abandoned
	RetryCount      int            `json:"retry_count"`
	MaxRetries      int            `json:"max_retries"`
	ResolvedAt      *time.Time     `json:"resolved_at,omitempty"`
	ResolvedBy      *int64         `json:"resolved_by,omitempty"`
	ResolutionNotes string         `json:"resolution_notes,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	CreatedBy       *int64         `json:"created_by,omitempty"`
	Version         int64          `json:"version"`
}

// CompensationStatus 补偿状态
const (
	CompensationStatusPending         = "pending"
	CompensationStatusRetrying        = "retrying"
	CompensationStatusResolved        = "resolved"
	CompensationStatusManualRequired = "manual_required"
	CompensationStatusAbandoned      = "abandoned"
)

// CompensationStore 补偿存储接口
type CompensationStore interface {
	// Create 创建补偿记录
	Create(ctx context.Context, comp *BatchCompensation) (int64, error)
	// GetByBatchID 获取批次补偿列表
	GetByBatchID(ctx context.Context, batchID string) ([]*BatchCompensation, error)
	// UpdateStatus 更新状态
	UpdateStatus(ctx context.Context, id int64, status string) error
	// Resolve 解决补偿
	Resolve(ctx context.Context, id int64, resolvedBy int64, notes string) error
	// MarkManualRequired 标记需要人工介入
	MarkManualRequired(ctx context.Context, id int64, reason string) error
}

// CompensationProcessor 补偿处理器
type CompensationProcessor struct {
	store         CompensationStore
	operationExecutor OperationExecutor
	stats          CompensationStats
}

// OperationExecutor 操作执行器接口
type OperationExecutor interface {
	// Execute 执行单个操作
	Execute(ctx context.Context, operationType string, payload json.RawMessage) error
}

// CompensationStats 补偿统计接口
type CompensationStats interface {
	RecordCompensationRetry(operationType string)
	RecordCompensationResolved(operationType string)
	RecordCompensationManual(operationType string)
}

// DefaultCompensationConfig 默认补偿配置
func DefaultCompensationConfig() *CompensationConfig {
	return &CompensationConfig{
		MaxRetries:      3,
		RetryInterval:    1 * time.Minute,
	}
}

// NoOpCompensationStats No-op补偿统计实现
type NoOpCompensationStats struct{}

func (s *NoOpCompensationStats) RecordCompensationRetry(operationType string)         {}
func (s *NoOpCompensationStats) RecordCompensationResolved(operationType string)      {}
func (s *NoOpCompensationStats) RecordCompensationManual(operationType string)        {}

// NewCompensationProcessor 创建补偿处理器
func NewCompensationProcessor(store CompensationStore, executor OperationExecutor, stats CompensationStats) *CompensationProcessor {
	return &CompensationProcessor{
		store:           store,
		operationExecutor: executor,
		stats:           stats,
	}
}

// CompensationConfig 补偿配置
type CompensationConfig struct {
	MaxRetries   int
	RetryInterval time.Duration
}

// ProcessBatchCompensations 处理批次补偿
func (p *CompensationProcessor) ProcessBatchCompensations(ctx context.Context, batchID string) (*CompensationResult, error) {
	// 获取批次补偿列表
	compensations, err := p.store.GetByBatchID(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get compensations: %w", err)
	}

	result := &CompensationResult{
		BatchID:         batchID,
		TotalItems:      len(compensations),
		SuccessCount:    0,
		RetryCount:     0,
		ManualCount:     0,
		FailedCount:     0,
	}

	for _, comp := range compensations {
		if comp.Status != CompensationStatusPending {
			continue
		}

		// 重试执行
		err := p.operationExecutor.Execute(ctx, comp.OperationType, comp.ItemPayload)
		if err != nil {
			comp.RetryCount++
			comp.FailureReason = err.Error()

			if comp.RetryCount >= comp.MaxRetries {
				// 超过最大重试次数，标记需要人工介入
				if err := p.store.MarkManualRequired(ctx, comp.ID, err.Error()); err != nil {
					result.FailedCount++
					continue
				}
				result.ManualCount++
				p.stats.RecordCompensationManual(comp.OperationType)
			} else {
				// 继续重试
				if err := p.store.UpdateStatus(ctx, comp.ID, CompensationStatusRetrying); err != nil {
					result.FailedCount++
					continue
				}
				result.RetryCount++
				p.stats.RecordCompensationRetry(comp.OperationType)
			}
		} else {
			// 执行成功，标记解决
			if err := p.store.Resolve(ctx, comp.ID, 0, "auto_resolved"); err != nil {
				result.FailedCount++
				continue
			}
			result.SuccessCount++
			p.stats.RecordCompensationResolved(comp.OperationType)
		}
	}

	return result, nil
}

// CompensationResult 补偿处理结果
type CompensationResult struct {
	BatchID       string `json:"batch_id"`
	TotalItems    int    `json:"total_items"`
	SuccessCount  int    `json:"success_count"`
	RetryCount    int    `json:"retry_count"`
	ManualCount   int    `json:"manual_count"`
	FailedCount   int    `json:"failed_count"`
}

// SQLCompensationStore SQL实现的补偿存储
type SQLCompensationStore struct {
	pool *pgxpool.Pool
}

// NewSQLCompensationStore 创建SQL补偿存储
func NewSQLCompensationStore(pool *pgxpool.Pool) *SQLCompensationStore {
	return &SQLCompensationStore{pool: pool}
}

func (s *SQLCompensationStore) Create(ctx context.Context, comp *BatchCompensation) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO supply_batch_compensation (
			batch_id, operation_type, item_index, item_payload,
			failure_reason, status, max_retries, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, comp.BatchID, comp.OperationType, comp.ItemIndex, comp.ItemPayload,
		comp.FailureReason, CompensationStatusPending, comp.MaxRetries, comp.CreatedBy).
		Scan(&id)
	return id, err
}

func (s *SQLCompensationStore) GetByBatchID(ctx context.Context, batchID string) ([]*BatchCompensation, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, batch_id, operation_type, item_index, item_payload,
			   failure_reason, status, retry_count, max_retries,
			   resolved_at, resolved_by, resolution_notes,
			   created_at, updated_at, created_by, version
		FROM supply_batch_compensation
		WHERE batch_id = $1
		ORDER BY item_index
	`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var compensations []*BatchCompensation
	for rows.Next() {
		comp := &BatchCompensation{}
		err := rows.Scan(
			&comp.ID, &comp.BatchID, &comp.OperationType, &comp.ItemIndex,
			&comp.ItemPayload, &comp.FailureReason, &comp.Status,
			&comp.RetryCount, &comp.MaxRetries, &comp.ResolvedAt,
			&comp.ResolvedBy, &comp.ResolutionNotes, &comp.CreatedAt,
			&comp.UpdatedAt, &comp.CreatedBy, &comp.Version,
		)
		if err != nil {
			return nil, err
		}
		compensations = append(compensations, comp)
	}
	return compensations, rows.Err()
}

func (s *SQLCompensationStore) UpdateStatus(ctx context.Context, id int64, status string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE supply_batch_compensation
		SET status = $1, updated_at = CURRENT_TIMESTAMP, version = version + 1
		WHERE id = $2
	`, status, id)
	return err
}

func (s *SQLCompensationStore) Resolve(ctx context.Context, id int64, resolvedBy int64, notes string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE supply_batch_compensation
		SET status = $1, resolved_at = CURRENT_TIMESTAMP,
			resolved_by = $2, resolution_notes = $3,
			updated_at = CURRENT_TIMESTAMP, version = version + 1
		WHERE id = $4
	`, CompensationStatusResolved, resolvedBy, notes, id)
	return err
}

func (s *SQLCompensationStore) MarkManualRequired(ctx context.Context, id int64, reason string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE supply_batch_compensation
		SET status = $1, failure_reason = COALESCE(failure_reason || '; ', '') || $2,
			updated_at = CURRENT_TIMESTAMP, version = version + 1
		WHERE id = $3
	`, CompensationStatusManualRequired, reason, id)
	return err
}
