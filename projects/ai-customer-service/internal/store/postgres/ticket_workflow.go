package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
	"github.com/bridge/ai-customer-service/internal/domain/ticket"
)

// TicketWorkflowStore composes TicketStore with AuditStore for workflow operations.
type TicketWorkflowStore struct {
	*TicketStore
	audit *AuditStore
	log   *slog.Logger
}

// NewTicketWorkflowStore creates a TicketWorkflowStore that writes audit logs for Assign/Resolve/Close.
func NewTicketWorkflowStore(db *sql.DB, auditStore *AuditStore) *TicketWorkflowStore {
	return &TicketWorkflowStore{
		TicketStore: NewTicketStore(db),
		audit:       auditStore,
		log:         slog.Default(),
	}
}

// writeAudit writes an audit log for a ticket workflow action.
// Errors are only logged and never returned, per fail-closed policy.
func (s *TicketWorkflowStore) writeAudit(ctx context.Context, ticketID, action, actorID, sourceIP string, afterState map[string]any) {
	if s.audit == nil {
		return
	}
	now := time.Now()
	event := audit.Event{
		ID:        fmt.Sprintf("wf-%d", now.UnixNano()),
		Type:      "ticket_state_changed",
		Action:    action,
		TicketID:  ticketID,
		ActorID:   actorID,
		SourceIP:  sourceIP,
		AfterState: afterState,
		CreatedAt: now,
	}
	if err := s.audit.Add(ctx, event); err != nil {
		if s.log != nil {
			s.log.Error("ticket workflow audit write failed", "ticket_id", ticketID, "action", action, "error", err.Error())
		}
	}
}

func (s *TicketStore) ListOpen(ctx context.Context, limit int) ([]ticket.Ticket, error) {
	if s.db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id::text, session_id::text, COALESCE(user_id,''), priority, status, handoff_reason, COALESCE(assigned_to,''), context_snapshot, COALESCE(resolution,''), created_at, resolved_at, updated_at FROM cs_tickets WHERE status IN ('open','assigned','processing') ORDER BY CASE priority WHEN 'P0' THEN 0 WHEN 'P1' THEN 1 WHEN 'P2' THEN 2 ELSE 3 END, created_at ASC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ticket.Ticket, 0, limit)
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

func (s *TicketWorkflowStore) Assign(ctx context.Context, ticketID, agentID, actorID, sourceIP string, now time.Time) error {
	if s.db == nil {
		return fmt.Errorf("db is nil")
	}
	// P0-2 fix: first check if ticket exists and its current status
	var currentStatus string
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(status,'') FROM cs_tickets WHERE id = $1::uuid`, ticketID).Scan(&currentStatus)
	if err != nil {
		// ticket does not exist
		return fmt.Errorf("CS_TICKET_4001:ticket not found")
	}
	if currentStatus != "open" {
		// ticket exists but not in 'open' state
		if currentStatus == "assigned" || currentStatus == "processing" || currentStatus == "resolved" || currentStatus == "closed" {
			return fmt.Errorf("CS_TKT_4002:ticket already assigned")
		}
		return fmt.Errorf("CS_TKT_4002:ticket state conflict")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE cs_tickets SET assigned_to = NULLIF($2,''), status = 'assigned', updated_at = $3 WHERE id = $1::uuid AND status = 'open'`, ticketID, agentID, now)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("CS_TKT_4002:ticket already assigned")
	}
	s.writeAudit(ctx, ticketID, "assign", actorID, sourceIP, map[string]any{"assigned_to": agentID, "status": ticket.StatusAssigned})
	return nil
}

func (s *TicketWorkflowStore) Resolve(ctx context.Context, ticketID, resolution, actorID, sourceIP string, now time.Time) error {
	if s.db == nil {
		return fmt.Errorf("db is nil")
	}
	// P0-2 fix: first check if ticket exists and its current status
	var currentStatus string
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(status,'') FROM cs_tickets WHERE id = $1::uuid`, ticketID).Scan(&currentStatus)
	if err != nil {
		// ticket does not exist
		return fmt.Errorf("CS_TICKET_4001:ticket not found")
	}
	if currentStatus == "" {
		return fmt.Errorf("CS_TICKET_4001:ticket not found")
	}
	if currentStatus == "resolved" || currentStatus == "closed" {
		return fmt.Errorf("CS_TICKET_4092:ticket resolve conflict")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE cs_tickets SET resolution = NULLIF($2,''), status = 'resolved', resolved_at = $3, updated_at = $3 WHERE id = $1::uuid AND status IN ('assigned','processing','open')`, ticketID, resolution, now)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("CS_TICKET_4092:ticket resolve conflict")
	}
	s.writeAudit(ctx, ticketID, "resolve", actorID, sourceIP, map[string]any{"resolution": resolution, "status": ticket.StatusResolved})
	return nil
}

func (s *TicketWorkflowStore) Close(ctx context.Context, ticketID, resolution, actorID, sourceIP string, now time.Time) error {
	if s.db == nil {
		return fmt.Errorf("db is nil")
	}
	// P0-2 fix: first check if ticket exists and its current status
	var currentStatus string
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(status,'') FROM cs_tickets WHERE id = $1::uuid`, ticketID).Scan(&currentStatus)
	if err != nil {
		// ticket does not exist
		return fmt.Errorf("CS_TICKET_4001:ticket not found")
	}
	if currentStatus == "" {
		return fmt.Errorf("CS_TICKET_4001:ticket not found")
	}
	if currentStatus == "closed" {
		return fmt.Errorf("CS_TICKET_4093:ticket close conflict")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE cs_tickets SET resolution = NULLIF($2,''), status = 'closed', resolved_at = COALESCE(resolved_at, $3), updated_at = $3 WHERE id = $1::uuid AND status IN ('resolved','assigned','processing')`, ticketID, resolution, now)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("CS_TICKET_4093:ticket close conflict")
	}
	s.writeAudit(ctx, ticketID, "close", actorID, sourceIP, map[string]any{"resolution": resolution, "status": ticket.StatusClosed})
	return nil
}
