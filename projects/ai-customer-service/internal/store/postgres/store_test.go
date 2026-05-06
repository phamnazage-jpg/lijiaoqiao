package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
	"github.com/bridge/ai-customer-service/internal/domain/session"
	"github.com/bridge/ai-customer-service/internal/domain/ticket"
)

func getDSN() string {
	return "host=localhost port=5434 user=ai_cs password=ai_cs_secret dbname=ai_customer_service sslmode=disable"
}

func uniqueID(prefix string) string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	uuid := hex.EncodeToString(b)
	return uuid[:8] + "-" + uuid[8:12] + "-" + uuid[12:16] + "-" + uuid[16:20] + "-" + uuid[20:]
}

func openDBForTest(t *testing.T) *sql.DB {
	dsn := getDSN()
	if dsn == "" {
		t.Skip("AI_CS_POSTGRES_DSN not set")
	}
	db, err := Open(Config{
		DSN:             dsn,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Second * 30,
	})
	if err != nil {
		t.Fatalf("failed to open DB: %v", err)
	}
	if err := RunMigrations(db, filepath.Join("..", "..", "..", "db", "migration")); err != nil {
		_ = db.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}
	return db
}

// --- TicketStore tests ---

func TestTicketStore_CreateAndGet(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	sessionStore := NewSessionStore(db)
	ticketStore := NewTicketStore(db)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	// Create session first (FK constraint)
	sess, err := sessionStore.GetOrCreate(ctx, "widget", uniqueID("user"), now)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	tkt := &ticket.Ticket{
		ID:              uniqueID("tick"),
		SessionID:       sess.ID,
		UserID:          "user-001",
		Priority:        ticket.PriorityP1,
		Status:          ticket.StatusOpen,
		HandoffReason:   "Test handoff",
		AssignedTo:      "agent-001",
		ContextSnapshot: map[string]any{"key": "value"},
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := ticketStore.Create(ctx, tkt); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	fetched, err := ticketStore.GetByID(ctx, tkt.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if fetched.ID != tkt.ID {
		t.Errorf("expected ID %s, got %s", tkt.ID, fetched.ID)
	}
	if fetched.SessionID != tkt.SessionID {
		t.Errorf("expected SessionID %s, got %s", tkt.SessionID, fetched.SessionID)
	}
	if fetched.Priority != ticket.PriorityP1 {
		t.Errorf("expected Priority P1, got %s", fetched.Priority)
	}
	if fetched.Status != ticket.StatusOpen {
		t.Errorf("expected Status open, got %s", fetched.Status)
	}
}

func TestTicketStore_GetStats(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	store := NewTicketStore(db)
	ctx := context.Background()

	stats, err := store.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.Total < 0 {
		t.Errorf("expected non-negative Total, got %d", stats.Total)
	}
	if stats.ByChannel == nil {
		t.Error("expected non-nil ByChannel")
	}
	if stats.ByPriority == nil {
		t.Error("expected non-nil ByPriority")
	}
}

func TestTicketStore_Create_NilTicket(t *testing.T) {
	store := NewTicketStore(nil)
	err := store.Create(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil ticket")
	}
}

func TestTicketStore_Create_NilDB(t *testing.T) {
	store := NewTicketStore(nil)
	err := store.Create(context.Background(), &ticket.Ticket{})
	if err == nil {
		t.Error("expected error for nil db")
	}
}

func TestTicketStore_GetByID_NilDB(t *testing.T) {
	store := NewTicketStore(nil)
	_, err := store.GetByID(context.Background(), "any-id")
	if err == nil {
		t.Error("expected error for nil db")
	}
}

func TestTicketStore_GetStats_NilDB(t *testing.T) {
	store := NewTicketStore(nil)
	_, err := store.GetStats(context.Background())
	if err == nil {
		t.Error("expected error for nil db")
	}
}

// --- SessionStore tests ---

func TestSessionStore_GetOrCreate(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	store := NewSessionStore(db)
	ctx := context.Background()
	now := time.Now()

	openID := uniqueID("sess")

	// First call creates
	sess1, err := store.GetOrCreate(ctx, "widget", openID, now)
	if err != nil {
		t.Fatalf("GetOrCreate (create) failed: %v", err)
	}
	if sess1.Channel != "widget" {
		t.Errorf("expected channel widget, got %s", sess1.Channel)
	}
	if sess1.OpenID != openID {
		t.Errorf("expected openID %s, got %s", openID, sess1.OpenID)
	}

	// Second call returns existing
	sess2, err := store.GetOrCreate(ctx, "widget", openID, now)
	if err != nil {
		t.Fatalf("GetOrCreate (get) failed: %v", err)
	}
	if sess2.ID != sess1.ID {
		t.Errorf("expected same ID on second call, got %s vs %s", sess2.ID, sess1.ID)
	}
}

func TestSessionStore_GetOrCreate_NilDB(t *testing.T) {
	store := NewSessionStore(nil)
	_, err := store.GetOrCreate(context.Background(), "widget", "any", time.Now())
	if err == nil {
		t.Error("expected error for nil db")
	}
}

func TestSessionStore_GetByID(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	store := NewSessionStore(db)
	ctx := context.Background()
	now := time.Now()
	openID := uniqueID("sess")

	created, err := store.GetOrCreate(ctx, "widget", openID, now)
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}

	fetched, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, fetched.ID)
	}
}

