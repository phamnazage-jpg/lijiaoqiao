package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/app"
	"github.com/bridge/ai-customer-service/internal/config"
	"github.com/bridge/ai-customer-service/internal/http/handlers"
	"github.com/bridge/ai-customer-service/internal/platform/logging"
)

func newTestAppWithSecret(t *testing.T) *app.App {
	t.Helper()
	cfg := &config.Config{}
	cfg.HTTP.Addr = ":0"
	cfg.HTTP.ReadHeaderTimeout = 5
	cfg.HTTP.ReadTimeout = 10
	cfg.HTTP.WriteTimeout = 15
	cfg.HTTP.IdleTimeout = 60
	cfg.HTTP.MaxHeaderBytes = 1 << 20
	cfg.HTTP.MaxBodyBytes = 1 << 20
	cfg.Webhook.Secret = "e2e-test-secret"
	cfg.Webhook.TimestampHeader = "X-CS-Timestamp"
	cfg.Webhook.SignatureHeader = "X-CS-Signature"
	cfg.Webhook.MaxSkewSeconds = 300
	application, err := app.New(cfg, logging.New())
	if err != nil {
		t.Fatalf("app.New() error = %v", err)
	}
	return application
}

// TestSecurity_InvalidSignature verifies that a request with a wrong signature
// is rejected with 403 and error code CS_AUTH_4034.
func TestSecurity_InvalidSignature(t *testing.T) {
	application := newTestAppWithSecret(t)
	server := httptest.NewServer(application.Server.Handler)
	defer server.Close()

	body := []byte(`{"message_id":"m-sec-1","channel":"widget","open_id":"u_sec","content":"查询额度"}`)
	timestamp, _, err := handlers.SignWebhookRequest("e2e-test-secret", time.Now().Unix(), body)
	if err != nil {
		t.Fatalf("SignWebhookRequest error = %v", err)
	}

	// Use a deliberately wrong signature value
	wrongSig := "deadbeefcafebabe0000000000000000000000000000000000000000000000"
	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/customer-service/webhook", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CS-Timestamp", timestamp)
	req.Header.Set("X-CS-Signature", wrongSig)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}

	bodyOut, _ := io.ReadAll(resp.Body)
	var errPayload map[string]any
	if err := json.Unmarshal(bodyOut, &errPayload); err != nil {
		t.Fatalf("decode error response error = %v", err)
	}
	errObj := errPayload["error"].(map[string]any)
	code := errObj["code"].(string)
	if code != "CS_AUTH_4034" {
		t.Fatalf("error code = %s, want CS_AUTH_4034", code)
	}
}

