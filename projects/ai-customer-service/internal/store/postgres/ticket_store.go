package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/ticket"
	"github.com/bridge/ai-customer-service/internal/domain/ticketstats"
)

type TicketStore struct {
	db *sql.DB
}

func NewTicketStore(db *sql.DB) *TicketStore {
	return &TicketStore{db: db}
}

func (s *TicketStore) ListAll(ctx context.Context) ([]ticket.Ticket, error) {
	if s.db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id::text, session_id::text, COALESCE(user_id,''), priority, status, handoff_reason, COALESCE(assigned_to,''), context_snapshot, COALESCE(resolution,''), created_at, resolved_at, updated_at FROM cs_tickets ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ticket.Ticket, 0, 8)
	for rows.Next() {
		var (
			item       ticket.Ticket
			payload    []byte
			resolvedAt sql.NullTime
		)
		if err := rows.Scan(&item.ID, &item.SessionID, &item.UserID, &item.Priority, &item.Status, &item.HandoffReason, &item.AssignedTo, &payload, &item.Resolution, &item.CreatedAt, &resolvedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if len(payload) > 0 {
			_ = json.Unmarshal(payload, &item.ContextSnapshot)
		}
		if resolvedAt.Valid {
			value := resolvedAt.Time
			item.ResolvedAt = &value
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *TicketStore) Create(ctx context.Context, t *ticket.Ticket) error {
	if s.db == nil {
		return fmt.Errorf("db is nil")
	}
	if t == nil {
		return fmt.Errorf("ticket is nil")
	}
	if t.CreatedAt.IsZero() {
		now := time.Now()
		t.CreatedAt = now
		t.UpdatedAt = now
	}
	payload, err := json.Marshal(t.ContextSnapshot)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO cs_tickets(id, session_id, user_id, priority, status, handoff_reason, assigned_to, context_snapshot, resolution, created_at, resolved_at, updated_at) VALUES ($1::uuid,$2::uuid,NULLIF($3,''),$4,$5,$6,NULLIF($7,''),$8::jsonb,NULLIF($9,''),$10,$11,$12)`, t.ID, t.SessionID, t.UserID, string(t.Priority), string(t.Status), t.HandoffReason, t.AssignedTo, string(payload), t.Resolution, t.CreatedAt, t.ResolvedAt, t.UpdatedAt)
	return err
}

func (s *TicketStore) GetByID(ctx context.Context, id string) (*ticket.Ticket, error) {
	if s.db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	var t ticket.Ticket
	var payload []byte
	var resolvedAt sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT id::text, session_id::text, COALESCE(user_id,''), priority, status, handoff_reason, COALESCE(assigned_to,''), context_snapshot, COALESCE(resolution,''), created_at, resolved_at, updated_at FROM cs_tickets WHERE id = $1::uuid`,
		id,
	).Scan(&t.ID, &t.SessionID, &t.UserID, &t.Priority, &t.Status, &t.HandoffReason, &t.AssignedTo, &payload, &t.Resolution, &t.CreatedAt, &resolvedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &t.ContextSnapshot)
	}
	if resolvedAt.Valid {
		value := resolvedAt.Time
		t.ResolvedAt = &value
	}
	return &t, nil
}

// GetStats aggregates ticket statistics for monitoring and dashboards.
func (s *TicketStore) GetStats(ctx context.Context) (ticketstats.Stats, error) {
	if s.db == nil {
		return ticketstats.Stats{}, fmt.Errorf("db is nil")
	}
	var stats ticketstats.Stats
	stats.ByChannel = make(map[string]int)
	stats.ByPriority = make(map[string]int)

	// Total counts by status
	rows, err := s.db.QueryContext(ctx, `
		SELECT status, COUNT(*)::int FROM cs_tickets GROUP BY status
	`)
	if err != nil {
		return stats, err
	}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return stats, err
		}
		stats.Total += count
		switch status {
		case "open", "assigned", "processing":
			stats.Open += count
		case "resolved":
			stats.Resolved += count
		case "closed":
			stats.Closed += count
		}
	}
	if err := rows.Err(); err != nil {
		return stats, err
	}

	// By channel (via session join)
	rows, err = s.db.QueryContext(ctx, `
		SELECT COALESCE(cs_sessions.channel, 'unknown'), COUNT(*)::int
		FROM cs_tickets
		JOIN cs_sessions ON cs_tickets.session_id = cs_sessions.id
		GROUP BY cs_sessions.channel
	`)
	if err != nil {
		return stats, err
	}
	for rows.Next() {
		var channel string
		var count int
		if err := rows.Scan(&channel, &count); err != nil {
			return stats, err
		}
		stats.ByChannel[channel] = count
	}
	if err := rows.Err(); err != nil {
		return stats, err
	}

	// By priority
	rows, err = s.db.QueryContext(ctx, `
		SELECT priority, COUNT(*)::int FROM cs_tickets GROUP BY priority
	`)
	if err != nil {
		return stats, err
	}
	for rows.Next() {
		var priority string
		var count int
		if err := rows.Scan(&priority, &count); err != nil {
			return stats, err
		}
		stats.ByPriority[priority] = count
	}
	if err := rows.Err(); err != nil {
		return stats, err
	}

	// Handoff count (tickets with non-empty handoff_reason)
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)::int FROM cs_tickets WHERE handoff_reason <> ''
	`).Scan(&stats.HandoffCount); err != nil {
		return stats, err
	}

	// Average resolution time in minutes (only resolved/closed tickets with resolved_at)
	var avgSeconds sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, `
		SELECT AVG(EXTRACT(EPOCH FROM (resolved_at - created_at)))::float
		FROM cs_tickets
		WHERE resolved_at IS NOT NULL
	`).Scan(&avgSeconds); err != nil {
		return stats, err
	}
	if avgSeconds.Valid {
		stats.AvgResolutionTimeMinutes = avgSeconds.Float64 / 60.0
	}

	return stats, nil
}
