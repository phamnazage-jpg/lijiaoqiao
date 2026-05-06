package platformevent

import (
	"fmt"
	"strings"
	"time"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusRetrying   Status = "retrying"
	StatusDelivered  Status = "delivered"
	StatusDeadLetter Status = "dead_letter"
)

const (
	TypeMessageReceived   = "message.received"
	TypeMessageRejected   = "message.rejected"
	TypeMessageDeduped    = "message.deduplicated"
	TypeMessageProcessing = "message.processing"
	TypeIntentResolved    = "intent.resolved"
	TypeHandoffTriggered  = "handoff.triggered"
	TypeTicketCreated     = "ticket.created"
	TypeTicketAssigned    = "ticket.assigned"
	TypeTicketResolved    = "ticket.resolved"
	TypeTicketClosed      = "ticket.closed"
	TypeReplyGenerated    = "reply.generated"
	TypeCallbackDelivered = "callback.delivered"
	TypeCallbackFailed    = "callback.failed"
)

type Event struct {
	ID              string         `json:"event_id"`
	Platform        string         `json:"platform"`
	EventType       string         `json:"event_type"`
	SessionID       string         `json:"session_id,omitempty"`
	TicketID        string         `json:"ticket_id,omitempty"`
	SourceMessageID string         `json:"source_message_id,omitempty"`
	CallbackTarget  string         `json:"callback_target"`
	Payload         map[string]any `json:"payload"`
	Status          Status         `json:"status"`
	AttemptCount    int            `json:"attempt_count"`
	NextAttemptAt   time.Time      `json:"next_attempt_at"`
	OccurredAt      time.Time      `json:"occurred_at"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeliveredAt     *time.Time     `json:"delivered_at,omitempty"`
	LastError       string         `json:"last_error,omitempty"`
}

func (e Event) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("event id is required")
	}
	if strings.TrimSpace(e.Platform) == "" {
		return fmt.Errorf("platform is required")
	}
	if strings.TrimSpace(e.EventType) == "" {
		return fmt.Errorf("event type is required")
	}
	if strings.TrimSpace(e.CallbackTarget) == "" {
		return fmt.Errorf("callback target is required")
	}
	switch e.Status {
	case StatusPending, StatusRetrying, StatusDelivered, StatusDeadLetter:
	default:
		return fmt.Errorf("invalid status: %s", e.Status)
	}
	if e.AttemptCount < 0 {
		return fmt.Errorf("attempt count must not be negative")
	}
	if e.NextAttemptAt.IsZero() {
		return fmt.Errorf("next attempt at is required")
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("occurred at is required")
	}
	return nil
}
