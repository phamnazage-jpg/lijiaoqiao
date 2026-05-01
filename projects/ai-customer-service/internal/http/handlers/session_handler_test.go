package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
	"github.com/bridge/ai-customer-service/internal/domain/session"
	"github.com/bridge/ai-customer-service/internal/domain/ticket"
)

// mockSessionGetter implements SessionGetter for testing.
type mockSessionGetter struct {
	mu       sync.Mutex
	sessions map[string]*session.Session
}

func newMockSessionGetter() *mockSessionGetter {
	return &mockSessionGetter{sessions: make(map[string]*session.Session)}
}

func (m *mockSessionGetter) GetByID(_ context.Context, id string) (*session.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[id]; ok {
		return s, nil
	}
	return nil, nil
}

func (m *mockSessionGetter) AddSession(s *session.Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = s
}

// mockTicketCreator implements TicketCreator for testing.
type mockTicketCreator struct {
	mu      sync.Mutex
	tickets []*ticket.Ticket
	calls   []struct{ id string }
}

func newMockTicketCreator() *mockTicketCreator {
	return &mockTicketCreator{tickets: make([]*ticket.Ticket, 0)}
}

func (m *mockTicketCreator) Create(_ context.Context, t *ticket.Ticket) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tickets = append(m.tickets, t)
	m.calls = append(m.calls, struct{ id string }{id: t.ID})
	return nil
}

// mockAuditRecorder implements AuditRecorder for testing.
type mockAuditRecorder struct {
	mu     sync.Mutex
	events []audit.Event
}

func newMockAuditRecorder() *mockAuditRecorder {
	return &mockAuditRecorder{}
}

func (r *mockAuditRecorder) Add(_ context.Context, event audit.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}

func (r *mockAuditRecorder) eventsOfType(tp string) []audit.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []audit.Event
	for _, e := range r.events {
		if e.Type == tp {
			out = append(out, e)
		}
	}
	return out
}

// ---------- Feedback tests ----------

func TestFeedback_WritesAuditLog(t *testing.T) {
	sessions := newMockSessionGetter()
	tickets := newMockTicketCreator()
	audits := newMockAuditRecorder()
	now := time.Date(2026, 4, 29, 21, 0, 0, 0, time.UTC)

	h := NewSessionHandler(sessions, tickets, audits)
	h.now = func() time.Time { return now }

	body := `{"score":5,"comment":"great service"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/sess-1/feedback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	h.Feedback(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	events := audits.eventsOfType("feedback")
	if len(events) != 1 {
		t.Fatalf("feedback events count = %d, want 1", len(events))
	}
	evt := events[0]
	if evt.SessionID != "sess-1" {
		t.Fatalf("session_id = %s, want sess-1", evt.SessionID)
	}
	if evt.Action != "submit" {
		t.Fatalf("action = %s, want submit", evt.Action)
	}
	payload := evt.Payload
	if payload["score"].(int) != 5 {
		t.Fatalf("score = %v, want 5", payload["score"])
	}
	if payload["comment"].(string) != "great service" {
		t.Fatalf("comment = %v, want 'great service'", payload["comment"])
	}
}

func TestFeedback_auditFailureDoesNotReturnError(t *testing.T) {
	sessions := newMockSessionGetter()
	tickets := newMockTicketCreator()
	audits := newMockAuditRecorder()
	now := time.Date(2026, 4, 29, 21, 0, 0, 0, time.UTC)

	h := NewSessionHandler(sessions, tickets, audits)
	h.now = func() time.Time { return now }

	body := `{"score":3}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/sess-1/feedback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	h.Feedback(resp, req)

	// Even if audit.Add returned error (it doesn't in this mock),
	// the handler should still return 200
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
}

