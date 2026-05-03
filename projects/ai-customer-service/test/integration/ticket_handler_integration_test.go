package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/app"
	"github.com/bridge/ai-customer-service/internal/config"
	"github.com/bridge/ai-customer-service/internal/domain/audit"
	"github.com/bridge/ai-customer-service/internal/domain/session"
	"github.com/bridge/ai-customer-service/internal/domain/ticket"
	"github.com/bridge/ai-customer-service/internal/http/handlers"
	"github.com/bridge/ai-customer-service/internal/platform/logging"
	"github.com/bridge/ai-customer-service/internal/store/memory"
)

// --------------------------------------------------
// Mock infrastructure
// --------------------------------------------------

type ticketIntgAuditRecorder struct {
	events []audit.Event
}

func (r *ticketIntgAuditRecorder) Add(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

func (r *ticketIntgAuditRecorder) eventsOfType(action string) []audit.Event {
	var out []audit.Event
	for _, e := range r.events {
		if e.Action == action {
			out = append(out, e)
		}
	}
	return out
}

// mockTicketSvcForHandler wraps memory.TicketStore + provides TicketService interface.
type mockTicketSvcForHandler struct {
	store *memory.TicketStore
	audit *ticketIntgAuditRecorder
}

func newMockTicketSvcForHandler(auditRecorder *ticketIntgAuditRecorder) *mockTicketSvcForHandler {
	return &mockTicketSvcForHandler{
		store: memory.NewTicketStore(),
		audit: auditRecorder,
	}
}

func (m *mockTicketSvcForHandler) ListOpen(ctx context.Context, limit int) ([]ticket.Ticket, error) {
	return m.store.ListOpen(ctx, limit)
}

func (m *mockTicketSvcForHandler) GetByID(ctx context.Context, id string) (*ticket.Ticket, error) {
	return m.store.GetByID(ctx, id)
}

func (m *mockTicketSvcForHandler) Assign(ctx context.Context, ticketID, agentID, actorID, sourceIP string, now time.Time) error {
	if err := m.store.Assign(ctx, ticketID, agentID, actorID, sourceIP, now); err != nil {
		return err
	}
	m.audit.Add(ctx, audit.Event{
		ID:        "audit-assign-1",
		Type:      "ticket_state_changed",
		Action:    "assign",
		TicketID:  ticketID,
		ActorID:   actorID,
		SourceIP:  sourceIP,
		AfterState: map[string]any{"assigned_to": agentID, "status": ticket.StatusAssigned},
		CreatedAt: now,
	})
	return nil
}

func (m *mockTicketSvcForHandler) Resolve(ctx context.Context, ticketID, resolution, actorID, sourceIP string, now time.Time) error {
	if err := m.store.Resolve(ctx, ticketID, resolution, actorID, sourceIP, now); err != nil {
		return err
	}
	m.audit.Add(ctx, audit.Event{
		ID:        "audit-resolve-1",
		Type:      "ticket_state_changed",
		Action:    "resolve",
		TicketID:  ticketID,
		ActorID:   actorID,
		SourceIP:  sourceIP,
		AfterState: map[string]any{"resolution": resolution, "status": ticket.StatusResolved},
		CreatedAt: now,
	})
	return nil
}

func (m *mockTicketSvcForHandler) Close(ctx context.Context, ticketID, resolution, actorID, sourceIP string, now time.Time) error {
	if err := m.store.Close(ctx, ticketID, resolution, actorID, sourceIP, now); err != nil {
		return err
	}
	m.audit.Add(ctx, audit.Event{
		ID:        "audit-close-1",
		Type:      "ticket_state_changed",
		Action:    "close",
		TicketID:  ticketID,
		ActorID:   actorID,
		SourceIP:  sourceIP,
		AfterState: map[string]any{"resolution": resolution, "status": ticket.StatusClosed},
		CreatedAt: now,
	})
	return nil
}

// mockHandoffSessions satisfies handlers.SessionGetter
type mockHandoffSessions struct {
	store *memory.SessionStore
}

func (m *mockHandoffSessions) GetByID(ctx context.Context, id string) (*session.Session, error) {
	return m.store.GetByID(ctx, id)
}

// mockHandoffTickets satisfies handlers.TicketCreator
type mockHandoffTickets struct {
	store *memory.TicketStore
}

func (m *mockHandoffTickets) Create(ctx context.Context, t *ticket.Ticket) error {
	return m.store.Create(ctx, t)
}

// --------------------------------------------------
// Tests: POST /api/v1/customer-service/tickets (via session handoff)
// and GET /api/v1/customer-service/tickets (list)
// --------------------------------------------------

// TestTicketCreateAndList_CreateThenFind verifies that a ticket created via
// session handoff can be retrieved via GET /tickets/{id}.
func TestTicketCreateAndList_CreateThenFind(t *testing.T) {
	auditRecorder := &ticketIntgAuditRecorder{}
	svc := newMockTicketSvcForHandler(auditRecorder)
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()

	// Create a session first (required for handoff)
	sessions := memory.NewSessionStore()
	_, _ = sessions.GetOrCreate(ctx, "widget", "u_list_test", now)
	sess, _ := sessions.GetOrCreate(ctx, "widget", "u_list_test", now)
	sess.Status = session.StatusIdle
	_ = sessions.Save(ctx, sess)

	// Use the session handler to create a ticket (simulates POST /tickets behavior)
	// This uses the REAL handlers.NewSessionHandler
	sessionAudit := &ticketIntgAuditRecorder{}
	sessionSvc := &mockHandoffSessions{store: sessions}
	ticketSvc := &mockHandoffTickets{store: svc.store}
	sessionHdlr := handlers.NewSessionHandler(sessionSvc, ticketSvc, sessionAudit)

	handoffBody := handlers.HandoffRequest{Reason: "test ticket creation"}
	handoffBodyBytes, _ := json.Marshal(handoffBody)
	sessionReq := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/widget:u_list_test/handoff", bytes.NewReader(handoffBodyBytes))
	sessionReq.Header.Set("Content-Type", "application/json")
	sessionResp := httptest.NewRecorder()
	sessionHdlr.Handoff(sessionResp, sessionReq)

	if sessionResp.Code != http.StatusOK {
		t.Fatalf("handoff failed: status=%d body=%s", sessionResp.Code, sessionResp.Body.String())
	}

	var handoffResp map[string]any
	if err := json.Unmarshal(sessionResp.Body.Bytes(), &handoffResp); err != nil {
		t.Fatalf("decode handoff response error = %v", err)
	}
	ticketID, ok := handoffResp["ticket_id"].(string)
	if !ok || ticketID == "" {
		t.Fatalf("ticket_id missing from handoff response: %v", handoffResp)
	}

	// Now verify the ticket can be found via GET /tickets/{id}
	ticketHandler := handlers.NewTicketHandler(svc, auditRecorder)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/customer-service/tickets/"+ticketID, nil)
	getResp := httptest.NewRecorder()
	ticketHandler.Get(getResp, getReq)

	if getResp.Code != http.StatusOK {
		t.Fatalf("GET ticket status = %d, want 200", getResp.Code)
	}

	var ticketResp map[string]any
	if err := json.Unmarshal(getResp.Body.Bytes(), &ticketResp); err != nil {
		t.Fatalf("decode ticket response error = %v", err)
	}
	tkt := ticketResp["ticket"].(map[string]any)
	if tkt["id"] != ticketID {
		t.Fatalf("ticket id = %v, want %s", tkt["id"], ticketID)
	}
	if tkt["status"] != "open" {
		t.Fatalf("ticket status = %v, want open", tkt["status"])
	}
}

// TestTicketList_ReturnsArray verifies GET /tickets returns a JSON array
// under the "items" key.
func TestTicketList_ReturnsArray(t *testing.T) {
	auditRecorder := &ticketIntgAuditRecorder{}
	svc := newMockTicketSvcForHandler(auditRecorder)
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()

	// Seed two tickets
	for i := 1; i <= 2; i++ {
		tkt := &ticket.Ticket{
			ID:            "list-test-tkt-" + string(rune('0'+i)),
			SessionID:     "session-list-" + string(rune('0'+i)),
			UserID:        "user-list-" + string(rune('0'+i)),
			Priority:      ticket.PriorityP1,
			Status:        ticket.StatusOpen,
			HandoffReason: "test list",
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		_ = svc.store.Create(ctx, tkt)
	}

	h := handlers.NewTicketHandler(svc, auditRecorder)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customer-service/tickets", nil)
	resp := httptest.NewRecorder()
	h.List(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error = %v", err)
	}

	items, ok := payload["items"].([]any)
	if !ok {
		t.Fatalf("items field missing or not an array; got %T: %v", payload["items"], payload["items"])
	}
	if len(items) < 2 {
		t.Fatalf("items length = %d, want at least 2", len(items))
	}
}

// TestTicketList_PaginationParams verifies that the list endpoint handles
// pagination query parameters without error. Tests via the full HTTP router.
func TestTicketList_PaginationParams(t *testing.T) {
	cfg := &config.Config{}
	cfg.HTTP.Addr = ":0"
	cfg.HTTP.ReadHeaderTimeout = 5
	cfg.HTTP.ReadTimeout = 10
	cfg.HTTP.WriteTimeout = 15
	cfg.HTTP.IdleTimeout = 60
	cfg.HTTP.MaxHeaderBytes = 1 << 20
	cfg.HTTP.MaxBodyBytes = 1 << 20
	cfg.Runtime.Env = "test"
	application, err := app.New(cfg, logging.New())
	if err != nil {
		t.Fatalf("app.New() error = %v", err)
	}
	server := httptest.NewServer(application.Server.Handler)
	defer server.Close()

	// Create tickets via webhook first
	for i := 0; i < 5; i++ {
		payload := map[string]any{
			"message_id": "m-page-" + string(rune('a'+i)),
			"channel":    "widget",
			"open_id":    "u-page-" + string(rune('a'+i)),
			"content":    "转人工",
		}
		body, _ := json.Marshal(payload)
		_, _ = http.Post(server.URL+"/api/v1/customer-service/webhook", "application/json", bytes.NewReader(body))
	}

	tests := []struct {
		name  string
		query string
	}{
		{"no params", "/api/v1/customer-service/tickets"},
		{"limit=2", "/api/v1/customer-service/tickets?limit=2"},
		{"limit=10", "/api/v1/customer-service/tickets?limit=10"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(server.URL + tc.query)
			if err != nil {
				t.Fatalf("GET error = %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200 for query %q", resp.StatusCode, tc.query)
			}

			var payload map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				t.Fatalf("decode error = %v", err)
			}

			items, ok := payload["items"].([]any)
			if !ok {
				t.Fatalf("items not an array for query %q", tc.query)
			}
			if len(items) == 0 {
				t.Fatalf("items empty for query %q, want non-empty", tc.query)
			}
		})
	}
}

// TestTicketList_EmptyStore returns empty array (not null or error).
func TestTicketList_EmptyStore(t *testing.T) {
	auditRecorder := &ticketIntgAuditRecorder{}
	svc := newMockTicketSvcForHandler(auditRecorder)

	h := handlers.NewTicketHandler(svc, auditRecorder)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customer-service/tickets", nil)
	resp := httptest.NewRecorder()
	h.List(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error = %v", err)
	}

	items, ok := payload["items"].([]any)
	if !ok {
		t.Fatalf("items missing or not array")
	}
	if items == nil {
		t.Fatalf("items should be empty array, not null")
	}
}