func TestSessionStore_GetByID_NilDB(t *testing.T) {
	store := NewSessionStore(nil)
	_, err := store.GetByID(context.Background(), "any-id")
	if err == nil {
		t.Error("expected error for nil db")
	}
}

func TestSessionStore_Save(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	store := NewSessionStore(db)
	ctx := context.Background()
	now := time.Now()
	openID := uniqueID("sess")

	sess, err := store.GetOrCreate(ctx, "widget", openID, now)
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}

	sess.Status = session.StatusProcessing
	sess.TurnCount = 5
	if err := store.Save(ctx, sess); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	fetched, err := store.GetByID(ctx, sess.ID)
	if err != nil {
		t.Fatalf("GetByID after Save failed: %v", err)
	}
	if fetched.Status != session.StatusProcessing {
		t.Errorf("expected status processing, got %s", fetched.Status)
	}
	if fetched.TurnCount != 5 {
		t.Errorf("expected turncount 5, got %d", fetched.TurnCount)
	}
}

func TestSessionStore_Save_NilDB(t *testing.T) {
	store := NewSessionStore(nil)
	err := store.Save(context.Background(), &session.Session{})
	if err == nil {
		t.Error("expected error for nil db")
	}
}

// --- AuditStore tests ---

func TestAuditStore_Add(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	store := NewAuditStore(db)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	event := audit.Event{
		ID:          uniqueID("audit"),
		SessionID:   uniqueID("sess"),
		TicketID:    "",
		Type:        "session",
		Action:      "message",
		Channel:     "widget",
		OpenID:      "ou_test",
		ActorID:     "agent-001",
		SourceIP:    "10.0.0.1",
		Payload:     map[string]any{"content": "hello world"},
		BeforeState: map[string]any{"status": "idle"},
		AfterState:  map[string]any{"status": "processing"},
		CreatedAt:   now,
	}

	if err := store.Add(ctx, event); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
}

func TestAuditStore_Add_NilDB(t *testing.T) {
	store := NewAuditStore(nil)
	err := store.Add(context.Background(), audit.Event{Type: "test"})
	if err == nil {
		t.Error("expected error for nil db")
	}
}

func TestAuditStore_Add_TicketScoped(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	store := NewAuditStore(db)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	event := audit.Event{
		ID:          uniqueID("audit"),
		TicketID:    uniqueID("tick"),
		Type:        "ticket",
		Action:      "resolve",
		OpenID:      "ou_test2",
		ActorID:     "agent-002",
		BeforeState: map[string]any{"status": "open"},
		AfterState:  map[string]any{"status": "resolved"},
		CreatedAt:   now,
	}

	if err := store.Add(ctx, event); err != nil {
		t.Fatalf("Add ticket-scoped event failed: %v", err)
	}
}

func TestAuditStore_Add_SystemActor(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	store := NewAuditStore(db)
	ctx := context.Background()

	// Event with no ActorID and no OpenID -> defaults to "system"
	event := audit.Event{
		ID:        uniqueID("audit"),
		SessionID: uniqueID("sess"),
		Type:      "session",
		Action:    "create",
		CreatedAt: time.Now().Truncate(time.Second),
	}

	if err := store.Add(ctx, event); err != nil {
		t.Fatalf("Add system actor event failed: %v", err)
	}
}

func TestAuditStore_Add_EmptyAction(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	store := NewAuditStore(db)
	ctx := context.Background()

	// Empty action should default to "update"
	event := audit.Event{
		ID:        uniqueID("audit"),
		SessionID: uniqueID("sess"),
		Type:      "session",
		CreatedAt: time.Now().Truncate(time.Second),
	}

	if err := store.Add(ctx, event); err != nil {
		t.Fatalf("Add with empty action failed: %v", err)
	}
}
