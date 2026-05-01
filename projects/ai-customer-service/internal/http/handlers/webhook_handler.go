package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
	"github.com/bridge/ai-customer-service/internal/domain/error/cserrors"
	"github.com/bridge/ai-customer-service/internal/domain/message"
	"github.com/bridge/ai-customer-service/internal/service/dialog"
)

const maxContentLen = 2000

type WebhookHandler struct {
	dialog *dialog.Service
	logger *slog.Logger
	audit  AuditRecorder
}

func NewWebhookHandler(dialog *dialog.Service, logger *slog.Logger, auditRecorder AuditRecorder) *WebhookHandler {
	return &WebhookHandler{dialog: dialog, logger: logger, audit: auditRecorder}
}

func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	h.handle(w, r, "")
}

// HandleChannel accepts a channel from the URL path ({channel}), which overrides
// the channel in the request body when present.
func (h *WebhookHandler) HandleChannel(w http.ResponseWriter, r *http.Request, channel string) {
	h.handle(w, r, strings.TrimSpace(channel))
}

func (h *WebhookHandler) handle(w http.ResponseWriter, r *http.Request, channelOverride string) {
	if r.Method != http.MethodPost {
		h.auditRejectedRequest(r.Context(), r, cserrors.CS_HTTP_405, cserrors.ErrorMsg(cserrors.CS_HTTP_405), map[string]any{"method": r.Method})
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": cserrors.CS_HTTP_405, "message": cserrors.ErrorMsg(cserrors.CS_HTTP_405)}})
		return
	}

	var msg message.UnifiedMessage
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&msg); err != nil {
		status := http.StatusBadRequest
		code := cserrors.CS_REQ_4001
		messageText := cserrors.ErrorMsg(cserrors.CS_REQ_4001)
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			code = cserrors.CS_REQ_4131
			status = http.StatusRequestEntityTooLarge
			messageText = cserrors.ErrorMsg(cserrors.CS_REQ_4131)
		} else if errors.Is(err, io.EOF) {
			messageText = "empty body"
		}
		h.auditRejectedRequest(r.Context(), r, code, messageText, map[string]any{"decode_error": err.Error()})
		writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": messageText}})
		return
	}

	msg.Channel = strings.TrimSpace(msg.Channel)
	msg.OpenID = strings.TrimSpace(msg.OpenID)
	msg.Content = strings.TrimSpace(msg.Content)
	if channelOverride != "" {
		msg.Channel = channelOverride
	}
	if msg.Channel == "" || msg.OpenID == "" || msg.Content == "" {
		h.auditRejectedRequest(r.Context(), r, cserrors.CS_REQ_4002, cserrors.ErrorMsg(cserrors.CS_REQ_4002), map[string]any{"channel": msg.Channel, "open_id": msg.OpenID})
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": cserrors.CS_REQ_4002, "message": cserrors.ErrorMsg(cserrors.CS_REQ_4002)}})
		return
	}

	// P0-1: truncate content > 2000 chars (do not reject), audit the truncation
	if len(msg.Content) > maxContentLen {
		h.auditRejectedRequest(r.Context(), r, cserrors.CS_REQ_4003, "content truncated", map[string]any{"channel": msg.Channel, "open_id": msg.OpenID, "original_length": len(msg.Content), "truncated_length": maxContentLen})
		msg.Content = msg.Content[:maxContentLen]
	}

	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	result, err := h.dialog.Process(r.Context(), &msg)
	if err != nil {
		if h.logger != nil {
			h.logger.Error("webhook process failed", "channel", msg.Channel, "open_id", msg.OpenID, "message_id", msg.MessageID, "error", err.Error())
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": cserrors.CS_SYS_5001, "message": cserrors.ErrorMsg(cserrors.CS_SYS_5001)}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"received": true, "session_id": result.SessionID, "reply": result.Reply, "intent": result.Intent.Intent, "handoff": result.Handoff.ShouldHandoff, "ticket_id": result.TicketID})
}

func (h *WebhookHandler) auditRejectedRequest(ctx context.Context, r *http.Request, code, messageText string, details map[string]any) {
	if h == nil || h.audit == nil {
		return
	}
	now := time.Now()
	payload := map[string]any{"error_code": code, "message": messageText, "path": r.URL.Path, "remote_addr": r.RemoteAddr}
	for k, v := range details {
		payload[k] = v
	}
	// P0 quality standard: audit write failure only logs, does not return error
	_ = h.audit.Add(ctx, audit.Event{ID: newAuditID("audit", now), Type: "webhook_rejected", Action: "reject", ActorID: "system", SourceIP: clientIP(r.RemoteAddr), Payload: payload, CreatedAt: now})
}

func clientIP(remoteAddr string) string {
	if idx := strings.LastIndex(remoteAddr, ":"); idx > 0 {
		return remoteAddr[:idx]
	}
	return remoteAddr
}
