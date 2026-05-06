package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPlatformWebhookSecurity_ShouldAcceptSignedSub2APIRequest(t *testing.T) {
	secured := PlatformWebhookSecurity{
		Sub2APISecret:   "sub2api-secret",
		TimestampHeader: "X-CS-Timestamp",
		SignatureHeader: "X-CS-Signature",
		MaxSkew:         5 * time.Minute,
	}
	handler := secured.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := []byte(`{"message_id":"m1","channel":"sub2api","open_id":"u1","content":"hello"}`)
	timestampStr := formatUnix(time.Now().Unix())
	signature := signBody("sub2api-secret", timestampStr, body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/platforms/sub2api/webhook", bytes.NewReader(body))
	req.Header.Set("X-CS-Timestamp", timestampStr)
	req.Header.Set("X-CS-Signature", signature)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
}

func TestPlatformWebhookSecurity_ShouldRejectInvalidSignatureForConfiguredPlatform(t *testing.T) {
	auditRecorder := &stubAuditRecorder{}
	secured := PlatformWebhookSecurity{
		Sub2APISecret:   "sub2api-secret",
		TimestampHeader: "X-CS-Timestamp",
		SignatureHeader: "X-CS-Signature",
		MaxSkew:         5 * time.Minute,
		Audit:           auditRecorder,
	}
	handler := secured.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := []byte(`{"message_id":"m1","channel":"sub2api","open_id":"u1","content":"hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/platforms/sub2api/webhook", bytes.NewReader(body))
	req.Header.Set("X-CS-Timestamp", formatUnix(time.Now().Unix()))
	req.Header.Set("X-CS-Signature", "wrong-signature")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.Code)
	}
	if len(auditRecorder.events) != 1 {
		t.Fatalf("audit count = %d, want 1", len(auditRecorder.events))
	}
}

func TestPlatformWebhookSecurity_ShouldBypassUnknownPlatform(t *testing.T) {
	hit := false
	secured := PlatformWebhookSecurity{
		Sub2APISecret: "sub2api-secret",
		MaxSkew:       5 * time.Minute,
	}
	handler := secured.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/platforms/unknown/webhook", bytes.NewBufferString(`{}`))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if !hit {
		t.Fatal("expected next handler to handle unknown platform")
	}
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.Code)
	}
}
