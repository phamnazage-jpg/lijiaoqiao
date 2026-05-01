package audit

import (
	"testing"
	"time"
)

func TestNewAuditEntry(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	event := Event{
		ID:        "test-id-123",
		SessionID: "session-456",
		TicketID:  "ticket-789",
		Type:      "ticket",
		Action:    "create",
		Channel:   "feishu",
		OpenID:    "ou_abc",
		ActorID:   "agent-001",
		SourceIP:  "192.168.1.1",
		Payload: map[string]any{
			"message": "hello",
		},
		BeforeState: map[string]any{
			"status": "open",
		},
		AfterState: map[string]any{
			"status": "resolved",
		},
		CreatedAt: now,
	}

	if event.ID != "test-id-123" {
		t.Errorf("expected ID test-id-123, got %s", event.ID)
	}
	if event.SessionID != "session-456" {
		t.Errorf("expected SessionID session-456, got %s", event.SessionID)
	}
	if event.TicketID != "ticket-789" {
		t.Errorf("expected TicketID ticket-789, got %s", event.TicketID)
	}
	if event.Type != "ticket" {
		t.Errorf("expected Type ticket, got %s", event.Type)
	}
	if event.Action != "create" {
		t.Errorf("expected Action create, got %s", event.Action)
	}
	if event.Channel != "feishu" {
		t.Errorf("expected Channel feishu, got %s", event.Channel)
	}
	if event.OpenID != "ou_abc" {
		t.Errorf("expected OpenID ou_abc, got %s", event.OpenID)
	}
	if event.ActorID != "agent-001" {
		t.Errorf("expected ActorID agent-001, got %s", event.ActorID)
	}
	if event.SourceIP != "192.168.1.1" {
		t.Errorf("expected SourceIP 192.168.1.1, got %s", event.SourceIP)
	}
	if event.Payload == nil {
		t.Fatal("expected non-nil Payload")
	}
	if event.Payload["message"] != "hello" {
		t.Errorf("expected Payload[message]=hello, got %v", event.Payload["message"])
	}
	if event.BeforeState == nil {
		t.Fatal("expected non-nil BeforeState")
	}
	if event.BeforeState["status"] != "open" {
		t.Errorf("expected BeforeState[status]=open, got %v", event.BeforeState["status"])
	}
	if event.AfterState == nil {
		t.Fatal("expected non-nil AfterState")
	}
	if event.AfterState["status"] != "resolved" {
		t.Errorf("expected AfterState[status]=resolved, got %v", event.AfterState["status"])
	}
	if !event.CreatedAt.Equal(now) {
		t.Errorf("expected CreatedAt %v, got %v", now, event.CreatedAt)
	}
}

func TestEvent_AllFieldsOptional(t *testing.T) {
	// Event should allow empty optional fields
	event := Event{
		Type: "session",
	}

	if event.ID != "" {
		t.Errorf("expected empty ID, got %s", event.ID)
	}
	if event.SessionID != "" {
		t.Errorf("expected empty SessionID, got %s", event.SessionID)
	}
	if event.TicketID != "" {
		t.Errorf("expected empty TicketID, got %s", event.TicketID)
	}
	if event.Action != "" {
		t.Errorf("expected empty Action, got %s", event.Action)
	}
	if event.Channel != "" {
		t.Errorf("expected empty Channel, got %s", event.Channel)
	}
	if event.OpenID != "" {
		t.Errorf("expected empty OpenID, got %s", event.OpenID)
	}
	if event.ActorID != "" {
		t.Errorf("expected empty ActorID, got %s", event.ActorID)
	}
	if event.SourceIP != "" {
		t.Errorf("expected empty SourceIP, got %s", event.SourceIP)
	}
	if event.Payload != nil {
		t.Errorf("expected nil Payload, got %v", event.Payload)
	}
	if event.BeforeState != nil {
		t.Errorf("expected nil BeforeState, got %v", event.BeforeState)
	}
	if event.AfterState != nil {
		t.Errorf("expected nil AfterState, got %v", event.AfterState)
	}
	if !event.CreatedAt.IsZero() {
		t.Errorf("expected zero CreatedAt, got %v", event.CreatedAt)
	}
}

func TestEvent_PayloadMap(t *testing.T) {
	event := Event{
		ID:   "id-1",
		Type: "ticket",
		Payload: map[string]any{
			"key1": "value1",
			"key2": float64(42),
			"key3": true,
			"key4": nil,
		},
	}

	if len(event.Payload) != 4 {
		t.Fatalf("expected 4 payload entries, got %d", len(event.Payload))
	}
	if event.Payload["key1"] != "value1" {
		t.Errorf("expected Payload[key1]=value1, got %v", event.Payload["key1"])
	}
	if event.Payload["key2"] != float64(42) {
		t.Errorf("expected Payload[key2]=42, got %v", event.Payload["key2"])
	}
	if event.Payload["key3"] != true {
		t.Errorf("expected Payload[key3]=true, got %v", event.Payload["key3"])
	}
}

func TestEvent_TicketAndSessionFields(t *testing.T) {
	// Ticket-scoped event
	ticketEvent := Event{
		ID:       "e1",
		TicketID: "t-1",
		Type:     "ticket",
		Action:   "resolve",
	}

	if ticketEvent.TicketID != "t-1" {
		t.Errorf("expected TicketID t-1, got %s", ticketEvent.TicketID)
	}

	// Session-scoped event
	sessionEvent := Event{
		ID:       "e2",
		SessionID: "s-1",
		Type:     "session",
		Action:   "message",
	}

	if sessionEvent.SessionID != "s-1" {
		t.Errorf("expected SessionID s-1, got %s", sessionEvent.SessionID)
	}
}
