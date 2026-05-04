package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
	"github.com/bridge/ai-customer-service/internal/domain/ticket"
	"github.com/bridge/ai-customer-service/internal/store/memory"
)

// ticketAuditRecorder implements AuditRecorder for testing.
type ticketAuditRecorder struct {
	events []audit.Event
	mu     sync.Mutex
}

func (r *ticketAuditRecorder) Add(_ context.Context, event audit.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}

func (r *ticketAuditRecorder) eventsOfType(action string) []audit.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []audit.Event
	for _, e := range r.events {
		if e.Action == action {
			out = append(out, e)
		}
	}
	return out
}

// mockTicketService implements TicketService for testing,
// mirroring TicketWorkflowStore behavior (calls store + writes audit).
type mockTicketService struct {
	mu            sync.Mutex
	tickets       *memory.TicketStore
	auditRecorder *ticketAuditRecorder
	calls         []struct {
		method string
		args   []string
	}
}

func newMockTicketService(auditRecorder *ticketAuditRecorder) *mockTicketService {
	return &mockTicketService{tickets: memory.NewTicketStore(), auditRecorder: auditRecorder}
}

func (m *mockTicketService) ListOpen(ctx context.Context, limit int) ([]ticket.Ticket, error) {
	return m.tickets.ListOpen(ctx, limit)
}

func (m *mockTicketService) GetByID(ctx context.Context, id string) (*ticket.Ticket, error) {
	return m.tickets.GetByID(ctx, id)
}

func (m *mockTicketService) Assign(ctx context.Context, ticketID, agentID, actorID, sourceIP string, now time.Time) error {
	m.mu.Lock()
	m.calls = append(m.calls, struct {
		method string
		args   []string
	}{method: "Assign", args: []string{ticketID, agentID, actorID, sourceIP}})
	m.mu.Unlock()
	if err := m.tickets.Assign(ctx, ticketID, agentID, actorID, sourceIP, now); err != nil {
		return err
	}
	evt := audit.Event{
		ID:         fmt.Sprintf("wf-%d", now.UnixNano()),
		Type:       "ticket_state_changed",
		Action:     "assign",
		TicketID:   ticketID,
		ActorID:    actorID,
		SourceIP:   sourceIP,
		AfterState: map[string]any{"assigned_to": agentID, "status": ticket.StatusAssigned},
		CreatedAt:  now,
	}
	m.auditRecorder.Add(ctx, evt)
	return nil
}

func (m *mockTicketService) Resolve(ctx context.Context, ticketID, resolution, actorID, sourceIP string, now time.Time) error {
	m.mu.Lock()
	m.calls = append(m.calls, struct {
		method string
		args   []string
	}{method: "Resolve", args: []string{ticketID, resolution, actorID, sourceIP}})
	m.mu.Unlock()
	if err := m.tickets.Resolve(ctx, ticketID, resolution, actorID, sourceIP, now); err != nil {
		return err
	}
	evt := audit.Event{
		ID:         fmt.Sprintf("wf-%d", now.UnixNano()),
		Type:       "ticket_state_changed",
		Action:     "resolve",
		TicketID:   ticketID,
		ActorID:    actorID,
		SourceIP:   sourceIP,
		AfterState: map[string]any{"resolution": resolution, "status": ticket.StatusResolved},
		CreatedAt:  now,
	}
	m.auditRecorder.Add(ctx, evt)
	return nil
}

func (m *mockTicketService) Close(ctx context.Context, ticketID, resolution, actorID, sourceIP string, now time.Time) error {
	m.mu.Lock()
	m.calls = append(m.calls, struct {
		method string
		args   []string
	}{method: "Close", args: []string{ticketID, resolution, actorID, sourceIP}})
	m.mu.Unlock()
	if err := m.tickets.Close(ctx, ticketID, resolution, actorID, sourceIP, now); err != nil {
		return err
	}
	evt := audit.Event{
		ID:         fmt.Sprintf("wf-%d", now.UnixNano()),
		Type:       "ticket_state_changed",
		Action:     "close",
		TicketID:   ticketID,
		ActorID:    actorID,
		SourceIP:   sourceIP,
		AfterState: map[string]any{"resolution": resolution, "status": ticket.StatusClosed},
		CreatedAt:  now,
	}
	m.auditRecorder.Add(ctx, evt)
	return nil
}

func (m *mockTicketService) lastCall() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) == 0 {
		return nil
	}
	return m.calls[len(m.calls)-1].args
}

