package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/session"
)

func TestSessionStore_GetOrCreate(t *testing.T) {
	store := NewSessionStore()
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	t.Run("creates new session", func(t *testing.T) {
		sess, err := store.GetOrCreate(ctx, "wechat", "user1", now)
		if err != nil {
			t.Fatalf("GetOrCreate() error = %v", err)
		}
		if sess == nil {
			t.Fatal("GetOrCreate() returned nil session")
		}
		if sess.ID != "wechat:user1" {
			t.Errorf("GetOrCreate().ID = %q, want %q", sess.ID, "wechat:user1")
		}
		if sess.Status != session.StatusIdle {
			t.Errorf("GetOrCreate().Status = %v, want %v", sess.Status, session.StatusIdle)
		}
	})

	t.Run("returns existing session", func(t *testing.T) {
		sess, err := store.GetOrCreate(ctx, "wechat", "user1", now.Add(time.Minute))
		if err != nil {
			t.Fatalf("GetOrCreate() error = %v", err)
		}
		if sess == nil {
			t.Fatal("GetOrCreate() returned nil session")
		}
		if sess.ID != "wechat:user1" {
			t.Errorf("GetOrCreate().ID = %q, want %q", sess.ID, "wechat:user1")
		}
		// Should use original creation time, not new time
		if !sess.LastMessageAt.Equal(now) {
			t.Errorf("GetOrCreate().LastMessageAt = %v, want %v", sess.LastMessageAt, now)
		}
	})

	t.Run("different channel creates different session", func(t *testing.T) {
		sess, err := store.GetOrCreate(ctx, "feishu", "user1", now)
		if err != nil {
			t.Fatalf("GetOrCreate() error = %v", err)
		}
		if sess.ID != "feishu:user1" {
			t.Errorf("GetOrCreate().ID = %q, want %q", sess.ID, "feishu:user1")
		}
	})

	t.Run("empty store", func(t *testing.T) {
		// New empty store - no sessions exist
		emptyStore := NewSessionStore()
		sess, err := emptyStore.GetOrCreate(ctx, "wechat", "ghost", now)
		if err != nil {
			t.Fatalf("GetOrCreate() error = %v", err)
		}
		if sess == nil {
			t.Fatal("GetOrCreate() returned nil session")
		}
		if sess.ID != "wechat:ghost" {
			t.Errorf("GetOrCreate().ID = %q, want %q", sess.ID, "wechat:ghost")
		}
	})
}

func TestSessionStore_Save(t *testing.T) {
	store := NewSessionStore()
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	t.Run("save updates existing session", func(t *testing.T) {
		sess, _ := store.GetOrCreate(ctx, "wechat", "saveuser", now)
		sess.TurnCount = 5
		sess.Status = session.StatusProcessing
		err := store.Save(ctx, sess)
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Retrieve and verify
		retrieved, _ := store.GetByID(ctx, "wechat:saveuser")
		if retrieved.TurnCount != 5 {
			t.Errorf("GetByID().TurnCount = %d, want 5", retrieved.TurnCount)
		}
		if retrieved.Status != session.StatusProcessing {
			t.Errorf("GetByID().Status = %v, want %v", retrieved.Status, session.StatusProcessing)
		}
	})

	t.Run("save preserves context slice", func(t *testing.T) {
		sess, _ := store.GetOrCreate(ctx, "wechat", "ctxuser", now)
		sess.Context = append(sess.Context, session.MessageContext{
			Direction: "in",
			Content:   "hello",
			Timestamp: now,
		})
		err := store.Save(ctx, sess)
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		retrieved, _ := store.GetByID(ctx, "wechat:ctxuser")
		if len(retrieved.Context) != 1 {
			t.Errorf("GetByID().Context len = %d, want 1", len(retrieved.Context))
		}
	})

	t.Run("empty store save", func(t *testing.T) {
		emptyStore := NewSessionStore()
		sess := &session.Session{ID: "brandnew", Channel: "test", Status: session.StatusIdle}
		err := emptyStore.Save(ctx, sess)
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		retrieved, err := emptyStore.GetByID(ctx, "brandnew")
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if retrieved == nil {
			t.Fatal("GetByID() returned nil after save")
		}
	})
}

func TestSessionStore_GetByID(t *testing.T) {
	store := NewSessionStore()
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	store.GetOrCreate(ctx, "wechat", "getuser", now)

	tests := []struct {
		name    string
		id      string
		wantErr error
		wantNil bool
	}{
		{
			name:    "existing session",
			id:      "wechat:getuser",
			wantErr: nil,
			wantNil: false,
		},
		{
			name:    "nonexistent session",
			id:      "not:found",
			wantErr: errors.New("session not found: not:found"),
			wantNil: true,
		},
		{
			name:    "empty store",
			id:      "empty:id",
			wantErr: errors.New("session not found: empty:id"),
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fresh empty store for "empty store" case
			if tt.name == "empty store" {
				store = NewSessionStore()
			}
			got, err := store.GetByID(ctx, tt.id)
			if (err == nil) != (tt.wantErr == nil) {
				t.Errorf("GetByID() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantNil && got != nil {
				t.Errorf("GetByID() = %v, want nil", got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("GetByID() = nil, want non-nil")
			}
		})
	}
}

func TestSessionStore_List(t *testing.T) {
	store := NewSessionStore()
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	t.Run("empty store returns empty slice", func(t *testing.T) {
		got := store.List()
		if len(got) != 0 {
			t.Errorf("List() len = %d, want 0", len(got))
		}
	})

	t.Run("returns all sessions", func(t *testing.T) {
		store.GetOrCreate(ctx, "wechat", "listuser1", now)
		store.GetOrCreate(ctx, "feishu", "listuser2", now)
		store.GetOrCreate(ctx, "wechat", "listuser3", now)

		got := store.List()
		if len(got) != 3 {
			t.Errorf("List() len = %d, want 3", len(got))
		}
	})

	t.Run("list returns copy not reference", func(t *testing.T) {
		store.GetOrCreate(ctx, "wechat", "copyuser", now)
		got := store.List()
		if len(got) > 0 {
			got[0].TurnCount = 999
			if store.List()[0].TurnCount == 999 {
				t.Error("List() should return copies, not references")
			}
		}
	})

	t.Run("sessions are distinct", func(t *testing.T) {
		got := store.List()
		ids := make(map[string]bool)
		for _, s := range got {
			if ids[s.ID] {
				t.Errorf("List() contains duplicate ID %q", s.ID)
			}
			ids[s.ID] = true
		}
		if len(ids) != len(store.List()) {
			t.Errorf("List() returned inconsistent lengths")
		}
	})
}
