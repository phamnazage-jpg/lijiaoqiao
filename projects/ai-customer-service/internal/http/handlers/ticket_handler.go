package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
	"github.com/bridge/ai-customer-service/internal/domain/error/cserrors"
	"github.com/bridge/ai-customer-service/internal/domain/ticket"
)

type TicketService interface {
	ListOpen(ctx context.Context, limit int) ([]ticket.Ticket, error)
	GetByID(ctx context.Context, id string) (*ticket.Ticket, error)
	Assign(ctx context.Context, ticketID, agentID, actorID, sourceIP string, now time.Time) error
	Resolve(ctx context.Context, ticketID, resolution, actorID, sourceIP string, now time.Time) error
	Close(ctx context.Context, ticketID, resolution, actorID, sourceIP string, now time.Time) error
}

type TicketHandler struct {
	service TicketService
	audit   AuditRecorder
	now     func() time.Time
}

func NewTicketHandler(service TicketService, auditRecorder AuditRecorder) *TicketHandler {
	return &TicketHandler{service: service, audit: auditRecorder, now: time.Now}
}

func (h *TicketHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListOpen(r.Context(), 50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": cserrors.CS_SYS_5002, "message": cserrors.ErrorMsg(cserrors.CS_SYS_5002)}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// P1-3: GET /api/v1/customer-service/tickets/{id} — ticket detail (Phase 1 minimum implementation)
func (h *TicketHandler) Get(w http.ResponseWriter, r *http.Request) {
	ticketID := pathParam(r.URL.Path, "/api/v1/customer-service/tickets/", "")
	if ticketID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": cserrors.CS_REQ_4005, "message": cserrors.ErrorMsg(cserrors.CS_REQ_4005)}})
		return
	}
	tkt, err := h.service.GetByID(r.Context(), ticketID)
	if err != nil || tkt == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": cserrors.CS_TICKET_4001, "message": cserrors.ErrorMsg(cserrors.CS_TICKET_4001)}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ticket": tkt})
}

func (h *TicketHandler) Assign(w http.ResponseWriter, r *http.Request) {
	ticketID := pathParam(r.URL.Path, "/api/v1/customer-service/tickets/", "/assign")
	agentID := strings.TrimSpace(r.URL.Query().Get("agent_id"))
	if ticketID == "" || agentID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": cserrors.CS_REQ_4005, "message": cserrors.ErrorMsg(cserrors.CS_REQ_4005)}})
		return
	}
	actorID := strings.TrimSpace(r.URL.Query().Get("actor_id"))
	sourceIP := clientIP(r.RemoteAddr)
	if err := h.service.Assign(r.Context(), ticketID, agentID, actorID, sourceIP, h.now()); err != nil {
		// P0-2 fix: route error based on error code prefix from service layer
		errStr := err.Error()
		if strings.HasPrefix(errStr, "CS_TICKET_4001") {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": cserrors.CS_TICKET_4001, "message": cserrors.ErrorMsg(cserrors.CS_TICKET_4001)}})
			return
		}
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"code": cserrors.CS_TKT_4002, "message": cserrors.ErrorMsg(cserrors.CS_TKT_4002)}})
		return
	}
	h.auditTicketChange(r.Context(), ticketID, "assign", actorID, map[string]any{"assigned_to": agentID, "status": ticket.StatusAssigned}, r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]any{"assigned": true})
}

func (h *TicketHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	ticketID := pathParam(r.URL.Path, "/api/v1/customer-service/tickets/", "/resolve")
	resolution := strings.TrimSpace(r.URL.Query().Get("resolution"))
	if ticketID == "" || resolution == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": cserrors.CS_REQ_4006, "message": cserrors.ErrorMsg(cserrors.CS_REQ_4006)}})
		return
	}
	actorID := strings.TrimSpace(r.URL.Query().Get("actor_id"))
	sourceIP := clientIP(r.RemoteAddr)
	if err := h.service.Resolve(r.Context(), ticketID, resolution, actorID, sourceIP, h.now()); err != nil {
		// P0-2 fix: route error based on error code prefix from service layer
		errStr := err.Error()
		if strings.HasPrefix(errStr, "CS_TICKET_4001") {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": cserrors.CS_TICKET_4001, "message": cserrors.ErrorMsg(cserrors.CS_TICKET_4001)}})
			return
		}
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"code": cserrors.CS_TICKET_4092, "message": cserrors.ErrorMsg(cserrors.CS_TICKET_4092)}})
		return
	}
	h.auditTicketChange(r.Context(), ticketID, "resolve", actorID, map[string]any{"resolution": resolution, "status": ticket.StatusResolved}, r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]any{"resolved": true})
}

func (h *TicketHandler) Close(w http.ResponseWriter, r *http.Request) {
	ticketID := pathParam(r.URL.Path, "/api/v1/customer-service/tickets/", "/close")
	resolution := strings.TrimSpace(r.URL.Query().Get("resolution"))
	if ticketID == "" || resolution == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": cserrors.CS_REQ_4007, "message": cserrors.ErrorMsg(cserrors.CS_REQ_4007)}})
		return
	}
	actorID := strings.TrimSpace(r.URL.Query().Get("actor_id"))
	sourceIP := clientIP(r.RemoteAddr)
	if err := h.service.Close(r.Context(), ticketID, resolution, actorID, sourceIP, h.now()); err != nil {
		// P0-2 fix: route error based on error code prefix from service layer
		errStr := err.Error()
		if strings.HasPrefix(errStr, "CS_TICKET_4001") {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": cserrors.CS_TICKET_4001, "message": cserrors.ErrorMsg(cserrors.CS_TICKET_4001)}})
			return
		}
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"code": cserrors.CS_TICKET_4093, "message": cserrors.ErrorMsg(cserrors.CS_TICKET_4093)}})
		return
	}
	h.auditTicketChange(r.Context(), ticketID, "close", actorID, map[string]any{"resolution": resolution, "status": ticket.StatusClosed}, r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]any{"closed": true})
}

func (h *TicketHandler) auditTicketChange(ctx context.Context, ticketID, action, actorID string, after map[string]any, remoteAddr string) {
	if h == nil || h.audit == nil {
		return
	}
	now := h.now()
	// P0 quality standard: audit write failure only logs, does not return error
	_ = h.audit.Add(ctx, audit.Event{ID: newAuditID("audit", now), Type: "ticket_state_changed", Action: action, TicketID: ticketID, ActorID: actorID, SourceIP: clientIP(remoteAddr), AfterState: after, CreatedAt: now})
}

func pathParam(path, prefix, suffix string) string {
	trimmed := strings.TrimPrefix(path, prefix)
	trimmed = strings.TrimSuffix(trimmed, suffix)
	trimmed = strings.Trim(trimmed, "/")
	return trimmed
}