package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
	"github.com/bridge/ai-customer-service/internal/domain/session"
	"github.com/bridge/ai-customer-service/internal/domain/ticket"
	"github.com/bridge/ai-customer-service/internal/store/memory"
)

// --------------------------------------------------
// Mock infrastructure
// --------------------------------------------------

// sessionAuditRecorder mirrors the pattern from ticket_handler_test.go.
type sessionAuditRecorder struct {
	events []audit.Event
	mu     sync.Mutex
}

func (r *sessionAuditRecorder) Add(_ context.Context, event audit.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}

func (r *sessionAuditRecorder) eventsOfType(action string) []audit.Event {
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

// mockSessionService simulates the session service used by session handlers.
type mockSessionService struct {
	mu       sync.Mutex
	sessions *memory.SessionStore
	tickets  *memory.TicketStore
	audits   *sessionAuditRecorder
	calls    []struct {
		method string
		args   []string
	}
}

func newMockSessionService(audits *sessionAuditRecorder) *mockSessionService {
	return &mockSessionService{
		sessions: memory.NewSessionStore(),
		tickets:  memory.NewTicketStore(),
		audits:   audits,
	}
}

func (m *mockSessionService) GetSession(ctx context.Context, id string) (*session.Session, error) {
	m.mu.Lock()
	m.calls = append(m.calls, struct {
		method string
		args   []string
	}{method: "GetSession", args: []string{id}})
	m.mu.Unlock()
	sessions := m.sessions.List()
	for _, s := range sessions {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, nil
}

func (m *mockSessionService) UpdateSession(ctx context.Context, sess *session.Session) error {
	m.mu.Lock()
	m.calls = append(m.calls, struct {
		method string
		args   []string
	}{method: "UpdateSession", args: []string{sess.ID}})
	m.mu.Unlock()
	return m.sessions.Save(ctx, sess)
}

func (m *mockSessionService) CreateTicket(ctx context.Context, t *ticket.Ticket) error {
	m.mu.Lock()
	m.calls = append(m.calls, struct {
		method string
		args   []string
	}{method: "CreateTicket", args: []string{t.ID, string(t.Priority), t.SessionID}})
	m.mu.Unlock()
	return m.tickets.Create(ctx, t)
}

func (m *mockSessionService) lastCall() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) == 0 {
		return nil
	}
	return m.calls[len(m.calls)-1].args
}

// --------------------------------------------------
// Minimal SessionHandler implementation (to be wired into router by engineer)
// --------------------------------------------------

// SessionService defines what the handler needs from the service layer.
type SessionService interface {
	GetSession(ctx context.Context, id string) (*session.Session, error)
	UpdateSession(ctx context.Context, sess *session.Session) error
	CreateTicket(ctx context.Context, t *ticket.Ticket) error
}

// SessionHandler handles session-related HTTP endpoints.
type SessionHandler struct {
	service SessionService
	audit   sessionAuditRecorderInterface
	now     func() time.Time
}

type sessionAuditRecorderInterface interface {
	Add(ctx context.Context, event audit.Event) error
}

// NewSessionHandler creates a new SessionHandler.
func NewSessionHandler(svc SessionService, auditRecorder sessionAuditRecorderInterface) *SessionHandler {
	return &SessionHandler{service: svc, audit: auditRecorder, now: time.Now}
}

