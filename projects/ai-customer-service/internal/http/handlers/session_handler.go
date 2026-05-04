package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
	"github.com/bridge/ai-customer-service/internal/domain/error/cserrors"
	"github.com/bridge/ai-customer-service/internal/domain/session"
	"github.com/bridge/ai-customer-service/internal/domain/ticket"
	"github.com/bridge/ai-customer-service/internal/http/middleware"
)

type SessionGetter interface {
	GetByID(ctx context.Context, id string) (*session.Session, error)
}

type TicketCreator interface {
	Create(ctx context.Context, t *ticket.Ticket) error
}

// SessionHandler handles session-related API endpoints: feedback and manual handoff.
type SessionHandler struct {
	sessions SessionGetter
	tickets  TicketCreator
	audits   AuditRecorder
	now      func() time.Time
}

// NewSessionHandler creates a new SessionHandler.
func NewSessionHandler(sessions SessionGetter, tickets TicketCreator, audits AuditRecorder) *SessionHandler {
	return &SessionHandler{
		sessions: sessions,
		tickets:  tickets,
		audits:   audits,
		now:      time.Now,
	}
}

// FeedbackRequest represents the feedback submission request body.
type FeedbackRequest struct {
	Score   int    `json:"score"`
	Comment string `json:"comment,omitempty"`
}

// Feedback handles POST /api/v1/customer-service/sessions/{id}/feedback
// Feedback is written directly to audit_log and does not update the session itself.
func (h *SessionHandler) Feedback(w http.ResponseWriter, r *http.Request) {
	sessionID := sessionPathParam(r.URL.Path)
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": cserrors.CS_REQ_4005, "message": cserrors.ErrorMsg(cserrors.CS_REQ_4005)}})
		return
	}

	var req FeedbackRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": cserrors.CS_REQ_4001, "message": cserrors.ErrorMsg(cserrors.CS_REQ_4001)}})
		return
	}

	// Validate score range (1-5)
	if req.Score < 1 || req.Score > 5 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": cserrors.CS_REQ_4009, "message": cserrors.ErrorMsg(cserrors.CS_REQ_4009)}})
		return
	}

	actorID := "system"
	if actor, ok := middleware.ActorFromContext(r.Context()); ok {
		actorID = actor.ID
	}
	sourceIP := clientIP(r.RemoteAddr)
	now := h.now()

	// Write feedback to audit log (P0 quality standard: audit failure only logs, does not return error)
	feedbackPayload := map[string]any{
		"score":   req.Score,
		"comment": req.Comment,
	}
	_ = h.audits.Add(r.Context(), audit.Event{
		ID:        newAuditID("feedback", now),
		SessionID: sessionID,
		Type:      "feedback",
		Action:    "submit",
		ActorID:   actorID,
		SourceIP:  sourceIP,
		Payload:   feedbackPayload,
		CreatedAt: now,
	})

	writeJSON(w, http.StatusOK, map[string]any{"session_id": sessionID, "submitted": true})
}

// HandoffRequest represents the manual handoff request body.
type HandoffRequest struct {
	Reason   string `json:"reason"`
	Priority string `json:"priority,omitempty"`
}

// Handoff handles POST /api/v1/customer-service/sessions/{id}/handoff
// This is a客服后台主动发起的 manual handoff, not triggered by intent recognition.
func (h *SessionHandler) Handoff(w http.ResponseWriter, r *http.Request) {
	sessionID := sessionPathParam(r.URL.Path)
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": cserrors.CS_REQ_4005, "message": cserrors.ErrorMsg(cserrors.CS_REQ_4005)}})
		return
	}

	var req HandoffRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": cserrors.CS_REQ_4001, "message": cserrors.ErrorMsg(cserrors.CS_REQ_4001)}})
		return
	}

	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": cserrors.CS_REQ_4010, "message": cserrors.ErrorMsg(cserrors.CS_REQ_4010)}})
		return
	}

	// Verify session exists
	sess, err := h.sessions.GetByID(r.Context(), sessionID)
	if err != nil || sess == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": cserrors.CS_SES_4001, "message": cserrors.ErrorMsg(cserrors.CS_SES_4001)}})
		return
	}

	// Determine priority
	priority := ticket.Priority(strings.ToUpper(req.Priority))
	if priority == "" {
		priority = ticket.PriorityP2
	}

	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"code": cserrors.CS_AUTH_4001, "message": cserrors.ErrorMsg(cserrors.CS_AUTH_4001)}})
		return
	}
	actorID := actor.ID
	sourceIP := clientIP(r.RemoteAddr)
	now := h.now()

	// Create ticket for manual handoff
	ticketID := fmt.Sprintf("%s-%d", sessionID, now.UnixNano())
	tkt := &ticket.Ticket{
		ID:            ticketID,
		SessionID:     sessionID,
		UserID:        sess.UserID,
		Priority:      priority,
		Status:        ticket.StatusOpen,
		HandoffReason: req.Reason,
		ContextSnapshot: map[string]any{
			"channel":  sess.Channel,
			"open_id":  sess.OpenID,
			"manual":   true,
			"actor_id": actorID,
			"source":   "customer_service_api",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.tickets.Create(r.Context(), tkt); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": cserrors.CS_SYS_5002, "message": cserrors.ErrorMsg(cserrors.CS_SYS_5002)}})
		return
	}

	// Audit the manual handoff (P0 quality standard: audit failure only logs, does not return error)
	_ = h.audits.Add(r.Context(), audit.Event{
		ID:         newAuditID("handoff", now),
		SessionID:  sessionID,
		TicketID:   ticketID,
		Type:       "manual_handoff",
		Action:     "create",
		ActorID:    actorID,
		SourceIP:   sourceIP,
		AfterState: map[string]any{"ticket_id": ticketID, "priority": string(priority), "reason": req.Reason},
		CreatedAt:  now,
	})

	writeJSON(w, http.StatusOK, map[string]any{"session_id": sessionID, "ticket_id": ticketID, "priority": string(priority)})
}

// sessionPathParam extracts the session ID from paths like
// /api/v1/customer-service/sessions/{id}/feedback or .../handoff
func sessionPathParam(path string) string {
	prefix := "/api/v1/customer-service/sessions/"
	trimmed := strings.TrimPrefix(path, prefix)
	// Only accept paths ending in /feedback or /handoff
	if !strings.HasSuffix(trimmed, "/feedback") && !strings.HasSuffix(trimmed, "/handoff") {
		return ""
	}
	// Remove trailing /feedback or /handoff
	trimmed = strings.TrimSuffix(trimmed, "/feedback")
	trimmed = strings.TrimSuffix(trimmed, "/handoff")
	trimmed = strings.Trim(trimmed, "/")
	return trimmed
}
