package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
	"github.com/bridge/ai-customer-service/internal/service/dialog"
	"github.com/bridge/ai-customer-service/internal/service/handoff"
	intentservice "github.com/bridge/ai-customer-service/internal/service/intent"
	"github.com/bridge/ai-customer-service/internal/service/reply"
	"github.com/bridge/ai-customer-service/internal/store/memory"
	"log/slog"
)

type stubAuditRecorder struct {
	events []audit.Event
}

func (s *stubAuditRecorder) Add(_ context.Context, event audit.Event) error {
	s.events = append(s.events, event)
	return nil
}

func newTestWebhookHandler(auditRecorder AuditRecorder) *WebhookHandler {
	sessions := memory.NewSessionStore()
	audits := memory.NewAuditStore()
	tickets := memory.NewTicketStore()
	dedup := memory.NewDedupStore()
	knowledge := memory.NewKnowledgeStore()
	dialogSvc := dialog.NewService(sessions, audits, tickets, dedup, intentservice.NewService(), reply.NewService(knowledge), handoff.NewService())
	return NewWebhookHandler(dialogSvc, slog.Default(), auditRecorder)
}

func TestWebhookTruncatesLongContent(t *testing.T) {
	h := newTestWebhookHandler(nil)
	longContent := string(bytes.Repeat([]byte("a"), 2001))
	payload := `{"message_id":"m1","channel":"widget","open_id":"u1","content":"` + longContent + `"}`
	resp := httptest.NewRecorder()
	h.Handle(resp, httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook", bytes.NewBufferString(payload)))
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (truncate, not reject)", resp.Code)
	}
}

func TestWebhookRejectsUnknownFields(t *testing.T) {
	h := newTestWebhookHandler(nil)
	resp := httptest.NewRecorder()
	h.Handle(resp, httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook", bytes.NewBufferString(`{"message_id":"m1","channel":"widget","open_id":"u1","content":"hi","unknown":1}`)))
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestWebhookRejectsAndAuditsMissingFields(t *testing.T) {
	auditRecorder := &stubAuditRecorder{}
	h := newTestWebhookHandler(auditRecorder)
	resp := httptest.NewRecorder()
	h.Handle(resp, httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook", bytes.NewBufferString(`{"message_id":"m1"}`)))
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
	if len(auditRecorder.events) != 1 {
		t.Fatalf("audit count = %d, want 1", len(auditRecorder.events))
	}
	if auditRecorder.events[0].Type != "webhook_rejected" {
		t.Fatalf("audit type = %s", auditRecorder.events[0].Type)
	}
}

func TestWebhookSecurityRejectsMissingSignature(t *testing.T) {
	auditRecorder := &stubAuditRecorder{}
	secured := WebhookSecurity{Secret: "secret", TimestampHeader: "X-CS-Timestamp", SignatureHeader: "X-CS-Signature", MaxSkew: 5 * time.Minute, Audit: auditRecorder}
	handler := secured.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook", bytes.NewBufferString(`{"ok":true}`)))
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.Code)
	}
	if len(auditRecorder.events) != 1 {
		t.Fatalf("audit count = %d, want 1", len(auditRecorder.events))
	}
}

func TestWebhookSecurityAcceptsSignedRequest(t *testing.T) {
	secret := "secret"
	body := []byte(`{"ok":true}`)
	timestamp, signature, err := SignWebhookRequest(secret, time.Now().Unix(), body)
	if err != nil {
		t.Fatalf("SignWebhookRequest() error = %v", err)
	}
	secured := WebhookSecurity{Secret: secret, TimestampHeader: "X-CS-Timestamp", SignatureHeader: "X-CS-Signature", MaxSkew: 5 * time.Minute}
	hit := false
	handler := secured.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook", bytes.NewReader(body))
	req.Header.Set("X-CS-Timestamp", timestamp)
	req.Header.Set("X-CS-Signature", signature)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if !hit {
		t.Fatalf("expected wrapped handler to be called")
	}
}

func TestHandleChannel_OverridesChannel(t *testing.T) {
	h := newTestWebhookHandler(nil)
	payload := `{"message_id":"m1","channel":"original","open_id":"u1","content":"hello"}`
	resp := httptest.NewRecorder()
	h.HandleChannel(resp, httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook/widget", bytes.NewBufferString(payload)), "widget")
	if resp.Code != http.StatusOK {
		t.Fatalf("HandleChannel status = %d, want 200", resp.Code)
	}
}

func TestHandleChannel_WithEmptyOverride(t *testing.T) {
	h := newTestWebhookHandler(nil)
	payload := `{"message_id":"m1","channel":"web","open_id":"u1","content":"hello"}`
	resp := httptest.NewRecorder()
	h.HandleChannel(resp, httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook/", bytes.NewBufferString(payload)), "")
	if resp.Code != http.StatusOK {
		t.Fatalf("HandleChannel status = %d, want 200", resp.Code)
	}
}

func TestHandleChannel_RejectsNonPost(t *testing.T) {
	h := newTestWebhookHandler(&stubAuditRecorder{})
	payload := `{"message_id":"m1","channel":"widget","open_id":"u1","content":"hello"}`
	resp := httptest.NewRecorder()
	h.HandleChannel(resp, httptest.NewRequest(http.MethodGet, "/api/v1/customer-service/webhook/widget", bytes.NewBufferString(payload)), "widget")
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("HandleChannel GET status = %d, want 405", resp.Code)
	}
}

func TestHandleChannel_RejectsMissingFields(t *testing.T) {
	h := newTestWebhookHandler(&stubAuditRecorder{})
	payload := `{"message_id":"m1"}`
	resp := httptest.NewRecorder()
	h.HandleChannel(resp, httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook/widget", bytes.NewBufferString(payload)), "widget")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("HandleChannel status = %d, want 400", resp.Code)
	}
}

func TestHandleChannel_EmptyBody(t *testing.T) {
	h := newTestWebhookHandler(&stubAuditRecorder{})
	resp := httptest.NewRecorder()
	h.HandleChannel(resp, httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook/widget", bytes.NewBufferString(``)), "widget")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("HandleChannel empty body status = %d, want 400", resp.Code)
	}
}

func TestClientIP_WithPort(t *testing.T) {
	ip := clientIP("192.168.1.100:12345")
	if ip != "192.168.1.100" {
		t.Errorf("clientIP() = %s, want 192.168.1.100", ip)
	}
}

func TestClientIP_NoPort(t *testing.T) {
	ip := clientIP("192.168.1.100")
	if ip != "192.168.1.100" {
		t.Errorf("clientIP() = %s, want 192.168.1.100", ip)
	}
}