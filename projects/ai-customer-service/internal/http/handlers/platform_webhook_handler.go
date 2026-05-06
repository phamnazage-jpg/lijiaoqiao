package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/error/cserrors"
	"github.com/bridge/ai-customer-service/internal/domain/message"
	"github.com/bridge/ai-customer-service/internal/domain/platformevent"
	"github.com/bridge/ai-customer-service/internal/platformadapter"
	"github.com/bridge/ai-customer-service/internal/service/dialog"
	"github.com/bridge/ai-customer-service/internal/service/platformevents"
)

type PlatformDialogProcessor interface {
	Process(ctx context.Context, msg *message.UnifiedMessage) (*dialog.Result, error)
}

type PlatformEventWriter interface {
	InsertPendingBatch(ctx context.Context, events []platformevent.Event) error
}

type PlatformWebhookHandler struct {
	dialog      PlatformDialogProcessor
	registry    *platformadapter.Registry
	eventWriter PlatformEventWriter
	now         func() time.Time
}

func NewPlatformWebhookHandler(dialogProcessor PlatformDialogProcessor, registry *platformadapter.Registry, eventWriter PlatformEventWriter) *PlatformWebhookHandler {
	return &PlatformWebhookHandler{
		dialog:      dialogProcessor,
		registry:    registry,
		eventWriter: eventWriter,
		now:         time.Now,
	}
}

func (h *PlatformWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": map[string]any{"code": cserrors.CS_HTTP_405, "message": cserrors.ErrorMsg(cserrors.CS_HTTP_405)}})
		return
	}
	platform, channel, ok := parsePlatformWebhookPath(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "CS_PLATFORM_4040", "message": "platform webhook path not found"}})
		return
	}
	if platform == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": "CS_PLATFORM_4001", "message": "platform is required"}})
		return
	}
	if h.registry == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": cserrors.CS_SYS_5001, "message": cserrors.ErrorMsg(cserrors.CS_SYS_5001)}})
		return
	}
	now := h.now()
	adapter, ok := h.registry.Resolve(platform)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "CS_PLATFORM_4041", "message": "platform adapter not found"}})
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": cserrors.CS_REQ_4004, "message": cserrors.ErrorMsg(cserrors.CS_REQ_4004)}})
		return
	}
	msg, meta, err := adapter.ParseInbound(r, body, platformadapter.IngressContext{
		Platform:    platform,
		PathChannel: channel,
		ReceivedAt:  now,
	})
	if err != nil {
		var reqErr *platformadapter.RequestError
		if errors.As(err, &reqErr) {
			writeJSON(w, reqErr.Status, map[string]any{"error": map[string]any{"code": reqErr.Code, "message": reqErr.Message}})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": cserrors.CS_REQ_4001, "message": cserrors.ErrorMsg(cserrors.CS_REQ_4001)}})
		return
	}
	result, err := h.dialog.Process(r.Context(), msg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": cserrors.CS_SYS_5001, "message": cserrors.ErrorMsg(cserrors.CS_SYS_5001)}})
		return
	}
	if h.eventWriter == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": cserrors.CS_SYS_5001, "message": cserrors.ErrorMsg(cserrors.CS_SYS_5001)}})
		return
	}
	events, err := platformevents.BuildInboundEvents(msg, result, meta, now)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": cserrors.CS_SYS_5001, "message": cserrors.ErrorMsg(cserrors.CS_SYS_5001)}})
		return
	}
	if err := h.eventWriter.InsertPendingBatch(r.Context(), events); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": cserrors.CS_SYS_5001, "message": cserrors.ErrorMsg(cserrors.CS_SYS_5001)}})
		return
	}
	writeJSON(w, http.StatusOK, adapter.BuildIngressAck(result, meta))
}

func parsePlatformWebhookPath(path string) (platform string, channel string, ok bool) {
	const prefix = "/api/v1/customer-service/platforms/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	trimmed := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 || parts[1] != "webhook" {
		return "", "", false
	}
	platform = strings.TrimSpace(parts[0])
	if len(parts) > 2 {
		channel = strings.TrimSpace(strings.Join(parts[2:], "/"))
	}
	return platform, channel, true
}
