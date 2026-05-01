package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
)

type AuditStore struct {
	db *sql.DB
}

func NewAuditStore(db *sql.DB) *AuditStore {
	return &AuditStore{db: db}
}

func (s *AuditStore) Add(ctx context.Context, event audit.Event) error {
	if s.db == nil {
		return fmt.Errorf("db is nil")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	beforeState, err := marshalJSON(event.BeforeState)
	if err != nil {
		return err
	}
	afterState, err := marshalJSON(resolveAfterState(event))
	if err != nil {
		return err
	}
	objectType, objectID := resolveAuditObject(event)
	action := strings.TrimSpace(event.Action)
	if action == "" {
		action = "update"
	}
	actorID := strings.TrimSpace(event.ActorID)
	if actorID == "" {
		actorID = coalesceActor(event.OpenID)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO cs_audit_logs(id, tenant_id, object_type, object_id, action, before_state, after_state, actor_id, source_ip, created_at) VALUES ($1::uuid, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8, NULLIF($9,''), $10)`, event.ID, "default", objectType, objectID, action, beforeState, afterState, actorID, event.SourceIP, event.CreatedAt)
	return err
}

func marshalJSON(value map[string]any) (string, error) {
	if len(value) == 0 {
		return "{}", nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func resolveAfterState(event audit.Event) map[string]any {
	if len(event.AfterState) > 0 {
		return event.AfterState
	}
	if len(event.Payload) > 0 {
		return event.Payload
	}
	return map[string]any{}
}

func resolveAuditObject(event audit.Event) (string, string) {
	if strings.TrimSpace(event.TicketID) != "" {
		return "ticket", event.TicketID
	}
	if strings.TrimSpace(event.SessionID) != "" {
		return event.Type, event.SessionID
	}
	return event.Type, "system"
}

func coalesceActor(actor string) string {
	if actor == "" {
		return "system"
	}
	return actor
}
