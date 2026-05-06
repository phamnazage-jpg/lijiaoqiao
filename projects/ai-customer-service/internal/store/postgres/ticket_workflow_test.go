package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/ticket"
)

func TestTicketWorkflowStore_ListOpen(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	sessionStore := NewSessionStore(db)
	ticketStore := NewTicketStore(db)
	auditStore := NewAuditStore(db)
	workflowStore := NewTicketWorkflowStore(db, auditStore)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	// Create session
	sess, err := sessionStore.GetOrCreate(ctx, "widget", uniqueID("user"), now)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Create tickets with different statuses
	openTicket := &ticket.Ticket{
		ID:        uniqueID("tick"),
		SessionID: sess.ID,
		UserID:    "user1",
		Priority:  ticket.PriorityP1,
		Status:    ticket.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	assignedTicket := &ticket.Ticket{
		ID:         uniqueID("tick"),
		SessionID:  sess.ID,
		UserID:     "user2",
		Priority:   ticket.PriorityP2,
		Status:     ticket.StatusAssigned,
		AssignedTo: "agent1",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	resolvedTicket := &ticket.Ticket{
		ID:        uniqueID("tick"),
		SessionID: sess.ID,
		UserID:    "user3",
		Priority:  ticket.PriorityP3,
		Status:    ticket.StatusResolved,
		CreatedAt: now,
		UpdatedAt: now,
	}

	ticketStore.Create(ctx, openTicket)
	ticketStore.Create(ctx, assignedTicket)
	ticketStore.Create(ctx, resolvedTicket)

	// ListOpen should return open + assigned (not resolved)
	openList, err := workflowStore.ListOpen(ctx, 10)
	if err != nil {
		t.Fatalf("ListOpen() error = %v", err)
	}
	if len(openList) < 2 {
		t.Errorf("ListOpen() len = %d, want >= 2 (open + assigned)", len(openList))
	}
}

func TestTicketWorkflowStore_Assign(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	sessionStore := NewSessionStore(db)
	ticketStore := NewTicketStore(db)
	auditStore := NewAuditStore(db)
	workflowStore := NewTicketWorkflowStore(db, auditStore)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	// Create session + open ticket
	sess, err := sessionStore.GetOrCreate(ctx, "widget", uniqueID("user"), now)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	tkt := &ticket.Ticket{
		ID:        uniqueID("tick"),
		SessionID: sess.ID,
		UserID:    "user1",
		Priority:  ticket.PriorityP1,
		Status:    ticket.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := ticketStore.Create(ctx, tkt); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Assign it
	err = workflowStore.Assign(ctx, tkt.ID, "agent-001", "admin", "127.0.0.1", now)
	if err != nil {
		t.Fatalf("Assign() error = %v", err)
	}

	// Verify assigned
	fetched, _ := ticketStore.GetByID(ctx, tkt.ID)
	if fetched.Status != ticket.StatusAssigned {
		t.Errorf("ticket.Status = %s, want %s", fetched.Status, ticket.StatusAssigned)
	}
	if fetched.AssignedTo != "agent-001" {
		t.Errorf("ticket.AssignedTo = %s, want agent-001", fetched.AssignedTo)
	}
}

func TestTicketWorkflowStore_Assign_AlreadyAssigned(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	sessionStore := NewSessionStore(db)
	ticketStore := NewTicketStore(db)
	auditStore := NewAuditStore(db)
	workflowStore := NewTicketWorkflowStore(db, auditStore)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	// Create session + assigned ticket
	sess, _ := sessionStore.GetOrCreate(ctx, "widget", uniqueID("user"), now)
	tkt := &ticket.Ticket{
		ID:         uniqueID("tick"),
		SessionID:  sess.ID,
		UserID:     "user1",
		Priority:   ticket.PriorityP1,
		Status:     ticket.StatusAssigned,
		AssignedTo: "agent-001",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	ticketStore.Create(ctx, tkt)

	// Try to assign again — should fail
	err := workflowStore.Assign(ctx, tkt.ID, "agent-002", "admin", "127.0.0.1", now)
	if err == nil {
		t.Fatal("Assign() on already-assigned ticket should return error")
	}
}

func TestTicketWorkflowStore_Resolve(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	sessionStore := NewSessionStore(db)
	ticketStore := NewTicketStore(db)
	auditStore := NewAuditStore(db)
	workflowStore := NewTicketWorkflowStore(db, auditStore)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	// Create session + assigned ticket
	sess, _ := sessionStore.GetOrCreate(ctx, "widget", uniqueID("user"), now)
	tkt := &ticket.Ticket{
		ID:         uniqueID("tick"),
		SessionID:  sess.ID,
		UserID:     "user1",
		Priority:   ticket.PriorityP1,
		Status:     ticket.StatusAssigned,
		AssignedTo: "agent-001",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	ticketStore.Create(ctx, tkt)

	// Resolve it
	err := workflowStore.Resolve(ctx, tkt.ID, "user satisfied", "admin", "127.0.0.1", now)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// Verify resolved
	fetched, _ := ticketStore.GetByID(ctx, tkt.ID)
	if fetched.Status != ticket.StatusResolved {
		t.Errorf("ticket.Status = %s, want %s", fetched.Status, ticket.StatusResolved)
	}
	if fetched.Resolution != "user satisfied" {
		t.Errorf("ticket.Resolution = %s, want 'user satisfied'", fetched.Resolution)
	}
	if fetched.ResolvedAt == nil {
		t.Error("ticket.ResolvedAt should be set")
	}
}

func TestTicketWorkflowStore_Resolve_ClosedTicketConflict(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	sessionStore := NewSessionStore(db)
	ticketStore := NewTicketStore(db)
	auditStore := NewAuditStore(db)
	workflowStore := NewTicketWorkflowStore(db, auditStore)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)
	resolvedTime := now.Add(-1 * time.Hour)

	sess, _ := sessionStore.GetOrCreate(ctx, "widget", uniqueID("user"), now)
	tkt := &ticket.Ticket{
		ID:         uniqueID("tick"),
		SessionID:  sess.ID,
		UserID:     "user1",
		Priority:   ticket.PriorityP1,
		Status:     ticket.StatusClosed,
		Resolution: "done",
		ResolvedAt: &resolvedTime,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	ticketStore.Create(ctx, tkt)

	err := workflowStore.Resolve(ctx, tkt.ID, "retry", "admin", "127.0.0.1", now)
	if err == nil {
		t.Fatal("Resolve() on closed ticket should return error")
	}
	if !strings.HasPrefix(err.Error(), "CS_TICKET_4092") {
		t.Fatalf("Resolve() error = %v, want CS_TICKET_4092 prefix", err)
	}
}

func TestTicketWorkflowStore_Close(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	sessionStore := NewSessionStore(db)
	ticketStore := NewTicketStore(db)
	auditStore := NewAuditStore(db)
	workflowStore := NewTicketWorkflowStore(db, auditStore)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	// Create session + resolved ticket
	sess, _ := sessionStore.GetOrCreate(ctx, "widget", uniqueID("user"), now)
	resolvedTime := now.Add(-1 * time.Hour)
	tkt := &ticket.Ticket{
		ID:         uniqueID("tick"),
		SessionID:  sess.ID,
		UserID:     "user1",
		Priority:   ticket.PriorityP1,
		Status:     ticket.StatusResolved,
		Resolution: "fixed",
		ResolvedAt: &resolvedTime,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	ticketStore.Create(ctx, tkt)

	// Close it
	err := workflowStore.Close(ctx, tkt.ID, "user confirmed", "admin", "127.0.0.1", now)
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Verify closed
	fetched, _ := ticketStore.GetByID(ctx, tkt.ID)
	if fetched.Status != ticket.StatusClosed {
		t.Errorf("ticket.Status = %s, want %s", fetched.Status, ticket.StatusClosed)
	}
}

func TestTicketWorkflowStore_Close_NotResolved(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	sessionStore := NewSessionStore(db)
	ticketStore := NewTicketStore(db)
	auditStore := NewAuditStore(db)
	workflowStore := NewTicketWorkflowStore(db, auditStore)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	// Create session + open ticket (not resolved)
	sess, _ := sessionStore.GetOrCreate(ctx, "widget", uniqueID("user"), now)
	tkt := &ticket.Ticket{
		ID:        uniqueID("tick"),
		SessionID: sess.ID,
		UserID:    "user1",
		Priority:  ticket.PriorityP1,
		Status:    ticket.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	ticketStore.Create(ctx, tkt)

	// Try to close — should fail
	err := workflowStore.Close(ctx, tkt.ID, "user confirmed", "admin", "127.0.0.1", now)
	if err == nil {
		t.Fatal("Close() on non-resolved ticket should return error")
	}
	if !strings.HasPrefix(err.Error(), "CS_TICKET_4093") {
		t.Fatalf("Close() error = %v, want CS_TICKET_4093 prefix", err)
	}
}

func TestTicketWorkflowStore_Close_AssignedTicketConflict(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	sessionStore := NewSessionStore(db)
	ticketStore := NewTicketStore(db)
	auditStore := NewAuditStore(db)
	workflowStore := NewTicketWorkflowStore(db, auditStore)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	sess, _ := sessionStore.GetOrCreate(ctx, "widget", uniqueID("user"), now)
	tkt := &ticket.Ticket{
		ID:         uniqueID("tick"),
		SessionID:  sess.ID,
		UserID:     "user1",
		Priority:   ticket.PriorityP1,
		Status:     ticket.StatusAssigned,
		AssignedTo: "agent-001",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	ticketStore.Create(ctx, tkt)

	err := workflowStore.Close(ctx, tkt.ID, "premature close", "admin", "127.0.0.1", now)
	if err == nil {
		t.Fatal("Close() on assigned ticket should return error")
	}
	if !strings.HasPrefix(err.Error(), "CS_TICKET_4093") {
		t.Fatalf("Close() error = %v, want CS_TICKET_4093 prefix", err)
	}
}

func TestTicketWorkflowStore_NilDB(t *testing.T) {
	workflowStore := NewTicketWorkflowStore(nil, nil)
	ctx := context.Background()
	now := time.Now()

	t.Run("ListOpen returns error", func(t *testing.T) {
		_, err := workflowStore.ListOpen(ctx, 10)
		if err == nil {
			t.Error("ListOpen() with nil db should return error")
		}
	})

	t.Run("Assign returns error", func(t *testing.T) {
		err := workflowStore.Assign(ctx, "t1", "agent1", "admin", "127.0.0.1", now)
		if err == nil {
			t.Error("Assign() with nil db should return error")
		}
	})

	t.Run("Resolve returns error", func(t *testing.T) {
		err := workflowStore.Resolve(ctx, "t1", "fixed", "admin", "127.0.0.1", now)
		if err == nil {
			t.Error("Resolve() with nil db should return error")
		}
	})

	t.Run("Close returns error", func(t *testing.T) {
		err := workflowStore.Close(ctx, "t1", "confirmed", "admin", "127.0.0.1", now)
		if err == nil {
			t.Error("Close() with nil db should return error")
		}
	})
}