// TestSecurity_MissingSignature verifies that a request without the signature
// header is rejected with 403 and error code CS_AUTH_4031.
func TestSecurity_MissingSignature(t *testing.T) {
	application := newTestAppWithSecret(t)
	server := httptest.NewServer(application.Server.Handler)
	defer server.Close()

	body := []byte(`{"message_id":"m-sec-2","channel":"widget","open_id":"u_sec","content":"查询额度"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/customer-service/webhook", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CS-Timestamp", timestamp)
	// Intentionally omit X-CS-Signature

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}

	bodyOut, _ := io.ReadAll(resp.Body)
	var errPayload map[string]any
	if err := json.Unmarshal(bodyOut, &errPayload); err != nil {
		t.Fatalf("decode error response error = %v", err)
	}
	errObj := errPayload["error"].(map[string]any)
	code := errObj["code"].(string)
	if code != "CS_AUTH_4031" {
		t.Fatalf("error code = %s, want CS_AUTH_4031", code)
	}
}

// TestSecurity_ExpiredTimestamp verifies that a request with a stale timestamp
// is rejected with 403 and error code CS_AUTH_4033.
func TestSecurity_ExpiredTimestamp(t *testing.T) {
	application := newTestAppWithSecret(t)
	server := httptest.NewServer(application.Server.Handler)
	defer server.Close()

	body := []byte(`{"message_id":"m-sec-3","channel":"widget","open_id":"u_sec","content":"查询额度"}`)
	// Timestamp 10 minutes in the past — beyond the 5-minute MaxSkew
	staleUnix := time.Now().Add(-10 * time.Minute).Unix()
	timestamp, signature, err := handlers.SignWebhookRequest("e2e-test-secret", staleUnix, body)
	if err != nil {
		t.Fatalf("SignWebhookRequest error = %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/customer-service/webhook", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CS-Timestamp", timestamp)
	req.Header.Set("X-CS-Signature", signature)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}

	bodyOut, _ := io.ReadAll(resp.Body)
	var errPayload map[string]any
	if err := json.Unmarshal(bodyOut, &errPayload); err != nil {
		t.Fatalf("decode error response error = %v", err)
	}
	errObj := errPayload["error"].(map[string]any)
	code := errObj["code"].(string)
	if code != "CS_AUTH_4033" {
		t.Fatalf("error code = %s, want CS_AUTH_4033", code)
	}
}

// TestSecurity_InvalidJSONBody verifies that a request with malformed JSON body
// is rejected with 400 and error code CS_REQ_4001.
func TestSecurity_InvalidJSONBody(t *testing.T) {
	application := newTestAppWithSecret(t)
	server := httptest.NewServer(application.Server.Handler)
	defer server.Close()

	// Malformed JSON — missing closing brace and invalid value
	malformedBody := []byte(`{"message_id":"m-sec-4","channel":"widget","open_id":"u_sec","content":}`)
	timestamp, signature, err := handlers.SignWebhookRequest("e2e-test-secret", time.Now().Unix(), malformedBody)
	if err != nil {
		t.Fatalf("SignWebhookRequest error = %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/customer-service/webhook", bytes.NewReader(malformedBody))
	if err != nil {
		t.Fatalf("new request error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CS-Timestamp", timestamp)
	req.Header.Set("X-CS-Signature", signature)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}

	bodyOut, _ := io.ReadAll(resp.Body)
	var errPayload map[string]any
	if err := json.Unmarshal(bodyOut, &errPayload); err != nil {
		t.Fatalf("decode error response error = %v", err)
	}
	errObj := errPayload["error"].(map[string]any)
	code := errObj["code"].(string)
	if code != "CS_REQ_4001" {
		t.Fatalf("error code = %s, want CS_REQ_4001", code)
	}
}

// TestSecurity_EmptyBody verifies that a request with an empty body is rejected
// with 400.
func TestSecurity_EmptyBody(t *testing.T) {
	application := newTestAppWithSecret(t)
	server := httptest.NewServer(application.Server.Handler)
	defer server.Close()

	timestamp, signature, err := handlers.SignWebhookRequest("e2e-test-secret", time.Now().Unix(), []byte{})
	if err != nil {
		t.Fatalf("SignWebhookRequest error = %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/customer-service/webhook", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatalf("new request error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CS-Timestamp", timestamp)
	req.Header.Set("X-CS-Signature", signature)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestSecurity_InvalidTimestampFormat verifies that a request with a
// non-numeric timestamp is rejected with 403 and code CS_AUTH_4032.
func TestSecurity_InvalidTimestampFormat(t *testing.T) {
	application := newTestAppWithSecret(t)
	server := httptest.NewServer(application.Server.Handler)
	defer server.Close()

	body := []byte(`{"message_id":"m-sec-5","channel":"widget","open_id":"u_sec","content":"查询额度"}`)
	timestamp := "not-a-number"
	signature := "somesig"

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/customer-service/webhook", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CS-Timestamp", timestamp)
	req.Header.Set("X-CS-Signature", signature)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}

	bodyOut, _ := io.ReadAll(resp.Body)
	var errPayload map[string]any
	if err := json.Unmarshal(bodyOut, &errPayload); err != nil {
		t.Fatalf("decode error response error = %v", err)
	}
	errObj := errPayload["error"].(map[string]any)
	code := errObj["code"].(string)
	if code != "CS_AUTH_4032" {
		t.Fatalf("error code = %s, want CS_AUTH_4032", code)
	}
}
