package platformadapter

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSub2APIAdapter_ShouldMapMinimalPayload(t *testing.T) {
	adapter := NewSub2APIAdapter()
	body := `{"message_id":"m1","channel":"sub2api","open_id":"u1","content":"我要退款"}`
	req := httptest.NewRequest("POST", "/api/v1/customer-service/platforms/sub2api/webhook", strings.NewReader(body))
	msg, meta, err := adapter.ParseInbound(req, []byte(body), IngressContext{Platform: "sub2api", ReceivedAt: time.Unix(100, 0)})
	if err != nil {
		t.Fatalf("ParseInbound() error = %v", err)
	}
	if msg.Channel != "sub2api" || msg.OpenID != "u1" || msg.Content != "我要退款" {
		t.Fatalf("unexpected unified message: %+v", msg)
	}
	if meta == nil || meta.SourceMessageID != "m1" || meta.Platform != "sub2api" {
		t.Fatalf("unexpected meta: %+v", meta)
	}
}

func TestSub2APIAdapter_ShouldRejectUnknownEnvelopeFields(t *testing.T) {
	adapter := NewSub2APIAdapter()
	body := `{"message_id":"m1","channel":"sub2api","open_id":"u1","content":"hi","unknown":1}`
	req := httptest.NewRequest("POST", "/api/v1/customer-service/platforms/sub2api/webhook", strings.NewReader(body))
	_, _, err := adapter.ParseInbound(req, []byte(body), IngressContext{Platform: "sub2api", ReceivedAt: time.Now()})
	if err == nil {
		t.Fatal("expected error for unknown fields")
	}
}

func TestSub2APIAdapter_ShouldUseChannelOverrideWhenPresent(t *testing.T) {
	adapter := NewSub2APIAdapter()
	body := `{"message_id":"m1","channel":"body-channel","open_id":"u1","content":"hi"}`
	req := httptest.NewRequest("POST", "/api/v1/customer-service/platforms/sub2api/webhook/widget", strings.NewReader(body))
	msg, _, err := adapter.ParseInbound(req, []byte(body), IngressContext{Platform: "sub2api", PathChannel: "widget", ReceivedAt: time.Now()})
	if err != nil {
		t.Fatalf("ParseInbound() error = %v", err)
	}
	if msg.Channel != "widget" {
		t.Fatalf("msg.Channel = %q, want widget", msg.Channel)
	}
}

func TestSub2APIAdapter_ShouldRequireOpenIDAndContent(t *testing.T) {
	adapter := NewSub2APIAdapter()
	body := `{"message_id":"m1","channel":"sub2api","open_id":"","content":""}`
	req := httptest.NewRequest("POST", "/api/v1/customer-service/platforms/sub2api/webhook", strings.NewReader(body))
	_, _, err := adapter.ParseInbound(req, []byte(body), IngressContext{Platform: "sub2api", ReceivedAt: time.Now()})
	if err == nil {
		t.Fatal("expected validation error")
	}
	reqErr, ok := err.(*RequestError)
	if !ok {
		t.Fatalf("error type = %T, want *RequestError", err)
	}
	if reqErr.Status != 400 {
		t.Fatalf("reqErr.Status = %d, want 400", reqErr.Status)
	}
}
