package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
	"github.com/bridge/ai-customer-service/internal/domain/error/cserrors"
	"github.com/bridge/ai-customer-service/internal/domain/ticketstats"
)

// TicketStatsService aggregates ticket statistics from the store.
type TicketStatsService interface {
	GetStats(ctx context.Context) (ticketstats.Stats, error)
}

type TicketStatsHandler struct {
	stats TicketStatsService
	audit AuditRecorder
	now   func() time.Time
}

func NewTicketStatsHandler(stats TicketStatsService, auditRecorder AuditRecorder) *TicketStatsHandler {
	return &TicketStatsHandler{stats: stats, audit: auditRecorder, now: time.Now}
}

// Get handles GET /api/v1/customer-service/tickets/stats
func (h *TicketStatsHandler) Get(w http.ResponseWriter, r *http.Request) {
	stats, err := h.stats.GetStats(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": cserrors.CS_SYS_5002, "message": cserrors.ErrorMsg(cserrors.CS_SYS_5002)}})
		return
	}
	// Audit access; failure does not block the response
	h.recordStatsAccess(r.Context(), r.RemoteAddr)
	writeJSON(w, http.StatusOK, stats)
}

// recordStatsAccess writes an audit log for stats access.
// Failures are logged but do not propagate.
func (h *TicketStatsHandler) recordStatsAccess(ctx context.Context, remoteAddr string) {
	if h == nil || h.audit == nil {
		return
	}
	now := h.now()
	// P0 quality standard: audit write failure only logs, does not return error
	_ = h.audit.Add(ctx, audit.Event{
		ID:       newAuditID("audit", now),
		Type:     "ticket_stats_accessed",
		Action:   "ticket_stats_accessed",
		ActorID:  "system",
		SourceIP: clientIP(remoteAddr),
		AfterState: map[string]any{
			"stats_accessed_at": now.Format(time.RFC3339),
		},
		CreatedAt: now,
	})
}
