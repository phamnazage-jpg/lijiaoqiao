package platformadapter

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/message"
	"github.com/bridge/ai-customer-service/internal/service/dialog"
)

type Sub2APIAdapter struct{}

func NewSub2APIAdapter() *Sub2APIAdapter {
	return &Sub2APIAdapter{}
}

func (a *Sub2APIAdapter) Platform() string {
	return "sub2api"
}

func (a *Sub2APIAdapter) ParseInbound(_ *http.Request, body []byte, ctx IngressContext) (*message.UnifiedMessage, *PlatformInboundMeta, error) {
	var payload Sub2APIInboundPayload
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return nil, nil, NewRequestError(http.StatusBadRequest, "CS_REQ_4001", "invalid request body")
	}

	payload.Channel = strings.TrimSpace(payload.Channel)
	payload.OpenID = strings.TrimSpace(payload.OpenID)
	payload.UserID = strings.TrimSpace(payload.UserID)
	payload.Content = strings.TrimSpace(payload.Content)
	payload.ContentType = strings.TrimSpace(payload.ContentType)
	payload.ReplyTo = strings.TrimSpace(payload.ReplyTo)
	if ctx.PathChannel != "" {
		payload.Channel = strings.TrimSpace(ctx.PathChannel)
	}
	if payload.Channel == "" || payload.OpenID == "" || payload.Content == "" {
		return nil, nil, NewRequestError(http.StatusBadRequest, "CS_REQ_4002", "channel, open_id and content are required")
	}
	if payload.Timestamp.IsZero() {
		payload.Timestamp = ctx.ReceivedAt
	}

	msg := &message.UnifiedMessage{
		MessageID:   payload.MessageID,
		Channel:     payload.Channel,
		OpenID:      payload.OpenID,
		UserID:      payload.UserID,
		Content:     payload.Content,
		ContentType: payload.ContentType,
		Timestamp:   payload.Timestamp,
		ReplyTo:     payload.ReplyTo,
	}
	meta := &PlatformInboundMeta{
		EventID:         buildEventID("sub2api", payload.MessageID, payload.OpenID, ctx.ReceivedAt),
		Platform:        a.Platform(),
		Channel:         payload.Channel,
		SourceMessageID: payload.MessageID,
		SourceUserID:    payload.OpenID,
	}
	return msg, meta, nil
}

func (a *Sub2APIAdapter) BuildIngressAck(result *dialog.Result, meta *PlatformInboundMeta) any {
	resp := map[string]any{
		"accepted": true,
		"platform": a.Platform(),
	}
	if meta != nil {
		resp["event_id"] = meta.EventID
	}
	if result != nil {
		resp["session_id"] = result.SessionID
		if strings.TrimSpace(result.TicketID) != "" {
			resp["ticket_id"] = result.TicketID
		}
	}
	return resp
}

func buildEventID(platform, messageID, openID string, now time.Time) string {
	if strings.TrimSpace(messageID) != "" {
		return strings.ToLower(strings.TrimSpace(platform)) + ":" + strings.TrimSpace(messageID)
	}
	return strings.ToLower(strings.TrimSpace(platform)) + ":" + strings.TrimSpace(openID) + ":" + now.UTC().Format(time.RFC3339Nano)
}
