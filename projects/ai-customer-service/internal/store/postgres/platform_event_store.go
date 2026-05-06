package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/platformevent"
)

type PlatformEventStore struct {
	db *sql.DB
}

func NewPlatformEventStore(db *sql.DB) *PlatformEventStore {
	return &PlatformEventStore{db: db}
}

func (s *PlatformEventStore) InsertPending(ctx context.Context, event *platformevent.Event) error {
	if event == nil {
		return fmt.Errorf("event is nil")
	}
	return s.InsertPendingBatch(ctx, []platformevent.Event{*event})
}

func (s *PlatformEventStore) InsertPendingBatch(ctx context.Context, events []platformevent.Event) error {
	if s.db == nil {
		return fmt.Errorf("db is nil")
	}
	if len(events) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := event.Validate(); err != nil {
			_ = tx.Rollback()
			return err
		}
		payload, err := json.Marshal(event.Payload)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO cs_platform_event_outbox(
				id, platform, event_type, session_id, ticket_id, source_message_id, callback_target,
				payload, status, attempt_count, next_attempt_at, occurred_at, delivered_at, last_error, created_at, updated_at
			) VALUES (
				$1, $2, $3, NULLIF($4,'')::uuid, NULLIF($5,'')::uuid, $6, $7,
				$8::jsonb, $9, $10, $11, $12, $13, NULLIF($14,''), $15, $16
			)
		`, event.ID, event.Platform, event.EventType, event.SessionID, event.TicketID, event.SourceMessageID, event.CallbackTarget,
			string(payload), string(event.Status), event.AttemptCount, event.NextAttemptAt, event.OccurredAt, event.DeliveredAt, event.LastError, event.CreatedAt, event.UpdatedAt); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *PlatformEventStore) ListDue(ctx context.Context, platform string, dueBefore time.Time, limit int) ([]platformevent.Event, error) {
	if s.db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive")
	}
	platform = strings.TrimSpace(platform)
	if platform == "" {
		return nil, fmt.Errorf("platform is required")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, platform, event_type, COALESCE(session_id::text, ''), COALESCE(ticket_id::text, ''), COALESCE(source_message_id, ''),
		       callback_target, payload, status, attempt_count, next_attempt_at, occurred_at, created_at, updated_at,
		       delivered_at, COALESCE(last_error, '')
		FROM cs_platform_event_outbox
		WHERE platform = $1 AND status IN ('pending', 'retrying') AND next_attempt_at <= $2
		ORDER BY next_attempt_at ASC, created_at ASC
		LIMIT $3
	`, platform, dueBefore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]platformevent.Event, 0, limit)
	for rows.Next() {
		var (
			event       platformevent.Event
			payloadJSON []byte
			status      string
		)
		if err := rows.Scan(
			&event.ID,
			&event.Platform,
			&event.EventType,
			&event.SessionID,
			&event.TicketID,
			&event.SourceMessageID,
			&event.CallbackTarget,
			&payloadJSON,
			&status,
			&event.AttemptCount,
			&event.NextAttemptAt,
			&event.OccurredAt,
			&event.CreatedAt,
			&event.UpdatedAt,
			&event.DeliveredAt,
			&event.LastError,
		); err != nil {
			return nil, err
		}
		event.Status = platformevent.Status(status)
		if len(payloadJSON) > 0 {
			if err := json.Unmarshal(payloadJSON, &event.Payload); err != nil {
				return nil, err
			}
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (s *PlatformEventStore) MarkDelivered(ctx context.Context, eventID string, deliveredAt time.Time) error {
	if s.db == nil {
		return fmt.Errorf("db is nil")
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE cs_platform_event_outbox
		SET status = 'delivered', delivered_at = $2, updated_at = $2
		WHERE id = $1
	`, eventID, deliveredAt)
	return err
}

func (s *PlatformEventStore) RecordDeliveryAttempt(ctx context.Context, eventID string, attemptNo int, responseStatus int, responseBody string, errorMessage string) error {
	if s.db == nil {
		return fmt.Errorf("db is nil")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO cs_platform_event_delivery_attempts(event_id, attempt_no, response_status, response_body, error_message)
		VALUES ($1, $2, NULLIF($3, 0), NULLIF($4, ''), NULLIF($5, ''))
	`, eventID, attemptNo, responseStatus, responseBody, errorMessage)
	return err
}

func (s *PlatformEventStore) MarkRetry(ctx context.Context, eventID string, attemptCount int, nextAttemptAt time.Time, lastError string) error {
	if s.db == nil {
		return fmt.Errorf("db is nil")
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE cs_platform_event_outbox
		SET status = 'retrying', attempt_count = $2, next_attempt_at = $3, last_error = NULLIF($4,''), updated_at = NOW()
		WHERE id = $1
	`, eventID, attemptCount, nextAttemptAt, lastError)
	return err
}

func (s *PlatformEventStore) MarkDeadLetter(ctx context.Context, eventID string, attemptCount int, finalError string) error {
	if s.db == nil {
		return fmt.Errorf("db is nil")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE cs_platform_event_outbox
		SET status = 'dead_letter', attempt_count = $2, last_error = NULLIF($3,''), updated_at = NOW()
		WHERE id = $1
	`, eventID, attemptCount, finalError); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO cs_platform_event_dead_letters(event_id, platform, event_type, callback_target, payload, attempt_count, final_error)
		SELECT id, platform, event_type, callback_target, payload, attempt_count, last_error
		FROM cs_platform_event_outbox
		WHERE id = $1
		ON CONFLICT (event_id) DO UPDATE
		SET attempt_count = EXCLUDED.attempt_count, final_error = EXCLUDED.final_error, payload = EXCLUDED.payload
	`, eventID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