func TestTicketHandlerAssignAuditsStateChange(t *testing.T) {
	auditRecorder := &ticketAuditRecorder{}
	svc := newMockTicketService(auditRecorder)
	now := time.Date(2026, 4, 29, 21, 0, 0, 0, time.UTC)
	if err := svc.tickets.Create(context.Background(), &ticket.Ticket{
		ID:            "ticket-1",
		SessionID:     "session-1",
		Priority:      ticket.PriorityP1,
		Status:        ticket.StatusOpen,
		HandoffReason: "refund",
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	h := NewTicketHandler(svc, auditRecorder)
	h.now = func() time.Time { return now.Add(time.Minute) }

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets/ticket-1/assign?agent_id=agent-007", nil)
	req = withActor(req, "admin-1", "admin")
	resp := httptest.NewRecorder()
	h.Assign(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	assignEvents := auditRecorder.eventsOfType("assign")
	if len(assignEvents) != 2 {
		t.Fatalf("audit assign count = %d, want 2", len(assignEvents))
	}
	event := assignEvents[1]
	if event.Type != "ticket_state_changed" {
		t.Fatalf("event.Type = %s, want ticket_state_changed", event.Type)
	}
	if event.TicketID != "ticket-1" {
		t.Fatalf("ticket id = %s, want ticket-1", event.TicketID)
	}
	if event.AfterState["assigned_to"] != "agent-007" {
		t.Fatalf("assigned_to = %v, want agent-007", event.AfterState["assigned_to"])
	}
	if event.AfterState["status"] != ticket.StatusAssigned {
		t.Fatalf("status = %v, want %s", event.AfterState["status"], ticket.StatusAssigned)
	}
	if event.ActorID != "admin-1" {
		t.Fatalf("actor_id = %v, want admin-1", event.ActorID)
	}
}

func TestTicketHandlerResolveAuditsStateChange(t *testing.T) {
	auditRecorder := &ticketAuditRecorder{}
	svc := newMockTicketService(auditRecorder)
	now := time.Date(2026, 4, 29, 21, 0, 0, 0, time.UTC)
	if err := svc.tickets.Create(context.Background(), &ticket.Ticket{
		ID:            "ticket-2",
		SessionID:     "session-2",
		Priority:      ticket.PriorityP2,
		Status:        ticket.StatusAssigned,
		AssignedTo:    "agent-1",
		HandoffReason: "quota",
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	h := NewTicketHandler(svc, auditRecorder)
	h.now = func() time.Time { return now.Add(2 * time.Minute) }

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets/ticket-2/resolve?resolution=handled", nil)
	req = withActor(req, "admin-2", "admin")
	resp := httptest.NewRecorder()
	h.Resolve(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	resolveEvents := auditRecorder.eventsOfType("resolve")
	if len(resolveEvents) != 2 {
		t.Fatalf("audit resolve count = %d, want 2", len(resolveEvents))
	}
	event := resolveEvents[1]
	if event.Action != "resolve" {
		t.Fatalf("action = %s, want resolve", event.Action)
	}
	if event.AfterState["resolution"] != "handled" {
		t.Fatalf("resolution = %v, want handled", event.AfterState["resolution"])
	}
	if event.AfterState["status"] != ticket.StatusResolved {
		t.Fatalf("status = %v, want %s", event.AfterState["status"], ticket.StatusResolved)
	}
	if event.ActorID != "admin-2" {
		t.Fatalf("actor_id = %v, want admin-2", event.ActorID)
	}
	stored := svc.tickets.List()
	if len(stored) != 1 || stored[0].Status != ticket.StatusResolved {
		t.Fatalf("stored status = %#v", stored)
	}
	if stored[0].ResolvedAt == nil {
		t.Fatalf("expected resolved_at to be set")
	}
}

func TestTicketHandlerCloseRequiresResolution(t *testing.T) {
	h := NewTicketHandler(newMockTicketService(&ticketAuditRecorder{}), &ticketAuditRecorder{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets/ticket-1/close", nil)
	resp := httptest.NewRecorder()
	h.Close(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json decode error = %v", err)
	}
	errPayload := payload["error"].(map[string]any)
	if errPayload["code"] != "CS_REQ_4007" {
		t.Fatalf("error code = %v, want CS_REQ_4007", errPayload["code"])
	}
}

func TestTicketHandlerAssignPassesActorAndSourceIP(t *testing.T) {
	auditRecorder := &ticketAuditRecorder{}
	svc := newMockTicketService(auditRecorder)
	now := time.Date(2026, 4, 29, 21, 0, 0, 0, time.UTC)
	if err := svc.tickets.Create(context.Background(), &ticket.Ticket{
		ID:            "ticket-3",
		SessionID:     "session-3",
		Priority:      ticket.PriorityP0,
		Status:        ticket.StatusOpen,
		HandoffReason: "urgent",
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	h := NewTicketHandler(svc, auditRecorder)
	h.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets/ticket-3/assign?agent_id=agent-x", nil)
	req = withActor(req, "supervisor-1", "supervisor")
	req.RemoteAddr = "192.168.1.100:12345"
	resp := httptest.NewRecorder()
	h.Assign(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	args := svc.lastCall()
	if len(args) < 4 {
		t.Fatalf("call args count = %d, want at least 4", len(args))
	}
	if args[2] != "supervisor-1" {
		t.Fatalf("actor_id = %s, want supervisor-1", args[2])
	}
	if args[3] != "192.168.1.100" {
		t.Fatalf("source_ip = %s, want 192.168.1.100", args[3])
	}
}

func TestTicketHandlerClosePassesActorAndSourceIP(t *testing.T) {
	auditRecorder := &ticketAuditRecorder{}
	svc := newMockTicketService(auditRecorder)
	now := time.Date(2026, 4, 29, 21, 0, 0, 0, time.UTC)
	if err := svc.tickets.Create(context.Background(), &ticket.Ticket{
		ID:            "ticket-4",
		SessionID:     "session-4",
		Priority:      ticket.PriorityP1,
		Status:        ticket.StatusResolved,
		HandoffReason: "done",
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	h := NewTicketHandler(svc, auditRecorder)
	h.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets/ticket-4/close?resolution=closed+by+agent", nil)
	req = withActor(req, "admin-1", "admin")
	req.RemoteAddr = "10.0.0.1:54321"
	resp := httptest.NewRecorder()
	h.Close(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	args := svc.lastCall()
	if len(args) < 4 {
		t.Fatalf("call args count = %d, want at least 4", len(args))
	}
	if args[2] != "admin-1" {
		t.Fatalf("actor_id = %s, want admin-1", args[2])
	}
	if args[3] != "10.0.0.1" {
		t.Fatalf("source_ip = %s, want 10.0.0.1", args[3])
	}
}

// P1-3: GET /api/v1/customer-service/tickets/{id} — Phase 1 minimum implementation

func TestTicketHandlerGetByID_NotFound(t *testing.T) {
	h := NewTicketHandler(newMockTicketService(&ticketAuditRecorder{}), &ticketAuditRecorder{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/customer-service/tickets/nonexistent-id", nil)
	resp := httptest.NewRecorder()
	h.Get(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json decode error = %v", err)
	}
	errPayload := payload["error"].(map[string]any)
	if errPayload["code"] != "CS_TICKET_4001" {
		t.Fatalf("error code = %v, want CS_TICKET_4001", errPayload["code"])
	}
}

func TestTicketHandlerGetByID_Success(t *testing.T) {
	auditRecorder := &ticketAuditRecorder{}
	svc := newMockTicketService(auditRecorder)
	now := time.Date(2026, 4, 29, 21, 0, 0, 0, time.UTC)
	expectedTicket := &ticket.Ticket{
		ID:              "ticket-get-1",
		SessionID:       "session-get-1",
		UserID:          "user-get-1",
		Priority:        ticket.PriorityP1,
		Status:          ticket.StatusOpen,
		HandoffReason:   "refund",
		AssignedTo:      "",
		ContextSnapshot: map[string]any{"channel": "widget", "open_id": "u1"},
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := svc.tickets.Create(context.Background(), expectedTicket); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	h := NewTicketHandler(svc, auditRecorder)
	h.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customer-service/tickets/ticket-get-1", nil)
	resp := httptest.NewRecorder()
	h.Get(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json decode error = %v", err)
	}
	tkt, ok := payload["ticket"].(map[string]any)
	if !ok {
		t.Fatalf("ticket field missing or not a map: %v", payload)
	}
	// Verify all critical fields
	if tkt["id"] != "ticket-get-1" {
		t.Fatalf("id = %v, want ticket-get-1", tkt["id"])
	}
	if tkt["session_id"] != "session-get-1" {
		t.Fatalf("session_id = %v, want session-get-1", tkt["session_id"])
	}
	if tkt["user_id"] != "user-get-1" {
		t.Fatalf("user_id = %v, want user-get-1", tkt["user_id"])
	}
	if tkt["priority"] != "P1" {
		t.Fatalf("priority = %v, want P1", tkt["priority"])
	}
	if tkt["status"] != "open" {
		t.Fatalf("status = %v, want open", tkt["status"])
	}
	if tkt["handoff_reason"] != "refund" {
		t.Fatalf("handoff_reason = %v, want refund", tkt["handoff_reason"])
	}
	if tkt["context_snapshot"] == nil {
		t.Fatalf("context_snapshot is nil, want non-nil")
	}
}

func TestTicketHandlerAssign_RejectsWhenActorOnlyProvidedByQuery(t *testing.T) {
	auditRecorder := &ticketAuditRecorder{}
	svc := newMockTicketService(auditRecorder)
	now := time.Date(2026, 4, 29, 21, 0, 0, 0, time.UTC)
	if err := svc.tickets.Create(context.Background(), &ticket.Ticket{
		ID:            "ticket-auth-1",
		SessionID:     "session-auth-1",
		Priority:      ticket.PriorityP1,
		Status:        ticket.StatusOpen,
		HandoffReason: "refund",
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	h := NewTicketHandler(svc, auditRecorder)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets/ticket-auth-1/assign?agent_id=agent-007&actor_id=forged-admin", nil)
	resp := httptest.NewRecorder()
	h.Assign(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.Code)
	}
}