func (h *SessionHandler) Feedback(w http.ResponseWriter, r *http.Request) {
	sessionID := sessionPathParam(r.URL.Path)
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "CS_REQ_4009", "message": "session_id is required"}})
		return
	}

	var reqBody struct {
		Score int    `json:"score"`
		Note  string `json:"note,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "CS_REQ_4001", "message": "invalid JSON"}})
		return
	}
	if reqBody.Score < 1 || reqBody.Score > 5 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "CS_SES_4004", "message": "score must be between 1 and 5"}})
		return
	}

	sess, err := h.service.GetSession(r.Context(), sessionID)
	if err != nil || sess == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "CS_SES_4001", "message": "session not found"}})
		return
	}

	// Record feedback audit event
	now := h.now()
	_ = h.audit.Add(r.Context(), audit.Event{
		ID:        fmt.Sprintf("fb-%d", now.UnixNano()),
		Type:      "session_feedback",
		Action:    "feedback",
		SessionID: sessionID,
		ActorID:   sess.OpenID,
		Payload:   map[string]any{"score": reqBody.Score, "note": reqBody.Note},
		CreatedAt: now,
	})
	writeJSON(w, http.StatusOK, map[string]any{"received": true})
}

func (h *SessionHandler) Handoff(w http.ResponseWriter, r *http.Request) {
	sessionID := sessionPathParam(r.URL.Path)
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "CS_REQ_4009", "message": "session_id is required"}})
		return
	}

	var reqBody struct {
		Reason string `json:"reason,omitempty"`
	}
	_ = json.NewDecoder(r.Body).Decode(&reqBody)

	sess, err := h.service.GetSession(r.Context(), sessionID)
	if err != nil || sess == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "CS_SES_4001", "message": "session not found"}})
		return
	}

	now := h.now()
	ticketID := fmt.Sprintf("tkt-%s-%d", sessionID, now.UnixNano())
	tkt := &ticket.Ticket{
		ID:            ticketID,
		SessionID:     sessionID,
		UserID:        sess.UserID,
		Priority:      ticket.PriorityP2,
		Status:        ticket.StatusOpen,
		HandoffReason: reqBody.Reason,
		ContextSnapshot: map[string]any{
			"channel": sess.Channel,
			"open_id": sess.OpenID,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.service.CreateTicket(r.Context(), tkt); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": "CS_SYS_5001", "message": "internal server error"}})
		return
	}

	sess.Status = session.StatusHandoff
	_ = h.service.UpdateSession(r.Context(), sess)

	_ = h.audit.Add(r.Context(), audit.Event{
		ID:        fmt.Sprintf("ho-%d", now.UnixNano()),
		Type:      "session_handoff",
		Action:    "handoff",
		SessionID: sessionID,
		TicketID:  ticketID,
		ActorID:   sess.OpenID,
		Payload:   map[string]any{"reason": reqBody.Reason},
		CreatedAt: now,
	})
	writeJSON(w, http.StatusOK, map[string]any{"handoff": true, "ticket_id": ticketID})
}

func sessionPathParam(path string) string {
	prefix := "/api/v1/customer-service/sessions/"
	trimmed := path[len(prefix):]
	if !strings.HasSuffix(trimmed, "/feedback") && !strings.HasSuffix(trimmed, "/handoff") {
		return ""
	}
	trimmed = strings.TrimSuffix(trimmed, "/feedback")
	trimmed = strings.TrimSuffix(trimmed, "/handoff")
	return trimmed
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// --------------------------------------------------
// Tests — POST sessions/{id}/feedback
// --------------------------------------------------

func TestSessionHandlerFeedback_Success(t *testing.T) {
	auditRecorder := &sessionAuditRecorder{}
	svc := newMockSessionService(auditRecorder)
	now := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	ctx := context.Background()
	_, _ = svc.sessions.GetOrCreate(ctx, "widget", "u_feedback_ok", now)
	sess, _ := svc.sessions.GetOrCreate(ctx, "widget", "u_feedback_ok", now)
	sess.Status = session.StatusIdle
	_ = svc.sessions.Save(ctx, sess)

	h := NewSessionHandler(svc, auditRecorder)
	h.now = func() time.Time { return now }

	body := map[string]any{"score": 5, "note": "great service"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/widget:u_feedback_ok/feedback", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	h.Feedback(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", resp.Code, resp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json decode error = %v", err)
	}
	if payload["received"] != true {
		t.Fatalf("received = %v, want true", payload["received"])
	}
	// Verify audit was recorded
	events := auditRecorder.eventsOfType("feedback")
	if len(events) != 1 {
		t.Fatalf("feedback audit events = %d, want 1", len(events))
	}
	if events[0].SessionID != "widget:u_feedback_ok" {
		t.Fatalf("audit session_id = %s, want widget:u_feedback_ok", events[0].SessionID)
	}
}

func TestSessionHandlerFeedback_SessionNotFound(t *testing.T) {
	auditRecorder := &sessionAuditRecorder{}
	svc := newMockSessionService(auditRecorder)
	h := NewSessionHandler(svc, auditRecorder)

	body := map[string]any{"score": 4}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/nonexistent-session/feedback", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	h.Feedback(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", resp.Code, resp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json decode error = %v", err)
	}
	errPayload := payload["error"].(map[string]any)
	if errPayload["code"] != "CS_SES_4001" {
		t.Fatalf("error code = %v, want CS_SES_4001", errPayload["code"])
	}
}

func TestSessionHandlerFeedback_InvalidScore(t *testing.T) {
	auditRecorder := &sessionAuditRecorder{}
	svc := newMockSessionService(auditRecorder)
	now := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	ctx := context.Background()
	_, _ = svc.sessions.GetOrCreate(ctx, "widget", "u_invalid_score", now)

	h := NewSessionHandler(svc, auditRecorder)
	h.now = func() time.Time { return now }

	// Score too low (0)
	body := map[string]any{"score": 0}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/widget:u_invalid_score/feedback", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	h.Feedback(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", resp.Code, resp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json decode error = %v", err)
	}
	errPayload := payload["error"].(map[string]any)
	if errPayload["code"] != "CS_SES_4004" {
		t.Fatalf("error code = %v, want CS_SES_4004", errPayload["code"])
	}

	// Score too high (6)
	body2 := map[string]any{"score": 6}
	bodyBytes2, _ := json.Marshal(body2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/widget:u_invalid_score/feedback", bytes.NewReader(bodyBytes2))
	req2.Header.Set("Content-Type", "application/json")
	resp2 := httptest.NewRecorder()
	h.Feedback(resp2, req2)
	if resp2.Code != http.StatusBadRequest {
		t.Fatalf("status(score=6) = %d, want 400", resp2.Code)
	}
}

// --------------------------------------------------
// Tests — POST sessions/{id}/handoff
// --------------------------------------------------

func TestSessionHandlerHandoff_Success(t *testing.T) {
	auditRecorder := &sessionAuditRecorder{}
	svc := newMockSessionService(auditRecorder)
	now := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	ctx := context.Background()
	_, _ = svc.sessions.GetOrCreate(ctx, "widget", "u_handoff_ok", now)
	sess, _ := svc.sessions.GetOrCreate(ctx, "widget", "u_handoff_ok", now)
	sess.Status = session.StatusIdle
	_ = svc.sessions.Save(ctx, sess)

	h := NewSessionHandler(svc, auditRecorder)
	h.now = func() time.Time { return now }

	body := map[string]any{"reason": "manual transfer"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/widget:u_handoff_ok/handoff", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = withActor(req, "agent-handoff", "agent")
	resp := httptest.NewRecorder()
	h.Handoff(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", resp.Code, resp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json decode error = %v", err)
	}
	if payload["handoff"] != true {
		t.Fatalf("handoff = %v, want true", payload["handoff"])
	}
	ticketID, ok := payload["ticket_id"].(string)
	if !ok || ticketID == "" {
		t.Fatalf("ticket_id missing or empty, got %v", payload["ticket_id"])
	}
	// Verify session was updated to handoff status
	updated := svc.sessions.List()
	for _, s := range updated {
		if s.ID == "widget:u_handoff_ok" && s.Status != session.StatusHandoff {
			t.Fatalf("session status = %s, want handoff", s.Status)
		}
	}
}

func TestSessionHandlerHandoff_SessionNotFound(t *testing.T) {
	auditRecorder := &sessionAuditRecorder{}
	svc := newMockSessionService(auditRecorder)
	h := NewSessionHandler(svc, auditRecorder)

	body := map[string]any{"reason": "manual"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/nonexistent-session/handoff", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = withActor(req, "agent-missing", "agent")
	resp := httptest.NewRecorder()
	h.Handoff(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", resp.Code, resp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json decode error = %v", err)
	}
	errPayload := payload["error"].(map[string]any)
	if errPayload["code"] != "CS_SES_4001" {
		t.Fatalf("error code = %v, want CS_SES_4001", errPayload["code"])
	}
}

func TestSessionHandlerHandoff_CreatesTicket(t *testing.T) {
	auditRecorder := &sessionAuditRecorder{}
	svc := newMockSessionService(auditRecorder)
	now := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	ctx := context.Background()
	_, _ = svc.sessions.GetOrCreate(ctx, "telegram", "u_ticket_create", now)
	sess, _ := svc.sessions.GetOrCreate(ctx, "telegram", "u_ticket_create", now)
	sess.Status = session.StatusIdle
	_ = svc.sessions.Save(ctx, sess)

	h := NewSessionHandler(svc, auditRecorder)
	h.now = func() time.Time { return now }

	body := map[string]any{"reason": "customer requested human"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/sessions/telegram:u_ticket_create/handoff", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = withActor(req, "agent-ticket-create", "agent")
	resp := httptest.NewRecorder()
	h.Handoff(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json decode error = %v", err)
	}
	ticketID, ok := payload["ticket_id"].(string)
	if !ok || ticketID == "" {
		t.Fatalf("ticket_id missing, got %v", payload["ticket_id"])
	}

	// Verify ticket was stored with correct fields
	tickets := svc.tickets.List()
	found := false
	for _, tk := range tickets {
		if tk.ID == ticketID {
			found = true
			if tk.SessionID != "telegram:u_ticket_create" {
				t.Fatalf("ticket session_id = %s, want telegram:u_ticket_create", tk.SessionID)
			}
			if tk.Status != ticket.StatusOpen {
				t.Fatalf("ticket status = %s, want open", tk.Status)
			}
			if tk.HandoffReason != "customer requested human" {
				t.Fatalf("handoff_reason = %s, want 'customer requested human'", tk.HandoffReason)
			}
			break
		}
	}
	if !found {
		t.Fatalf("ticket %s not found in store", ticketID)
	}

	// Verify handoff audit event was recorded
	events := auditRecorder.eventsOfType("handoff")
	if len(events) != 1 {
		t.Fatalf("handoff audit events = %d, want 1", len(events))
	}
	if events[0].TicketID != ticketID {
		t.Fatalf("audit ticket_id = %s, want %s", events[0].TicketID, ticketID)
	}
}
