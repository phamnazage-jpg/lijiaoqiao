package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

)

// TestWebhookSecurity_InvalidTimestampFormat covers CS_AUTH_4032:
// strconv.ParseInt fails on non-numeric timestamp → 403.
func TestWebhookSecurity_InvalidTimestampFormat(t *testing.T) {
	auditRecorder := &stubAuditRecorder{}
	secured := WebhookSecurity{Secret: "secret", TimestampHeader: "X-CS-Timestamp", SignatureHeader: "X-CS-Signature", MaxSkew: 5 * time.Minute, Audit: auditRecorder}
	handler := secured.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
	req.Header.Set("X-CS-Timestamp", "not-a-number")
	req.Header.Set("X-CS-Signature", "abc123")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (invalid timestamp format)", resp.Code)
	}
	if len(auditRecorder.events) != 1 {
		t.Fatalf("audit count = %d, want 1", len(auditRecorder.events))
	}
	if auditRecorder.events[0].Type != "webhook_security_rejected" {
		t.Fatalf("audit type = %s", auditRecorder.events[0].Type)
	}
}

// TestWebhookSecurity_TimestampSkewTooLarge covers CS_AUTH_4033:
// timestamp is too old or too far in the future → 403.
func TestWebhookSecurity_TimestampSkewTooLarge(t *testing.T) {
	auditRecorder := &stubAuditRecorder{}
	secured := WebhookSecurity{Secret: "secret", TimestampHeader: "X-CS-Timestamp", SignatureHeader: "X-CS-Signature", MaxSkew: 5 * time.Minute, Audit: auditRecorder}
	handler := secured.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	// Timestamp 10 minutes ago → skew > 5 min MaxSkew
	oldTimestamp := time.Now().Add(-10 * time.Minute).Unix()
	body := []byte(`{}`)
	timestampStr := formatUnix(oldTimestamp)
	signature := signBody("secret", timestampStr, body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-CS-Timestamp", timestampStr)
	req.Header.Set("X-CS-Signature", signature)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (timestamp skew too large)", resp.Code)
	}
}

// TestWebhookSecurity_BodyReadError documents CS_REQ_4004 coverage gap:
// io.ReadAll error is not reachable in unit tests (httptest always provides a valid body reader).
// This test validates the handler does NOT panic on empty body with valid signature.
func TestWebhookSecurity_EmptyBodyWithValidSignature(t *testing.T) {
	secured := WebhookSecurity{Secret: "secret", TimestampHeader: "X-CS-Timestamp", SignatureHeader: "X-CS-Signature", MaxSkew: 5 * time.Minute}
	handler := secured.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	body := []byte(`{}`)
	timestampStr := formatUnix(time.Now().Unix())
	signature := signBody("secret", timestampStr, body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-CS-Timestamp", timestampStr)
	req.Header.Set("X-CS-Signature", signature)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	// Empty body {} with valid HMAC passes all security checks
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (valid signature on empty body)", resp.Code)
	}
}

// TestWebhookSecurity_InvalidSignature covers CS_AUTH_4034:
// HMAC signature mismatch → 403.
func TestWebhookSecurity_InvalidSignature(t *testing.T) {
	auditRecorder := &stubAuditRecorder{}
	secured := WebhookSecurity{Secret: "secret", TimestampHeader: "X-CS-Timestamp", SignatureHeader: "X-CS-Signature", MaxSkew: 5 * time.Minute, Audit: auditRecorder}
	handler := secured.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	body := []byte(`{"ok":true}`)
	timestampStr := formatUnix(time.Now().Unix())

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-CS-Timestamp", timestampStr)
	req.Header.Set("X-CS-Signature", "wrong-signature")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (invalid signature)", resp.Code)
	}
	if len(auditRecorder.events) != 1 {
		t.Fatalf("audit count = %d, want 1", len(auditRecorder.events))
	}
	if auditRecorder.events[0].Type != "webhook_security_rejected" {
		t.Fatalf("audit type = %s", auditRecorder.events[0].Type)
	}
}

// TestWebhookSecurity_EmptyTimestampAndSignature covers CS_AUTH_4031:
// both timestamp and signature missing → 403.
func TestWebhookSecurity_EmptyTimestampAndSignature(t *testing.T) {
	auditRecorder := &stubAuditRecorder{}
	secured := WebhookSecurity{Secret: "secret", TimestampHeader: "X-CS-Timestamp", SignatureHeader: "X-CS-Signature", MaxSkew: 5 * time.Minute, Audit: auditRecorder}
	handler := secured.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
	// Neither header set
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (missing timestamp+signature)", resp.Code)
	}
	if len(auditRecorder.events) != 1 {
		t.Fatalf("audit count = %d, want 1", len(auditRecorder.events))
	}
}

// TestWebhookSecurity_EmptySignatureOnly covers CS_AUTH_4031:
// signature missing but timestamp present → 403.
func TestWebhookSecurity_EmptySignatureOnly(t *testing.T) {
	secured := WebhookSecurity{Secret: "secret", TimestampHeader: "X-CS-Timestamp", SignatureHeader: "X-CS-Signature", MaxSkew: 5 * time.Minute}
	handler := secured.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
	req.Header.Set("X-CS-Timestamp", formatUnix(time.Now().Unix()))
	// signature header missing
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (signature missing)", resp.Code)
	}
}

// TestWebhookSecurity_EmptyTimestampOnly covers CS_AUTH_4031:
// timestamp missing but signature present → 403.
func TestWebhookSecurity_EmptyTimestampOnly(t *testing.T) {
	secured := WebhookSecurity{Secret: "secret", TimestampHeader: "X-CS-Timestamp", SignatureHeader: "X-CS-Signature", MaxSkew: 5 * time.Minute}
	handler := secured.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
	req.Header.Set("X-CS-Signature", "some-signature")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (timestamp missing)", resp.Code)
	}
}

// TestWebhookSecurity_NonPostMethod bypasses security check for non-POST methods.
func TestWebhookSecurity_NonPostMethod(t *testing.T) {
	secured := WebhookSecurity{Secret: "secret", MaxSkew: 5 * time.Minute}
	handler := secured.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET passthrough, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (non-POST passthrough)", resp.Code)
	}
}

// TestWebhookSecurity_DisabledWhenNoSecret verifies security middleware is
// a no-op when Secret is not configured.
func TestWebhookSecurity_DisabledWhenNoSecret(t *testing.T) {
	hit := false
	handler := WebhookSecurity{}.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if !hit {
		t.Fatalf("wrapped handler was not called when secret is empty")
	}
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (security disabled)", resp.Code)
	}
}

// --- helpers ---

func formatUnix(unix int64) string {
	return strconv.FormatInt(unix, 10)
}

func signBody(secret, timestamp string, body []byte) string {
	return computeWebhookSignature(secret, timestamp, body)
}

// stubAuditRecorder is defined in webhook_handler_test.go and reused here.
// This file is in the same package so it can access stubAuditRecorder directly.
