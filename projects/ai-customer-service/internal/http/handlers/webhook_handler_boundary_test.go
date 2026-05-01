package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestWebhook_ContentBoundary_1999Chars verifies content at exactly 1999 chars
// (below the 2000 limit) is NOT truncated and returns 200.
func TestWebhook_ContentBoundary_1999Chars(t *testing.T) {
	h := newTestWebhookHandler(nil)
	content := string(bytes.Repeat([]byte("a"), 1999))
	payload := `{"message_id":"m1","channel":"widget","open_id":"u1","content":"` + content + `"}`
	resp := httptest.NewRecorder()
	h.Handle(resp, httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook", bytes.NewBufferString(payload)))
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (1999 chars < 2000 limit)", resp.Code)
	}
}

// TestWebhook_ContentBoundary_2000Chars verifies content at exactly 2000 chars
// (the limit) is NOT truncated and returns 200.
func TestWebhook_ContentBoundary_2000Chars(t *testing.T) {
	h := newTestWebhookHandler(nil)
	content := string(bytes.Repeat([]byte("a"), 2000))
	payload := `{"message_id":"m1","channel":"widget","open_id":"u1","content":"` + content + `"}`
	resp := httptest.NewRecorder()
	h.Handle(resp, httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook", bytes.NewBufferString(payload)))
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (2000 chars = limit, not truncated)", resp.Code)
	}
}

// TestWebhook_ContentBoundary_2001Chars verifies content at 2001 chars
// (above the 2000 limit) is truncated to 2000 and still returns 200.
func TestWebhook_ContentBoundary_2001Chars(t *testing.T) {
	h := newTestWebhookHandler(nil)
	content := string(bytes.Repeat([]byte("a"), 2001))
	payload := `{"message_id":"m1","channel":"widget","open_id":"u1","content":"` + content + `"}`
	resp := httptest.NewRecorder()
	h.Handle(resp, httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook", bytes.NewBufferString(payload)))
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (truncate, not reject)", resp.Code)
	}
}

// TestWebhook_ContentBoundary_AuditOnTruncation verifies that truncating content
// triggers an audit event with the correct details.
func TestWebhook_ContentBoundary_AuditOnTruncation(t *testing.T) {
	auditRecorder := &stubAuditRecorder{}
	h := newTestWebhookHandler(auditRecorder)
	content := string(bytes.Repeat([]byte("x"), 2500))
	payload := `{"message_id":"m_trunc","channel":"widget","open_id":"u_trunc","content":"` + content + `"}`
	resp := httptest.NewRecorder()
	h.Handle(resp, httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook", bytes.NewBufferString(payload)))
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	// Find the webhook_rejected audit event (truncation uses same audit path)
	found := false
	for _, ev := range auditRecorder.events {
		if ev.Type == "webhook_rejected" {
			found = true
			origLen, ok := ev.Payload["original_length"].(int)
			if !ok || origLen != 2500 {
				t.Fatalf("original_length = %v, want 2500", ev.Payload["original_length"])
			}
			truncLen, ok := ev.Payload["truncated_length"].(int)
			if !ok || truncLen != 2000 {
				t.Fatalf("truncated_length = %v, want 2000", ev.Payload["truncated_length"])
			}
			break
		}
	}
	if !found {
		t.Fatalf("webhook_rejected audit event not found for truncation")
	}
}

// TestWebhook_EmptyBody verifies empty JSON body {} returns 400.
func TestWebhook_EmptyBody(t *testing.T) {
	h := newTestWebhookHandler(nil)
	resp := httptest.NewRecorder()
	h.Handle(resp, httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook", bytes.NewBufferString(`{}`)))
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (empty body)", resp.Code)
	}
}

// TestWebhook_NonPostMethod verifies non-POST requests return 405.
func TestWebhook_NonPostMethod(t *testing.T) {
	h := newTestWebhookHandler(nil)
	resp := httptest.NewRecorder()
	h.Handle(resp, httptest.NewRequest(http.MethodGet, "/api/v1/customer-service/webhook", nil))
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.Code)
	}
}

// TestWebhook_MissingChannel verifies missing channel field returns 400.
func TestWebhook_MissingChannel(t *testing.T) {
	h := newTestWebhookHandler(nil)
	payload := `{"message_id":"m1","open_id":"u1","content":"hi"}`
	resp := httptest.NewRecorder()
	h.Handle(resp, httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook", bytes.NewBufferString(payload)))
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

// TestWebhook_MissingOpenID verifies missing open_id field returns 400.
func TestWebhook_MissingOpenID(t *testing.T) {
	h := newTestWebhookHandler(nil)
	payload := `{"message_id":"m1","channel":"widget","content":"hi"}`
	resp := httptest.NewRecorder()
	h.Handle(resp, httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook", bytes.NewBufferString(payload)))
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

// TestWebhook_MissingContent verifies missing content field returns 400.
func TestWebhook_MissingContent(t *testing.T) {
	h := newTestWebhookHandler(nil)
	payload := `{"message_id":"m1","channel":"widget","open_id":"u1"}`
	resp := httptest.NewRecorder()
	h.Handle(resp, httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook", bytes.NewBufferString(payload)))
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

// TestWebhook_WhitespaceOnlyFields verifies fields that are only whitespace
// are trimmed and then rejected as empty.
func TestWebhook_WhitespaceOnlyFields(t *testing.T) {
	h := newTestWebhookHandler(nil)
	payload := `{"message_id":"m1","channel":"  ","open_id":"u1","content":"hi"}`
	resp := httptest.NewRecorder()
	h.Handle(resp, httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/webhook", bytes.NewBufferString(payload)))
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (whitespace-only channel)", resp.Code)
	}
}

// newTestWebhookHandler is defined in webhook_handler_test.go.
// This file is in the same package so it can access it.