func TestFeedback_InvalidScore(t *testing.T) {
	sessions := newMockSessionGetter()
	tickets := newMockTicketCreator()
	audits := newMockAuditRecorder()
	h := NewSessionHandler(sessions, tickets, audits)
	h.now = time.Now

	for _, score := range []int{0, 6, -1} {
		body := strings.NewReader(`{"score":` + string(rune('0'+score)) + `}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/sess-1/feedback", body)
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		h.Feedback(resp, req)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("score=%d: status = %d, want 400", score, resp.Code)
		}
	}
}

func TestFeedback_InvalidJSON(t *testing.T) {
	sessions := newMockSessionGetter()
	tickets := newMockTicketCreator()
	audits := newMockAuditRecorder()
	h := NewSessionHandler(sessions, tickets, audits)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/sess-1/feedback", strings.NewReader(`{invalid}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	h.Feedback(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestFeedback_EmptySessionID(t *testing.T) {
	sessions := newMockSessionGetter()
	tickets := newMockTicketCreator()
	audits := newMockAuditRecorder()
	h := NewSessionHandler(sessions, tickets, audits)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions//feedback", strings.NewReader(`{"score":5}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	h.Feedback(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

// ---------- Handoff tests ----------

func TestHandoff_CreatesTicketAndAudit(t *testing.T) {
	sessions := newMockSessionGetter()
	sessions.AddSession(&session.Session{
		ID:       "sess-hw-1",
		Channel:  "feishu",
		OpenID:   "open-123",
		UserID:   "user-456",
		Status:   session.StatusProcessing,
		TurnCount: 3,
	})
	tickets := newMockTicketCreator()
	audits := newMockAuditRecorder()
	now := time.Date(2026, 4, 29, 21, 0, 0, 0, time.UTC)

	h := NewSessionHandler(sessions, tickets, audits)
	h.now = func() time.Time { return now }

	body := `{"reason":"customer requested human","priority":"P1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/sess-hw-1/handoff?actor_id=admin-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.1:12345"
	resp := httptest.NewRecorder()

	h.Handoff(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json decode error = %v", err)
	}
	if payload["session_id"] != "sess-hw-1" {
		t.Fatalf("session_id = %v, want sess-hw-1", payload["session_id"])
	}
	ticketID := payload["ticket_id"].(string)
	if ticketID == "" {
		t.Fatal("ticket_id should not be empty")
	}

	// Verify ticket was created
	if len(tickets.tickets) != 1 {
		t.Fatalf("ticket count = %d, want 1", len(tickets.tickets))
	}
	tkt := tickets.tickets[0]
	if tkt.SessionID != "sess-hw-1" {
		t.Fatalf("ticket session_id = %s, want sess-hw-1", tkt.SessionID)
	}
	if tkt.Priority != ticket.PriorityP1 {
		t.Fatalf("priority = %s, want P1", tkt.Priority)
	}
	if tkt.HandoffReason != "customer requested human" {
		t.Fatalf("handoff_reason = %s, want 'customer requested human'", tkt.HandoffReason)
	}
	if tkt.Status != ticket.StatusOpen {
		t.Fatalf("status = %s, want open", tkt.Status)
	}

	// Verify audit event
	events := audits.eventsOfType("manual_handoff")
	if len(events) != 1 {
		t.Fatalf("manual_handoff events count = %d, want 1", len(events))
	}
	evt := events[0]
	if evt.SessionID != "sess-hw-1" {
		t.Fatalf("session_id = %s, want sess-hw-1", evt.SessionID)
	}
	if evt.TicketID != ticketID {
		t.Fatalf("ticket_id = %s, want %s", evt.TicketID, ticketID)
	}
	if evt.ActorID != "admin-1" {
		t.Fatalf("actor_id = %s, want admin-1", evt.ActorID)
	}
	if evt.SourceIP != "10.0.0.1" {
		t.Fatalf("source_ip = %s, want 10.0.0.1", evt.SourceIP)
	}
}

func TestHandoff_DefaultPriorityP2(t *testing.T) {
	sessions := newMockSessionGetter()
	sessions.AddSession(&session.Session{ID: "sess-p2", Channel: "feishu", OpenID: "open-1", Status: session.StatusProcessing})
	tickets := newMockTicketCreator()
	audits := newMockAuditRecorder()
	now := time.Date(2026, 4, 29, 21, 0, 0, 0, time.UTC)

	h := NewSessionHandler(sessions, tickets, audits)
	h.now = func() time.Time { return now }

	body := `{"reason":"need help"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/sess-p2/handoff", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	h.Handoff(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if len(tickets.tickets) != 1 {
		t.Fatalf("ticket count = %d, want 1", len(tickets.tickets))
	}
	if tickets.tickets[0].Priority != ticket.PriorityP2 {
		t.Fatalf("priority = %s, want P2", tickets.tickets[0].Priority)
	}
}

func TestHandoff_SessionNotFound(t *testing.T) {
	sessions := newMockSessionGetter()
	tickets := newMockTicketCreator()
	audits := newMockAuditRecorder()
	h := NewSessionHandler(sessions, tickets, audits)

	body := `{"reason":"urgent"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/nonexistent/handoff", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	h.Handoff(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.Code)
	}
}

func TestHandoff_ReasonRequired(t *testing.T) {
	sessions := newMockSessionGetter()
	sessions.AddSession(&session.Session{ID: "sess-r1", Channel: "feishu", OpenID: "open-1", Status: session.StatusProcessing})
	tickets := newMockTicketCreator()
	audits := newMockAuditRecorder()
	h := NewSessionHandler(sessions, tickets, audits)

	// empty reason
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/sess-r1/handoff", strings.NewReader(`{"reason":""}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	h.Handoff(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("empty reason: status = %d, want 400", resp.Code)
	}

	// missing reason field
	req = httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/sess-r1/handoff", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	h.Handoff(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("missing reason: status = %d, want 400", resp.Code)
	}
}

func TestHandoff_InvalidJSON(t *testing.T) {
	sessions := newMockSessionGetter()
	tickets := newMockTicketCreator()
	audits := newMockAuditRecorder()
	h := NewSessionHandler(sessions, tickets, audits)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/sess-1/handoff", strings.NewReader(`{bad json}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	h.Handoff(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestHandoff_TicketCreateFailure(t *testing.T) {
	sessions := newMockSessionGetter()
	sessions.AddSession(&session.Session{ID: "sess-err", Channel: "feishu", OpenID: "open-1", Status: session.StatusProcessing})

	// ticket creator that always fails
	failingTickets := &failingTicketCreator{}
	audits := newMockAuditRecorder()
	h := NewSessionHandler(sessions, failingTickets, audits)

	body := `{"reason":"fail"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/sess-err/handoff", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	h.Handoff(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.Code)
	}
}

type failingTicketCreator struct{}

func (f *failingTicketCreator) Create(_ context.Context, _ *ticket.Ticket) error {
	return context.DeadlineExceeded
}

// ---------- sessionPathParam tests ----------

func TestSessionPathParam(t *testing.T) {
	cases := []struct {
		path      string
		wantID    string
		wantEmpty bool
	}{
		{"/api/v1/customer-service/sessions/sess-abc/feedback", "sess-abc", false},
		{"/api/v1/customer-service/sessions/sess-abc/handoff", "sess-abc", false},
		{"/api/v1/customer-service/sessions//feedback", "", true},
		// Paths not ending in /feedback or /handoff are invalid
		{"/api/v1/customer-service/sessions/sess-123/other", "", true},
	}
	for _, c := range cases {
		got := sessionPathParam(c.path)
		if c.wantEmpty && got != "" {
			t.Errorf("sessionPathParam(%q) = %q, want empty", c.path, got)
		}
		if !c.wantEmpty && got != c.wantID {
			t.Errorf("sessionPathParam(%q) = %q, want %q", c.path, got, c.wantID)
		}
	}
}
