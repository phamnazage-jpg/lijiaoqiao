package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	intentdomain "github.com/bridge/ai-customer-service/internal/domain/intent"
	"github.com/bridge/ai-customer-service/internal/domain/message"
	"github.com/bridge/ai-customer-service/internal/domain/platformevent"
	"github.com/bridge/ai-customer-service/internal/platformadapter"
	"github.com/bridge/ai-customer-service/internal/service/dialog"
	"github.com/bridge/ai-customer-service/internal/service/handoff"
)

type stubPlatformDialogProcessor struct {
	result *dialog.Result
	err    error
	msg    *message.UnifiedMessage
}

func (s *stubPlatformDialogProcessor) Process(_ context.Context, msg *message.UnifiedMessage) (*dialog.Result, error) {
	s.msg = msg
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

type stubPlatformEventWriter struct {
	events []platformevent.Event
	err    error
}

func (s *stubPlatformEventWriter) InsertPendingBatch(_ context.Context, events []platformevent.Event) error {
	s.events = append(s.events, events...)
	return s.err
}

func TestPlatformWebhookHandler_ShouldEnqueueMessageReceivedAndReplyGenerated(t *testing.T) {
	registry := platformadapter.NewRegistry(platformadapter.NewSub2APIAdapter())
	processor := &stubPlatformDialogProcessor{result: &dialog.Result{SessionID: "sess-1", Reply: "好的", Intent: &intentdomain.Result{Intent: intentdomain.IntentRefund, Confidence: 0.9}}}
	writer := &stubPlatformEventWriter{}
	handler := NewPlatformWebhookHandler(processor, registry, writer)

	body := `{"message_id":"m1","channel":"sub2api","open_id":"u1","content":"我要退款"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/platforms/sub2api/webhook", strings.NewReader(body))
	rr := httptest.NewRecorder()
	handler.Handle(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if processor.msg == nil || processor.msg.OpenID != "u1" {
		t.Fatalf("processor msg = %+v, want mapped message", processor.msg)
	}
	if !strings.Contains(rr.Body.String(), `"accepted":true`) {
		t.Fatalf("response body = %s, want accepted=true", rr.Body.String())
	}
	if len(writer.events) != 4 {
		t.Fatalf("events len = %d, want 4", len(writer.events))
	}
	if writer.events[0].EventType != platformevent.TypeMessageReceived {
		t.Fatalf("first event type = %s", writer.events[0].EventType)
	}
	if writer.events[len(writer.events)-1].EventType != platformevent.TypeReplyGenerated {
		t.Fatalf("last event type = %s", writer.events[len(writer.events)-1].EventType)
	}
}

func TestPlatformWebhookHandler_ShouldEnqueueHandoffAndTicketCreatedWhenNeeded(t *testing.T) {
	registry := platformadapter.NewRegistry(platformadapter.NewSub2APIAdapter())
	processor := &stubPlatformDialogProcessor{result: &dialog.Result{
		SessionID: "sess-1",
		Reply:     "已转人工",
		Intent:    &intentdomain.Result{Intent: intentdomain.IntentHandoff, Confidence: 0.88},
		Handoff:   &handoff.Decision{ShouldHandoff: true, Priority: "P1", Reason: "complaint"},
		TicketID:  "ticket-1",
	}}
	writer := &stubPlatformEventWriter{}
	handler := NewPlatformWebhookHandler(processor, registry, writer)

	body := `{"message_id":"m1","channel":"sub2api","open_id":"u1","content":"我要投诉"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/platforms/sub2api/webhook", strings.NewReader(body))
	rr := httptest.NewRecorder()
	handler.Handle(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if len(writer.events) != 6 {
		t.Fatalf("events len = %d, want 6", len(writer.events))
	}
	if writer.events[3].EventType != platformevent.TypeHandoffTriggered {
		t.Fatalf("handoff event type = %s", writer.events[3].EventType)
	}
	if writer.events[4].EventType != platformevent.TypeTicketCreated {
		t.Fatalf("ticket event type = %s", writer.events[4].EventType)
	}
}

func TestPlatformWebhookHandler_ShouldRejectUnknownPlatform(t *testing.T) {
	handler := NewPlatformWebhookHandler(&stubPlatformDialogProcessor{}, platformadapter.NewRegistry(platformadapter.NewSub2APIAdapter()), &stubPlatformEventWriter{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/platforms/unknown/webhook", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	handler.Handle(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}
