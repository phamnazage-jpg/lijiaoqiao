package memory

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
)

func TestAuditStore_Add(t *testing.T) {
	store := NewAuditStore()
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	t.Run("add single event", func(t *testing.T) {
		event := audit.Event{
			ID:        "e1",
			Type:      "ticket.created",
			SessionID: "sess1",
			CreatedAt: now,
		}
		err := store.Add(ctx, event)
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		got := store.List()
		if len(got) != 1 {
			t.Errorf("List() len = %d, want 1", len(got))
		}
	})

	t.Run("add multiple events", func(t *testing.T) {
		for i := 2; i <= 3; i++ {
			err := store.Add(ctx, audit.Event{
				ID:        "e" + string(rune('0'+i)),
				Type:      "ticket.updated",
				CreatedAt: now,
			})
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
		}
		got := store.List()
		if len(got) != 3 {
			t.Errorf("List() len = %d, want 3", len(got))
		}
	})

	t.Run("zero time is set to now", func(t *testing.T) {
		store2 := NewAuditStore()
		before := time.Now().Add(-time.Second)
		err := store2.Add(ctx, audit.Event{
			ID:   "zerotime",
			Type: "test",
		})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		after := time.Now().Add(time.Second)
		got := store2.List()
		if len(got) != 1 {
			t.Fatalf("List() len = %d, want 1", len(got))
		}
		if got[0].CreatedAt.Before(before) || got[0].CreatedAt.After(after) {
			t.Errorf("Add() zero CreatedAt not set to now: got %v, want between %v and %v", got[0].CreatedAt, before, after)
		}
	})

	t.Run("empty store", func(t *testing.T) {
		emptyStore := NewAuditStore()
		err := emptyStore.Add(ctx, audit.Event{ID: "first", Type: "init"})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if len(emptyStore.List()) != 1 {
			t.Errorf("List() len = %d, want 1", len(emptyStore.List()))
		}
	})
}

func TestAuditStore_List(t *testing.T) {
	store := NewAuditStore()
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	t.Run("empty store returns empty slice", func(t *testing.T) {
		got := store.List()
		if len(got) != 0 {
			t.Errorf("List() len = %d, want 0", len(got))
		}
	})

	t.Run("returns all events in order", func(t *testing.T) {
		events := []audit.Event{
			{ID: "l1", Type: "type1", CreatedAt: now.Add(-2 * time.Hour)},
			{ID: "l2", Type: "type2", CreatedAt: now.Add(-1 * time.Hour)},
			{ID: "l3", Type: "type3", CreatedAt: now},
		}
		for _, e := range events {
			store.Add(ctx, e)
		}

		got := store.List()
		if len(got) != 3 {
			t.Errorf("List() len = %d, want 3", len(got))
		}
		// Verify order is preserved
		ids := []string{got[0].ID, got[1].ID, got[2].ID}
		if !slices.Equal(ids, []string{"l1", "l2", "l3"}) {
			t.Errorf("List() order = %v, want [l1, l2, l3]", ids)
		}
	})

	t.Run("returns copy not reference", func(t *testing.T) {
		store2 := NewAuditStore()
		store2.Add(ctx, audit.Event{ID: "orig", Type: "test", CreatedAt: now})
		got := store2.List()
		if len(got) > 0 {
			got[0].ID = "mutated"
			if store2.List()[0].ID == "mutated" {
				t.Error("List() should return copies, not references")
			}
		}
	})

	t.Run("filters by session", func(t *testing.T) {
		store3 := NewAuditStore()
		store3.Add(ctx, audit.Event{ID: "sa1", SessionID: "sessA", Type: "a", CreatedAt: now})
		store3.Add(ctx, audit.Event{ID: "sa2", SessionID: "sessB", Type: "b", CreatedAt: now})
		store3.Add(ctx, audit.Event{ID: "sa3", SessionID: "sessA", Type: "c", CreatedAt: now})

		got := store3.List()
		sessionA := 0
		for _, e := range got {
			if e.SessionID == "sessA" {
				sessionA++
			}
		}
		if sessionA != 2 {
			t.Errorf("List() sessA count = %d, want 2", sessionA)
		}
	})
}
