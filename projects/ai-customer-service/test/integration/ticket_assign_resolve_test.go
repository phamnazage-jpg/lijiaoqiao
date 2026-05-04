package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
	"github.com/bridge/ai-customer-service/internal/domain/ticket"
	"github.com/bridge/ai-customer-service/internal/http/handlers"
	"github.com/bridge/ai-customer-service/internal/store/memory"
)

// --------------------------------------------------
// Shared mock infrastructure
// --------------------------------------------------

type arAuditRecorder struct{ events []audit.Event }

func (r *arAuditRecorder) Add(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

func (r *arAuditRecorder) eventsOfType(action string) []audit.Event {
	var out []audit.Event
	for _, e := range r.events {
		if e.Action == action {
			out = append(out, e)
		}
	}
	return out
}

// mockAssignResolveService wraps memory.TicketStore and satisfies TicketService.
type mockAssignResolveService struct {
	store *memory.TicketStore
	audit *arAuditRecorder
}

func newMockAssignResolveService(auditRecorder *arAuditRecorder) *mockAssignResolveService {
	return &mockAssignResolveService{
		store: memory.NewTicketStore(),
		audit: auditRecorder,
	}
}

func (m *mockAssignResolveService) ListOpen(ctx context.Context, limit int) ([]ticket.Ticket, error) {
	return m.store.ListOpen(ctx, limit)
}

func (m *mockAssignResolveService) GetByID(ctx context.Context, id string) (*ticket.Ticket, error) {
	return m.store.GetByID(ctx, id)
}

func (m *mockAssignResolveService) Assign(ctx context.Context, ticketID, agentID, actorID, sourceIP string, now time.Time) error {
	if err := m.store.Assign(ctx, ticketID, agentID, actorID, sourceIP, now); err != nil {
		return err
	}
	m.audit.Add(ctx, audit.Event{
		ID:       "audit-assign-" + ticketID,
		Type:     "ticket_state_changed",
		Action:   "assign",
		TicketID: ticketID,
		ActorID:  actorID,
		SourceIP: sourceIP,
		AfterState: map[string]any{
			"assigned_to": agentID,
			"status":      ticket.StatusAssigned,
		},
		CreatedAt: now,
	})
	return nil
}

func (m *mockAssignResolveService) Resolve(ctx context.Context, ticketID, resolution, actorID, sourceIP string, now time.Time) error {
	tkt, _ := m.store.GetByID(ctx, ticketID)
	if tkt == nil {
		return fmt.Errorf("ticket not found")
	}
	// Enforce state machine: only assigned/processing tickets can be resolved
	if tkt.Status != ticket.StatusAssigned && tkt.Status != ticket.StatusProcessing {
		return fmt.Errorf("ticket not resolvable from status: %s", tkt.Status)
	}
	if err := m.store.Resolve(ctx, ticketID, resolution, actorID, sourceIP, now); err != nil {
		return err
	}
	m.audit.Add(ctx, audit.Event{
		ID:       "audit-resolve-" + ticketID,
		Type:     "ticket_state_changed",
		Action:   "resolve",
		TicketID: ticketID,
		ActorID:  actorID,
		SourceIP: sourceIP,
		AfterState: map[string]any{
			"resolution": resolution,
			"status":     ticket.StatusResolved,
		},
		CreatedAt: now,
	})
	return nil
}

func (m *mockAssignResolveService) Close(ctx context.Context, ticketID, resolution, actorID, sourceIP string, now time.Time) error {
	if err := m.store.Close(ctx, ticketID, resolution, actorID, sourceIP, now); err != nil {
		return err
	}
	m.audit.Add(ctx, audit.Event{
		ID:       "audit-close-" + ticketID,
		Type:     "ticket_state_changed",
		Action:   "close",
		TicketID: ticketID,
		ActorID:  actorID,
		SourceIP: sourceIP,
		AfterState: map[string]any{
			"resolution": resolution,
			"status":     ticket.StatusClosed,
		},
		CreatedAt: now,
	})
	return nil
}

// --------------------------------------------------
// Tests: POST /assign — state transitions
// --------------------------------------------------

// TestAssign_UpdatesStatusToAssigned verifies that assigning an open ticket
// transitions it to the "assigned" status.
func TestAssign_UpdatesStatusToAssigned(t *testing.T) {
	auditRecorder := &arAuditRecorder{}
	svc := newMockAssignResolveService(auditRecorder)
	now := time.Date(2026, 4, 30, 13, 0, 0, 0, time.UTC)
	ctx := context.Background()

	// Create an open ticket
	_ = svc.store.Create(ctx, &ticket.Ticket{
		ID:            "assign-tkt-1",
		SessionID:     "session-assign-1",
		UserID:        "user-assign-1",
		Priority:      ticket.PriorityP1,
		Status:        ticket.StatusOpen,
		HandoffReason: "refund request",
		CreatedAt:     now,
		UpdatedAt:     now,
	})

	h := handlers.NewTicketHandler(svc, auditRecorder)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets/assign-tkt-1/assign?agent_id=agent-001", nil)
	req = withActor(req, "supervisor-1", "supervisor")
	req.RemoteAddr = "10.0.0.5:12345"
	resp := httptest.NewRecorder()
	h.Assign(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("assign status = %d, want 200; body: %s", resp.Code, resp.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if payload["assigned"] != true {
		t.Fatalf("assigned = %v, want true", payload["assigned"])
	}

	// Verify ticket status in store
	tkt, _ := svc.store.GetByID(ctx, "assign-tkt-1")
	if tkt.Status != ticket.StatusAssigned {
		t.Fatalf("ticket status = %s, want assigned", tkt.Status)
	}
	if tkt.AssignedTo != "agent-001" {
		t.Fatalf("assigned_to = %s, want agent-001", tkt.AssignedTo)
	}
}

// TestAssign_CannotReassignAlreadyAssigned verifies that a ticket already
// assigned cannot be reassigned (returns 409 Conflict).
func TestAssign_CannotReassignAlreadyAssigned(t *testing.T) {
	auditRecorder := &arAuditRecorder{}
	svc := newMockAssignResolveService(auditRecorder)
	now := time.Date(2026, 4, 30, 13, 0, 0, 0, time.UTC)
	ctx := context.Background()

	_ = svc.store.Create(ctx, &ticket.Ticket{
		ID:            "assign-tkt-2",
		SessionID:     "session-assign-2",
		Priority:      ticket.PriorityP2,
		Status:        ticket.StatusAssigned,
		AssignedTo:    "agent-first",
		HandoffReason: "quota inquiry",
		CreatedAt:     now,
		UpdatedAt:     now,
	})

	h := handlers.NewTicketHandler(svc, auditRecorder)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets/assign-tkt-2/assign?agent_id=agent-second", nil)
	req = withActor(req, "supervisor-2", "supervisor")
	resp := httptest.NewRecorder()
	h.Assign(resp, req)

	if resp.Code != http.StatusConflict {
		t.Fatalf("assign already-assigned ticket status = %d, want 409", resp.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	errPayload := payload["error"].(map[string]any)
	if errPayload["code"] != "CS_TKT_4002" {
		t.Fatalf("error code = %v, want CS_TKT_4002", errPayload["code"])
	}
}

// TestAssign_MissingAgentID returns 400.
func TestAssign_MissingAgentID(t *testing.T) {
	auditRecorder := &arAuditRecorder{}
	svc := newMockAssignResolveService(auditRecorder)
	h := handlers.NewTicketHandler(svc, auditRecorder)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets/some-ticket/assign", nil)
	resp := httptest.NewRecorder()
	h.Assign(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

// --------------------------------------------------
// Tests: POST /resolve — state transitions
// --------------------------------------------------

// TestResolve_UpdatesStatusToResolved verifies that resolving an assigned ticket
// transitions it to the "resolved" status.
func TestResolve_UpdatesStatusToResolved(t *testing.T) {
	auditRecorder := &arAuditRecorder{}
	svc := newMockAssignResolveService(auditRecorder)
	now := time.Date(2026, 4, 30, 13, 0, 0, 0, time.UTC)
	ctx := context.Background()

	_ = svc.store.Create(ctx, &ticket.Ticket{
		ID:            "resolve-tkt-1",
		SessionID:     "session-resolve-1",
		Priority:      ticket.PriorityP2,
		Status:        ticket.StatusAssigned,
		AssignedTo:    "agent-001",
		HandoffReason: "account issue",
		CreatedAt:     now,
		UpdatedAt:     now,
	})

	h := handlers.NewTicketHandler(svc, auditRecorder)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets/resolve-tkt-1/resolve?resolution=issue+fixed", nil)
	req = withActor(req, "agent-001", "agent")
	req.RemoteAddr = "10.0.0.6:54321"
	resp := httptest.NewRecorder()
	h.Resolve(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200; body: %s", resp.Code, resp.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if payload["resolved"] != true {
		t.Fatalf("resolved = %v, want true", payload["resolved"])
	}

	// Verify ticket in store
	tkt, _ := svc.store.GetByID(ctx, "resolve-tkt-1")
	if tkt.Status != ticket.StatusResolved {
		t.Fatalf("ticket status = %s, want resolved", tkt.Status)
	}
	if tkt.Resolution != "issue fixed" {
		t.Fatalf("resolution = %q, want 'issue fixed'", tkt.Resolution)
	}
	if tkt.ResolvedAt == nil {
		t.Fatalf("resolved_at should be set")
	}
}

// TestResolve_CannotResolveClosedTicket verifies that resolving a closed
// ticket returns 409 Conflict.
func TestResolve_CannotResolveClosedTicket(t *testing.T) {
	auditRecorder := &arAuditRecorder{}
	svc := newMockAssignResolveService(auditRecorder)
	now := time.Date(2026, 4, 30, 13, 0, 0, 0, time.UTC)
	ctx := context.Background()

	_ = svc.store.Create(ctx, &ticket.Ticket{
		ID:            "resolve-tkt-closed",
		SessionID:     "session-closed",
		Priority:      ticket.PriorityP3,
		Status:        ticket.StatusClosed,
		AssignedTo:    "agent-001",
		HandoffReason: "done",
		CreatedAt:     now,
		UpdatedAt:     now,
	})

	h := handlers.NewTicketHandler(svc, auditRecorder)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets/resolve-tkt-closed/resolve?resolution=already+closed", nil)
	req = withActor(req, "agent-001", "agent")
	resp := httptest.NewRecorder()
	h.Resolve(resp, req)

	if resp.Code != http.StatusConflict {
		t.Fatalf("resolve closed ticket status = %d, want 409", resp.Code)
	}
}

// TestResolve_MissingResolution returns 400.
func TestResolve_MissingResolution(t *testing.T) {
	auditRecorder := &arAuditRecorder{}
	svc := newMockAssignResolveService(auditRecorder)
	h := handlers.NewTicketHandler(svc, auditRecorder)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets/some-ticket/resolve", nil)
	resp := httptest.NewRecorder()
	h.Resolve(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

// TestResolve_TicketNotFound returns 409.
func TestResolve_TicketNotFound(t *testing.T) {
	auditRecorder := &arAuditRecorder{}
	svc := newMockAssignResolveService(auditRecorder)
	h := handlers.NewTicketHandler(svc, auditRecorder)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets/nonexistent/resolve?resolution=not+found", nil)
	req = withActor(req, "agent-404", "agent")
	resp := httptest.NewRecorder()
	h.Resolve(resp, req)

	if resp.Code != http.StatusConflict {
		t.Fatalf("resolve nonexistent ticket status = %d, want 409", resp.Code)
	}
}

// --------------------------------------------------
// Tests: State transition correctness
// --------------------------------------------------

// TestStateTransition_OpenToAssignedToResolved verifies the full happy-path
// state transition: open → assigned → resolved.
func TestStateTransition_OpenToAssignedToResolved(t *testing.T) {
	auditRecorder := &arAuditRecorder{}
	svc := newMockAssignResolveService(auditRecorder)
	now := time.Date(2026, 4, 30, 14, 0, 0, 0, time.UTC)
	ctx := context.Background()

	_ = svc.store.Create(ctx, &ticket.Ticket{
		ID:            "state-tkt-1",
		SessionID:     "session-state-1",
		UserID:        "user-state-1",
		Priority:      ticket.PriorityP1,
		Status:        ticket.StatusOpen,
		HandoffReason: "urgent refund",
		CreatedAt:     now,
		UpdatedAt:     now,
	})

	h := handlers.NewTicketHandler(svc, auditRecorder)

	// Step 1: Assign
	assignReq := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets/state-tkt-1/assign?agent_id=agent-alpha", nil)
	assignReq = withActor(assignReq, "admin-1", "admin")
	assignResp := httptest.NewRecorder()
	h.Assign(assignResp, assignReq)
	if assignResp.Code != http.StatusOK {
		t.Fatalf("[assign] status = %d, want 200", assignResp.Code)
	}

	tktAfterAssign, _ := svc.store.GetByID(ctx, "state-tkt-1")
	if tktAfterAssign.Status != ticket.StatusAssigned {
		t.Fatalf("[assign] status = %s, want assigned", tktAfterAssign.Status)
	}
	if tktAfterAssign.AssignedTo != "agent-alpha" {
		t.Fatalf("[assign] assigned_to = %s, want agent-alpha", tktAfterAssign.AssignedTo)
	}

	// Step 2: Resolve
	resolveReq := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets/state-tkt-1/resolve?resolution=refund+processed", nil)
	resolveReq = withActor(resolveReq, "agent-alpha", "agent")
	resolveResp := httptest.NewRecorder()
	h.Resolve(resolveResp, resolveReq)
	if resolveResp.Code != http.StatusOK {
		t.Fatalf("[resolve] status = %d, want 200", resolveResp.Code)
	}

	tktAfterResolve, _ := svc.store.GetByID(ctx, "state-tkt-1")
	if tktAfterResolve.Status != ticket.StatusResolved {
		t.Fatalf("[resolve] status = %s, want resolved", tktAfterResolve.Status)
	}
	if tktAfterResolve.Resolution != "refund processed" {
		t.Fatalf("[resolve] resolution = %q, want 'refund processed'", tktAfterResolve.Resolution)
	}
	if tktAfterResolve.ResolvedAt == nil {
		t.Fatalf("[resolve] resolved_at should be set")
	}
}

// TestStateTransition_InvalidTransition verifies that skipping states
// (e.g., resolving an open ticket directly) returns 409.
func TestStateTransition_InvalidTransition(t *testing.T) {
	auditRecorder := &arAuditRecorder{}
	svc := newMockAssignResolveService(auditRecorder)
	now := time.Date(2026, 4, 30, 14, 0, 0, 0, time.UTC)
	ctx := context.Background()

	_ = svc.store.Create(ctx, &ticket.Ticket{
		ID:            "state-tkt-2",
		SessionID:     "session-state-2",
		Priority:      ticket.PriorityP2,
		Status:        ticket.StatusOpen,
		HandoffReason: "test",
		CreatedAt:     now,
		UpdatedAt:     now,
	})

	h := handlers.NewTicketHandler(svc, auditRecorder)

	// Try to resolve an open ticket directly (should fail — must be assigned first)
	resolveReq := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets/state-tkt-2/resolve?resolution=skip+assign", nil)
	resolveReq = withActor(resolveReq, "agent-skip", "agent")
	resolveResp := httptest.NewRecorder()
	h.Resolve(resolveResp, resolveReq)
	if resolveResp.Code != http.StatusConflict {
		t.Fatalf("resolve open ticket (skip assign) status = %d, want 409", resolveResp.Code)
	}
}
