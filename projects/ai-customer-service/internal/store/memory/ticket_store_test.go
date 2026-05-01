package memory

import (
	"context"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/ticket"
)

func TestTicketStore_Create(t *testing.T) {
	store := NewTicketStore()
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	tests := []struct {
		name    string
		ticket  ticket.Ticket
		wantLen int
	}{
		{
			name: "create single ticket",
			ticket: ticket.Ticket{
				ID:     "t1",
				Status: ticket.StatusOpen,
			},
			wantLen: 1,
		},
		{
			name: "create multiple tickets",
			ticket: ticket.Ticket{
				ID:     "t2",
				Status: ticket.StatusOpen,
			},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.ticket.CreatedAt = now
			tt.ticket.UpdatedAt = now
			err := store.Create(ctx, &tt.ticket)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if got := len(store.List()); got != tt.wantLen {
				t.Errorf("List() len = %d, want %d", got, tt.wantLen)
			}
		})
	}
}

func TestTicketStore_GetByID(t *testing.T) {
	store := NewTicketStore()
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	// Empty store
	t.Run("empty store returns nil", func(t *testing.T) {
		got, err := store.GetByID(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got != nil {
			t.Errorf("GetByID() = %v, want nil", got)
		}
	})

	// Add a ticket
	ticket := ticket.Ticket{ID: "t1", Status: ticket.StatusOpen, CreatedAt: now, UpdatedAt: now}
	store.Create(ctx, &ticket)

	t.Run("found existing ticket", func(t *testing.T) {
		got, err := store.GetByID(ctx, "t1")
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got == nil || got.ID != "t1" {
			t.Errorf("GetByID() = %v, want ticket with ID t1", got)
		}
	})

	t.Run("not found returns nil", func(t *testing.T) {
		got, err := store.GetByID(ctx, "doesnotexist")
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got != nil {
			t.Errorf("GetByID() = %v, want nil", got)
		}
	})
}

func TestTicketStore_List(t *testing.T) {
	store := NewTicketStore()
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	t.Run("empty store", func(t *testing.T) {
		got := store.List()
		if len(got) != 0 {
			t.Errorf("List() len = %d, want 0", len(got))
		}
	})

	t.Run("multiple tickets", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			store.Create(ctx, &ticket.Ticket{ID: "t" + string(rune('1'+i)), Status: ticket.StatusOpen, CreatedAt: now, UpdatedAt: now})
		}
		got := store.List()
		if len(got) != 3 {
			t.Errorf("List() len = %d, want 3", len(got))
		}
	})

	t.Run("list returns copy", func(t *testing.T) {
		got := store.List()
		got[0].ID = "mutated"
		if store.List()[0].ID == "mutated" {
			t.Error("List() should return a copy, not the same slice")
		}
	})
}

func TestTicketStore_ListAll(t *testing.T) {
	store := NewTicketStore()
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	t.Run("empty store", func(t *testing.T) {
		got, err := store.ListAll(ctx)
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("ListAll() len = %d, want 0", len(got))
		}
	})

	t.Run("returns all tickets", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			store.Create(ctx, &ticket.Ticket{ID: "listall" + string(rune('a'+i)), Status: ticket.StatusOpen, CreatedAt: now, UpdatedAt: now})
		}
		got, err := store.ListAll(ctx)
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		if len(got) < 2 {
			t.Errorf("ListAll() len = %d, want >= 2", len(got))
		}
	})
}

func TestTicketStore_GetStats(t *testing.T) {
	store := NewTicketStore()
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	t.Run("empty store", func(t *testing.T) {
		stats, err := store.GetStats(ctx)
		if err != nil {
			t.Fatalf("GetStats() error = %v", err)
		}
		if stats.Total != 0 {
			t.Errorf("GetStats().Total = %d, want 0", stats.Total)
		}
	})

	t.Run("aggregates correctly", func(t *testing.T) {
		resolvedTime := now.Add(-1 * time.Hour)
		tickets := []ticket.Ticket{
			{ID: "s1", Status: ticket.StatusOpen, Priority: ticket.PriorityP0, ContextSnapshot: map[string]any{"channel": "wechat"}, CreatedAt: now, UpdatedAt: now},
			{ID: "s2", Status: ticket.StatusResolved, Priority: ticket.PriorityP1, ResolvedAt: &resolvedTime, CreatedAt: now.Add(-1 * time.Hour), UpdatedAt: now},
			{ID: "s3", Status: ticket.StatusClosed, Priority: ticket.PriorityP2, HandoffReason: "escalation", CreatedAt: now, UpdatedAt: now},
			{ID: "s4", Status: ticket.StatusOpen, Priority: ticket.PriorityP0, ContextSnapshot: map[string]any{"channel": "wechat"}, CreatedAt: now, UpdatedAt: now},
		}
		for i := range tickets {
			store.Create(ctx, &tickets[i])
		}

		stats, err := store.GetStats(ctx)
		if err != nil {
			t.Fatalf("GetStats() error = %v", err)
		}
		if stats.Total != 4 {
			t.Errorf("GetStats().Total = %d, want 4", stats.Total)
		}
		if stats.Open != 2 {
			t.Errorf("GetStats().Open = %d, want 2", stats.Open)
		}
		if stats.Resolved != 1 {
			t.Errorf("GetStats().Resolved = %d, want 1", stats.Resolved)
		}
		if stats.Closed != 1 {
			t.Errorf("GetStats().Closed = %d, want 1", stats.Closed)
		}
		if stats.HandoffCount != 1 {
			t.Errorf("GetStats().HandoffCount = %d, want 1", stats.HandoffCount)
		}
		if stats.ByChannel["wechat"] != 2 {
			t.Errorf("GetStats().ByChannel[wechat] = %d, want 2", stats.ByChannel["wechat"])
		}
		if stats.ByPriority[string(ticket.PriorityP0)] != 2 {
			t.Errorf("GetStats().ByPriority[P0] = %d, want 2", stats.ByPriority[string(ticket.PriorityP0)])
		}
	})
}
