package platformevents

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/bridge/ai-customer-service/internal/domain/message"
	"github.com/bridge/ai-customer-service/internal/domain/platformevent"
	"github.com/bridge/ai-customer-service/internal/platformadapter"
	"github.com/bridge/ai-customer-service/internal/service/dialog"
)

const defaultCallbackTarget = "default"

func BuildInboundEvents(msg *message.UnifiedMessage, result *dialog.Result, meta *platformadapter.PlatformInboundMeta, now time.Time) ([]platformevent.Event, error) {
	if msg == nil {
		return nil, fmt.Errorf("message is nil")
	}
	if result == nil {
		return nil, fmt.Errorf("result is nil")
	}
	if meta == nil {
		return nil, fmt.Errorf("platform inbound meta is nil")
	}
	if now.IsZero() {
		now = time.Now()
	}

	callbackTarget := meta.CallbackTarget
	if callbackTarget == "" {
		callbackTarget = defaultCallbackTarget
	}
	eventIndex := 0
	baseEvent := func(eventType string, payload map[string]any) platformevent.Event {
		eventTime := now.Add(time.Duration(eventIndex) * time.Nanosecond)
		eventIndex++
		return platformevent.Event{
			ID:              uuid.New().String(),
			Platform:        meta.Platform,
			EventType:       eventType,
			SessionID:       result.SessionID,
			TicketID:        result.TicketID,
			SourceMessageID: meta.SourceMessageID,
			CallbackTarget:  callbackTarget,
			Payload:         payload,
			Status:          platformevent.StatusPending,
			AttemptCount:    0,
			NextAttemptAt:   eventTime,
			OccurredAt:      eventTime,
			CreatedAt:       eventTime,
			UpdatedAt:       eventTime,
		}
	}

	events := []platformevent.Event{
		baseEvent(platformevent.TypeMessageReceived, map[string]any{
			"channel":      meta.Channel,
			"open_id":      msg.OpenID,
			"user_id":      msg.UserID,
			"content":      msg.Content,
			"content_type": msg.ContentType,
			"reply_to":     msg.ReplyTo,
		}),
		baseEvent(platformevent.TypeMessageProcessing, map[string]any{
			"session_id": result.SessionID,
		}),
	}

	if result.Intent != nil {
		events = append(events, baseEvent(platformevent.TypeIntentResolved, map[string]any{
			"intent":     result.Intent.Intent,
			"confidence": result.Intent.Confidence,
		}))
	}
	if result.Handoff != nil && result.Handoff.ShouldHandoff {
		events = append(events, baseEvent(platformevent.TypeHandoffTriggered, map[string]any{
			"priority": result.Handoff.Priority,
			"reason":   result.Handoff.Reason,
		}))
	}
	if result.TicketID != "" {
		events = append(events, baseEvent(platformevent.TypeTicketCreated, map[string]any{
			"ticket_id": result.TicketID,
		}))
	}
	events = append(events, baseEvent(platformevent.TypeReplyGenerated, map[string]any{
		"reply": result.Reply,
	}))
	return events, nil
}
