package memory

import (
	"context"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/ticket"
)

func TestTicketStore_ListOpen(t *testing.T) {
	store := NewTicketStore()
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	// Create tickets with different statuses
	tickets := []ticket.Ticket{
		{ID: "t1", Status: ticket.StatusOpen, CreatedAt: now, UpdatedAt: now},
		{ID: "t2", Status: ticket.StatusAssigned, CreatedAt: now, UpdatedAt: now},
		{ID: "t3", Status: ticket.StatusResolved, CreatedAt: now, UpdatedAt: now},
		{ID: "t4", Status: ticket.StatusClosed, CreatedAt: now, UpdatedAt: now},
	}
	for i := range tickets {
		store.Create(ctx, &tickets[i])
	}

	// ListOpen should return open + assigned (not resolved/closed)
	open, err := store.ListOpen(ctx, 10)
	if err != nil {
		t.Fatalf("ListOpen() error = %v", err)
	}
	if len(open) != 2 {
		t.Errorf("ListOpen() len = %d, want 2 (open + assigned)", len(open))
	}
}

func TestTicketStore_Assign(t *testing.T) {
	store := NewTicketStore()
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	// Create an open ticket
	store.Create(ctx, &ticket.Ticket{
		ID:     "t1",
		Status: ticket.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	})

	// Assign it
	err := store.Assign(ctx, "t1", "agent1", "admin", "127.0.0.1", now)
	if err != nil {
		t.Fatalf("Assign() error = %v", err)
	}

	// Verify assigned
	tkt, _ := store.GetByID(ctx, "t1")
	if tkt.Status != ticket.StatusAssigned {
		t.Errorf("ticket.Status = %s, want %s", tkt.Status, ticket.StatusAssigned)
	}
	if tkt.AssignedTo != "agent1" {
		t.Errorf("ticket.AssignedTo = %s, want agent1", tkt.AssignedTo)
	}
}

func TestTicketStore_Assign_AlreadyAssigned(t *testing.T) {
	store := NewTicketStore()
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	// Create an already-assigned ticket
	store.Create(ctx, &ticket.Ticket{
		ID:         "t1",
		Status:     ticket.StatusAssigned,
		AssignedTo: "agent1",
		CreatedAt:  now,
		UpdatedAt:  now,
	})

	// Try to assign again — should return error
	err := store.Assign(ctx, "t1", "agent2", "admin", "127.0.0.1", now)
	if err == nil {
		t.Fatal("Assign() on already-assigned ticket should return error")
	}
}

func TestTicketStore_Resolve(t *testing.T) {
	store := NewTicketStore()
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	// Create an assigned ticket
	store.Create(ctx, &ticket.Ticket{
		ID:     "t1",
		Status: ticket.StatusAssigned,
		AssignedTo: "agent1",
		CreatedAt: now,
		UpdatedAt: now,
	})

	// Resolve it
	err := store.Resolve(ctx, "t1", "fixed by agent", "admin", "127.0.0.1", now)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// Verify resolved
	tkt, _ := store.GetByID(ctx, "t1")
	if tkt.Status != ticket.StatusResolved {
		t.Errorf("ticket.Status = %s, want %s", tkt.Status, ticket.StatusResolved)
	}
	if tkt.Resolution != "fixed by agent" {
		t.Errorf("ticket.Resolution = %s, want 'fixed by agent'", tkt.Resolution)
	}
	if tkt.ResolvedAt == nil {
		t.Error("ticket.ResolvedAt should be set")
	}
}

func TestTicketStore_Close(t *testing.T) {
	store := NewTicketStore()
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	// Create a resolved ticket
	resolvedTime := now.Add(-1 * time.Hour)
	store.Create(ctx, &ticket.Ticket{
		ID:         "t1",
		Status:     ticket.StatusResolved,
		Resolution: "fixed",
		ResolvedAt: &resolvedTime,
		CreatedAt:  now,
		UpdatedAt:  now,
	})

	// Close it
	err := store.Close(ctx, "t1", "user confirmed", "admin", "127.0.0.1", now)
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Verify closed
	tkt, _ := store.GetByID(ctx, "t1")
	if tkt.Status != ticket.StatusClosed {
		t.Errorf("ticket.Status = %s, want %s", tkt.Status, ticket.StatusClosed)
	}
}

func TestTicketStore_Close_NotResolved(t *testing.T) {
	store := NewTicketStore()
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	// Create an open ticket (not resolved)
	store.Create(ctx, &ticket.Ticket{
		ID:     "t1",
		Status: ticket.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	})

	// Try to close — should return error
	err := store.Close(ctx, "t1", "user confirmed", "admin", "127.0.0.1", now)
	if err == nil {
		t.Fatal("Close() on non-resolved ticket should return error")
	}
}
