package ticket

import (
	"testing"
	"time"
)

func TestTicket_ID(t *testing.T) {
	// Ticket struct directly - verify ID field behavior
	tk := Ticket{
		ID:     "test-ticket-001",
		Status: StatusOpen,
	}
	if tk.ID != "test-ticket-001" {
		t.Errorf("expected ID 'test-ticket-001', got %q", tk.ID)
	}
}

func TestTicket_Status(t *testing.T) {
	tests := []struct {
		name       string
		initial    Status
		transition Status
	}{
		{"open to assigned", StatusOpen, StatusAssigned},
		{"assigned to processing", StatusAssigned, StatusProcessing},
		{"processing to resolved", StatusProcessing, StatusResolved},
		{"resolved to closed", StatusResolved, StatusClosed},
		{"open directly to closed", StatusOpen, StatusClosed},
	}

	for _, tt := range tests {
		tk := Ticket{Status: tt.initial}
		if tk.Status != tt.initial {
			t.Errorf("%s: expected status %q, got %q", tt.name, tt.initial, tk.Status)
		}
		tk.Status = tt.transition
		if tk.Status != tt.transition {
			t.Errorf("%s: expected transitioned status %q, got %q", tt.name, tt.transition, tk.Status)
		}
	}
}

func TestTicket_StatusConstants(t *testing.T) {
	// Verify status constants have expected values
	if StatusOpen != "open" {
		t.Errorf("StatusOpen: expected 'open', got %q", StatusOpen)
	}
	if StatusAssigned != "assigned" {
		t.Errorf("StatusAssigned: expected 'assigned', got %q", StatusAssigned)
	}
	if StatusProcessing != "processing" {
		t.Errorf("StatusProcessing: expected 'processing', got %q", StatusProcessing)
	}
	if StatusResolved != "resolved" {
		t.Errorf("StatusResolved: expected 'resolved', got %q", StatusResolved)
	}
	if StatusClosed != "closed" {
		t.Errorf("StatusClosed: expected 'closed', got %q", StatusClosed)
	}
}

func TestTicket_PriorityConstants(t *testing.T) {
	if PriorityP0 != "P0" {
		t.Errorf("PriorityP0: expected 'P0', got %q", PriorityP0)
	}
	if PriorityP1 != "P1" {
		t.Errorf("PriorityP1: expected 'P1', got %q", PriorityP1)
	}
	if PriorityP2 != "P2" {
		t.Errorf("PriorityP2: expected 'P2', got %q", PriorityP2)
	}
	if PriorityP3 != "P3" {
		t.Errorf("PriorityP3: expected 'P3', got %q", PriorityP3)
	}
}

func TestTicket_Fields(t *testing.T) {
	now := time.Now()
	resolvedAt := now.Add(24 * time.Hour)

	tk := Ticket{
		ID:              "ticket-123",
		SessionID:       "session-456",
		UserID:          "user-789",
		Priority:        PriorityP1,
		Status:          StatusOpen,
		HandoffReason:   "customer request",
		AssignedTo:      "agent-001",
		ContextSnapshot: map[string]any{"channel": "wechat", "locale": "zh-CN"},
		Resolution:      "resolved successfully",
		CreatedAt:       now,
		ResolvedAt:      &resolvedAt,
		UpdatedAt:       now,
	}

	if tk.ID != "ticket-123" {
		t.Errorf("ID: expected 'ticket-123', got %q", tk.ID)
	}
	if tk.SessionID != "session-456" {
		t.Errorf("SessionID: expected 'session-456', got %q", tk.SessionID)
	}
	if tk.UserID != "user-789" {
		t.Errorf("UserID: expected 'user-789', got %q", tk.UserID)
	}
	if tk.Priority != PriorityP1 {
		t.Errorf("Priority: expected 'P1', got %q", tk.Priority)
	}
	if tk.Status != StatusOpen {
		t.Errorf("Status: expected 'open', got %q", tk.Status)
	}
	if tk.HandoffReason != "customer request" {
		t.Errorf("HandoffReason: expected 'customer request', got %q", tk.HandoffReason)
	}
	if tk.AssignedTo != "agent-001" {
		t.Errorf("AssignedTo: expected 'agent-001', got %q", tk.AssignedTo)
	}
	if tk.ContextSnapshot["channel"] != "wechat" {
		t.Errorf("ContextSnapshot[channel]: expected 'wechat', got %v", tk.ContextSnapshot["channel"])
	}
	if tk.Resolution != "resolved successfully" {
		t.Errorf("Resolution: expected 'resolved successfully', got %q", tk.Resolution)
	}
	if tk.CreatedAt != now {
		t.Errorf("CreatedAt mismatch")
	}
	if tk.ResolvedAt == nil || !tk.ResolvedAt.Equal(resolvedAt) {
		t.Errorf("ResolvedAt: expected %v, got %v", resolvedAt, tk.ResolvedAt)
	}
}

func TestTicket_ResolvedAtOptional(t *testing.T) {
	// Test that ResolvedAt can be nil (open ticket)
	tk := Ticket{
		ID:        "open-ticket",
		Status:    StatusOpen,
		ResolvedAt: nil,
	}
	if tk.ResolvedAt != nil {
		t.Errorf("ResolvedAt should be nil for open ticket, got %v", tk.ResolvedAt)
	}
}

func TestTicket_StatusTransitions(t *testing.T) {
	// Test typical ticket lifecycle
	tk := Ticket{Status: StatusOpen}

	// Open -> Assigned
	tk.Status = StatusAssigned
	if tk.Status != StatusAssigned {
		t.Error("failed to transition to Assigned")
	}

	// Assigned -> Processing
	tk.Status = StatusProcessing
	if tk.Status != StatusProcessing {
		t.Error("failed to transition to Processing")
	}

	// Processing -> Resolved
	tk.Status = StatusResolved
	now := time.Now()
	tk.ResolvedAt = &now
	if tk.Status != StatusResolved || tk.ResolvedAt == nil {
		t.Error("failed to transition to Resolved")
	}

	// Resolved -> Closed
	tk.Status = StatusClosed
	if tk.Status != StatusClosed {
		t.Error("failed to transition to Closed")
	}
}