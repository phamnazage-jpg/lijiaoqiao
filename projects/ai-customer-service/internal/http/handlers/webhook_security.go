package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
	"github.com/bridge/ai-customer-service/internal/domain/error/cserrors"
)

type WebhookSecurity struct {
	Secret          string
	TimestampHeader string
	SignatureHeader string
	MaxSkew         time.Duration
	Audit           AuditRecorder
}

func (s WebhookSecurity) Enabled() bool {
	return strings.TrimSpace(s.Secret) != ""
}

func (s WebhookSecurity) Wrap(next http.Handler) http.Handler {
	if !s.Enabled() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		timestampHeader := strings.TrimSpace(s.TimestampHeader)
		if timestampHeader == "" {
			timestampHeader = "X-CS-Timestamp"
		}
		signatureHeader := strings.TrimSpace(s.SignatureHeader)
		if signatureHeader == "" {
			signatureHeader = "X-CS-Signature"
		}
		timestamp := strings.TrimSpace(r.Header.Get(timestampHeader))
		signature := strings.TrimSpace(r.Header.Get(signatureHeader))
		if timestamp == "" || signature == "" {
			s.auditReject(r.Context(), r, cserrors.CS_AUTH_4031, cserrors.ErrorMsg(cserrors.CS_AUTH_4031), map[string]any{"timestamp_present": timestamp != "", "signature_present": signature != ""})
			writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"code": cserrors.CS_AUTH_4031, "message": cserrors.ErrorMsg(cserrors.CS_AUTH_4031)}})
			return
		}
		unixSeconds, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			s.auditReject(r.Context(), r, cserrors.CS_AUTH_4032, cserrors.ErrorMsg(cserrors.CS_AUTH_4032), map[string]any{"timestamp": timestamp})
			writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"code": cserrors.CS_AUTH_4032, "message": cserrors.ErrorMsg(cserrors.CS_AUTH_4032)}})
			return
		}
		if skew := time.Since(time.Unix(unixSeconds, 0)); skew > s.MaxSkew || skew < -s.MaxSkew {
			s.auditReject(r.Context(), r, cserrors.CS_AUTH_4033, cserrors.ErrorMsg(cserrors.CS_AUTH_4033), map[string]any{"timestamp": timestamp, "max_skew_seconds": int(s.MaxSkew.Seconds())})
			writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"code": cserrors.CS_AUTH_4033, "message": cserrors.ErrorMsg(cserrors.CS_AUTH_4033)}})
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			s.auditReject(r.Context(), r, cserrors.CS_REQ_4004, cserrors.ErrorMsg(cserrors.CS_REQ_4004), map[string]any{"read_error": err.Error()})
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"code": cserrors.CS_REQ_4004, "message": cserrors.ErrorMsg(cserrors.CS_REQ_4004)}})
			return
		}
		expected := computeWebhookSignature(s.Secret, timestamp, body)
		if !hmac.Equal([]byte(strings.ToLower(signature)), []byte(expected)) {
			s.auditReject(r.Context(), r, cserrors.CS_AUTH_4034, cserrors.ErrorMsg(cserrors.CS_AUTH_4034), map[string]any{"timestamp": timestamp})
			writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"code": cserrors.CS_AUTH_4034, "message": cserrors.ErrorMsg(cserrors.CS_AUTH_4034)}})
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

func (s WebhookSecurity) auditReject(ctx context.Context, r *http.Request, code, messageText string, payload map[string]any) {
	if s.Audit == nil {
		return
	}
	now := time.Now()
	data := map[string]any{"error_code": code, "message": messageText, "path": r.URL.Path}
	for k, v := range payload {
		data[k] = v
	}
	// P0 quality standard: audit write failure only logs, does not return error
	_ = s.Audit.Add(ctx, audit.Event{ID: newAuditID("audit", now), Type: "webhook_security_rejected", Action: "security_reject", ActorID: "system", SourceIP: clientIP(r.RemoteAddr), Payload: data, CreatedAt: now})
}

func computeWebhookSignature(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func SignWebhookRequest(secret string, unixSeconds int64, body []byte) (string, string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", "", fmt.Errorf("secret is required")
	}
	timestamp := strconv.FormatInt(unixSeconds, 10)
	return timestamp, computeWebhookSignature(secret, timestamp, body), nil
}
