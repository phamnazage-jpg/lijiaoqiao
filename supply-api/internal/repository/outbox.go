package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OutboxStatus Outbox事件状态
type OutboxStatus string

const (
	OutboxStatusPending    OutboxStatus = "pending"
	OutboxStatusProcessing OutboxStatus = "processing"
	OutboxStatusCompleted  OutboxStatus = "completed"
	OutboxStatusFailed     OutboxStatus = "failed"
	OutboxStatusDeadLetter OutboxStatus = "dead_letter"
)

// OutboxEvent Outbox事件
type OutboxEvent struct {
	ID               int64           `json:"id"`
	AggregateType    string          `json:"aggregate_type"`
	AggregateID      string          `json:"aggregate_id"`
	EventType        string          `json:"event_type"`
	EventID          string          `json:"event_id"`
	Payload          json.RawMessage `json:"payload"`
	Status           OutboxStatus    `json:"status"`
	RetryCount       int             `json:"retry_count"`
	MaxRetries       int             `json:"max_retries"`
	ErrorMessage     string          `json:"error_message,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	ProcessedAt      *time.Time      `json:"processed_at,omitempty"`
	NextRetryAt      *time.Time      `json:"next_retry_at,omitempty"`
	DeadLetterReason string          `json:"dead_letter_reason,omitempty"`
	Version          int64           `json:"version"`
}

// OutboxDeadLetter 死信记录
type OutboxDeadLetter struct {
	ID                    int64           `json:"id"`
	OriginalEventID       string          `json:"original_event_id"`
	OriginalAggregateType string          `json:"original_aggregate_type"`
	OriginalAggregateID   string          `json:"original_aggregate_id"`
	EventType             string          `json:"event_type"`
	Payload               json.RawMessage `json:"payload"`
	ErrorMessage          string          `json:"error_message,omitempty"`
	RetryCount            int             `json:"retry_count"`
	FirstFailedAt         time.Time       `json:"first_failed_at"`
	DeadLetterAt          time.Time       `json:"dead_letter_at"`
	Handled               bool            `json:"handled"`
	HandledAt             *time.Time      `json:"handled_at,omitempty"`
	HandlerNotes          string          `json:"handler_notes,omitempty"`
	CreatedAt             time.Time       `json:"created_at"`
}

// OutboxRepository Outbox仓储
type OutboxRepository struct {
	db outboxDB
}

type outboxDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
}

// NewOutboxRepository 创建Outbox仓储
func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{db: pool}
}

// Create 创建Outbox事件
func (r *OutboxRepository) Create(ctx context.Context, event *OutboxEvent) error {
	query := `
		INSERT INTO supply_outbox (
			aggregate_type, aggregate_id, event_type, event_id,
			payload, status, retry_count, max_retries
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`

	err := r.db.QueryRow(ctx, query,
		event.AggregateType, event.AggregateID, event.EventType, event.EventID,
		event.Payload, event.Status, event.RetryCount, event.MaxRetries,
	).Scan(&event.ID, &event.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create outbox event: %w", err)
	}
	return nil
}

// FetchAndLock 获取并锁定待处理事件（使用FOR UPDATE SKIP LOCKED实现分布式锁）
func (r *OutboxRepository) FetchAndLock(ctx context.Context, limit int) ([]*OutboxEvent, error) {
	query := `
		WITH claimed AS (
			SELECT id
			FROM supply_outbox
			WHERE status IN ('pending', 'failed')
			AND (next_retry_at IS NULL OR next_retry_at <= CURRENT_TIMESTAMP)
			ORDER BY created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE supply_outbox AS o
		SET status = 'processing',
			version = o.version + 1
		FROM claimed
		WHERE o.id = claimed.id
		RETURNING o.id, o.aggregate_type, o.aggregate_id, o.event_type, o.event_id,
			o.payload, o.status, o.retry_count, o.max_retries, o.error_message,
			o.created_at, o.processed_at, o.next_retry_at, o.version
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch outbox events: %w", err)
	}
	defer rows.Close()

	var events []*OutboxEvent
	for rows.Next() {
		event := &OutboxEvent{}
		err := rows.Scan(
			&event.ID, &event.AggregateType, &event.AggregateID, &event.EventType,
			&event.EventID, &event.Payload, &event.Status, &event.RetryCount,
			&event.MaxRetries, &event.ErrorMessage, &event.CreatedAt,
			&event.ProcessedAt, &event.NextRetryAt, &event.Version,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan outbox event: %w", err)
		}

		events = append(events, event)
	}

	return events, nil
}

// MarkCompleted 标记事件为已完成
func (r *OutboxRepository) MarkCompleted(ctx context.Context, eventID string) error {
	query := `
		UPDATE supply_outbox SET
			status = 'completed',
			processed_at = CURRENT_TIMESTAMP,
			version = version + 1
		WHERE event_id = $1 AND status = 'processing'
	`

	result, err := r.db.Exec(ctx, query, eventID)
	if err != nil {
		return fmt.Errorf("failed to mark outbox event completed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("event not found or not in processing status")
	}

	return nil
}

// MarkFailed 标记事件为失败（并更新重试信息）
func (r *OutboxRepository) MarkFailed(ctx context.Context, eventID string, errorMsg string, nextRetryAt *time.Time) error {
	query := `
		UPDATE supply_outbox SET
			status = 'failed',
			error_message = $2,
			next_retry_at = $3,
			version = version + 1
		WHERE event_id = $1 AND status = 'processing'
	`

	result, err := r.db.Exec(ctx, query, eventID, errorMsg, nextRetryAt)
	if err != nil {
		return fmt.Errorf("failed to mark outbox event failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("event not found or not in processing status")
	}

	return nil
}

// MoveToDeadLetter 将事件移入死信队列
func (r *OutboxRepository) MoveToDeadLetter(ctx context.Context, event *OutboxEvent, errorMsg string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 获取事件详情
	selectQuery := `
		SELECT id, aggregate_type, aggregate_id, event_type, event_id,
			payload, retry_count, created_at
		FROM supply_outbox
		WHERE event_id = $1
		FOR UPDATE
	`

	var originalEvent OutboxEvent
	err = tx.QueryRow(ctx, selectQuery, event.EventID).Scan(
		&originalEvent.ID, &originalEvent.AggregateType, &originalEvent.AggregateID,
		&originalEvent.EventType, &originalEvent.EventID, &originalEvent.Payload,
		&originalEvent.RetryCount, &originalEvent.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to get original event: %w", err)
	}

	// 插入死信记录
	insertQuery := `
		INSERT INTO supply_outbox_dead_letter (
			original_event_id, original_aggregate_type, original_aggregate_id,
			event_type, payload, error_message, retry_count, first_failed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err = tx.Exec(ctx, insertQuery,
		originalEvent.EventID, originalEvent.AggregateType, originalEvent.AggregateID,
		originalEvent.EventType, originalEvent.Payload, errorMsg,
		originalEvent.RetryCount, originalEvent.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert dead letter: %w", err)
	}

	// 删除原始事件
	deleteQuery := `DELETE FROM supply_outbox WHERE event_id = $1`
	_, err = tx.Exec(ctx, deleteQuery, event.EventID)
	if err != nil {
		return fmt.Errorf("failed to delete original event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetDeadLetterByEventID 根据原始事件ID获取死信记录
func (r *OutboxRepository) GetDeadLetterByEventID(ctx context.Context, originalEventID string) (*OutboxDeadLetter, error) {
	query := `
		SELECT id, original_event_id, original_aggregate_type, original_aggregate_id,
			event_type, payload, error_message, retry_count, first_failed_at,
			dead_letter_at, handled, handled_at, handler_notes, created_at
		FROM supply_outbox_dead_letter
		WHERE original_event_id = $1
	`

	dl := &OutboxDeadLetter{}
	err := r.db.QueryRow(ctx, query, originalEventID).Scan(
		&dl.ID, &dl.OriginalEventID, &dl.OriginalAggregateType, &dl.OriginalAggregateID,
		&dl.EventType, &dl.Payload, &dl.ErrorMessage, &dl.RetryCount,
		&dl.FirstFailedAt, &dl.DeadLetterAt, &dl.Handled, &dl.HandledAt,
		&dl.HandlerNotes, &dl.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get dead letter: %w", err)
	}

	return dl, nil
}

// ListUnhandledDeadLetters 列出未处理的死信记录
func (r *OutboxRepository) ListUnhandledDeadLetters(ctx context.Context, limit int) ([]*OutboxDeadLetter, error) {
	query := `
		SELECT id, original_event_id, original_aggregate_type, original_aggregate_id,
			event_type, payload, error_message, retry_count, first_failed_at,
			dead_letter_at, handled, handled_at, handler_notes, created_at
		FROM supply_outbox_dead_letter
		WHERE handled = FALSE
		ORDER BY dead_letter_at ASC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list dead letters: %w", err)
	}
	defer rows.Close()

	var dls []*OutboxDeadLetter
	for rows.Next() {
		dl := &OutboxDeadLetter{}
		err := rows.Scan(
			&dl.ID, &dl.OriginalEventID, &dl.OriginalAggregateType, &dl.OriginalAggregateID,
			&dl.EventType, &dl.Payload, &dl.ErrorMessage, &dl.RetryCount,
			&dl.FirstFailedAt, &dl.DeadLetterAt, &dl.Handled, &dl.HandledAt,
			&dl.HandlerNotes, &dl.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan dead letter: %w", err)
		}
		dls = append(dls, dl)
	}

	return dls, nil
}

// MarkDeadLetterHandled 标记死信已处理
func (r *OutboxRepository) MarkDeadLetterHandled(ctx context.Context, id int64, notes string) error {
	query := `
		UPDATE supply_outbox_dead_letter SET
			handled = TRUE,
			handled_at = CURRENT_TIMESTAMP,
			handler_notes = $2
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, id, notes)
	if err != nil {
		return fmt.Errorf("failed to mark dead letter handled: %w", err)
	}

	return nil
}

// DeleteCompleted 删除已完成的旧事件（定时清理）
func (r *OutboxRepository) DeleteCompleted(ctx context.Context, before time.Time) (int64, error) {
	query := `DELETE FROM supply_outbox WHERE status = 'completed' AND processed_at < $1`

	result, err := r.db.Exec(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("failed to delete completed events: %w", err)
	}

	return result.RowsAffected(), nil
}
