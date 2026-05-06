package platformevents

import (
	"testing"
	"time"

	intentdomain "github.com/bridge/ai-customer-service/internal/domain/intent"
	"github.com/bridge/ai-customer-service/internal/domain/message"
	"github.com/bridge/ai-customer-service/internal/platformadapter"
	"github.com/bridge/ai-customer-service/internal/service/dialog"
	"github.com/bridge/ai-customer-service/internal/service/handoff"
)

func TestBuildInboundEvents_ShouldBuildReplyFlowEvents(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	events, err := BuildInboundEvents(
		&message.UnifiedMessage{OpenID: "u1", Content: "我要退款", ContentType: "text"},
		&dialog.Result{
			SessionID: "sess-1",
			Reply:     "好的",
			Intent:    &intentdomain.Result{Intent: intentdomain.IntentRefund, Confidence: 0.92},
		},
		&platformadapter.PlatformInboundMeta{
			Platform:        "sub2api",
			Channel:         "sub2api",
			SourceMessageID: "m1",
			CallbackTarget:  "default",
		},
		now,
	)
	if err != nil {
		t.Fatalf("BuildInboundEvents() error = %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("events len = %d, want 4", len(events))
	}
	if events[0].EventType != "message.received" {
		t.Fatalf("first event type = %s", events[0].EventType)
	}
	if events[len(events)-1].EventType != "reply.generated" {
		t.Fatalf("last event type = %s", events[len(events)-1].EventType)
	}
}

func TestBuildInboundEvents_ShouldIncludeHandoffAndTicketCreated(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	events, err := BuildInboundEvents(
		&message.UnifiedMessage{OpenID: "u1", Content: "我要投诉"},
		&dialog.Result{
			SessionID: "sess-1",
			Reply:     "已转人工",
			Intent:    &intentdomain.Result{Intent: intentdomain.IntentHandoff, Confidence: 0.88},
			Handoff:   &handoff.Decision{ShouldHandoff: true, Priority: "P1", Reason: "complaint"},
			TicketID:  "ticket-1",
		},
		&platformadapter.PlatformInboundMeta{
			Platform:        "sub2api",
			Channel:         "sub2api",
			SourceMessageID: "m1",
		},
		now,
	)
	if err != nil {
		t.Fatalf("BuildInboundEvents() error = %v", err)
	}
	if len(events) != 6 {
		t.Fatalf("events len = %d, want 6", len(events))
	}
	if events[3].EventType != "handoff.triggered" {
		t.Fatalf("handoff event type = %s", events[3].EventType)
	}
	if events[4].EventType != "ticket.created" {
		t.Fatalf("ticket event type = %s", events[4].EventType)
	}
	if events[0].CallbackTarget != "default" {
		t.Fatalf("callback target = %s, want default", events[0].CallbackTarget)
	}
}
