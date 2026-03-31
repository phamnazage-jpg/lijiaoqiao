package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lijiaoqiao/platform-token-runtime/internal/auth/model"
	"lijiaoqiao/platform-token-runtime/internal/auth/service"
)

const (
	tokenBasePath = "/api/v1/platform/tokens"
)

type Runtime interface {
	IssueAndAudit(ctx context.Context, input service.IssueTokenInput, auditor service.AuditEmitter) (service.TokenRecord, error)
	Refresh(ctx context.Context, tokenID string, ttl time.Duration) (service.TokenRecord, error)
	RevokeAndAudit(ctx context.Context, tokenID, reason, requestID, subjectID string, auditor service.AuditEmitter) (service.TokenRecord, error)
	Introspect(ctx context.Context, accessToken string) (service.TokenRecord, error)
	Lookup(ctx context.Context, tokenID string) (service.TokenRecord, error)
}

type TokenAPI struct {
	runtime Runtime
	auditor service.AuditEmitter
	now     func() time.Time
}

func NewTokenAPI(runtime Runtime, auditor service.AuditEmitter, now func() time.Time) *TokenAPI {
	if now == nil {
		now = time.Now
	}
	return &TokenAPI{runtime: runtime, auditor: auditor, now: now}
}

func (a *TokenAPI) Register(mux *http.ServeMux) {
	mux.HandleFunc(tokenBasePath+"/issue", a.handleIssue)
	mux.HandleFunc(tokenBasePath+"/introspect", a.handleIntrospect)
	mux.HandleFunc(tokenBasePath+"/audit-events", a.handleAuditEvents)
	mux.HandleFunc(tokenBasePath+"/", a.handleTokenAction)
}

func (a *TokenAPI) handleTokenAction(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, tokenBasePath+"/") {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "route not found")
		return
	}
	tail := strings.TrimPrefix(r.URL.Path, tokenBasePath+"/")
	parts := strings.Split(tail, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "route not found")
		return
	}
	tokenID := strings.TrimSpace(parts[0])
	action := strings.TrimSpace(parts[1])

	switch action {
	case "refresh":
		a.handleRefresh(w, r, tokenID)
	case "revoke":
		a.handleRevoke(w, r, tokenID)
	default:
		writeError(w, http.StatusNotFound, "NOT_FOUND", "route not found")
	}
}

type issueRequest struct {
	SubjectID  string   `json:"subject_id"`
	Role       string   `json:"role"`
	TTLSeconds int64    `json:"ttl_seconds"`
	Scope      []string `json:"scope"`
}

type refreshRequest struct {
	TTLSeconds int64 `json:"ttl_seconds"`
}

type revokeRequest struct {
	Reason string `json:"reason"`
}

type introspectRequest struct {
	Token string `json:"token"`
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (a *TokenAPI) handleIssue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if requestID == "" || idempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "missing X-Request-Id or Idempotency-Key")
		return
	}

	var req issueRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if err := validateIssueRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	record, err := a.runtime.IssueAndAudit(r.Context(), service.IssueTokenInput{
		SubjectID:      req.SubjectID,
		Role:           req.Role,
		Scope:          req.Scope,
		TTL:            time.Duration(req.TTLSeconds) * time.Second,
		RequestID:      requestID,
		IdempotencyKey: idempotencyKey,
	}, a.auditor)
	if err != nil {
		if strings.Contains(err.Error(), "idempotency key payload mismatch") {
			writeError(w, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "idempotency key payload mismatch")
			return
		}
		writeError(w, http.StatusUnprocessableEntity, "ISSUE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"request_id": requestID,
		"data": map[string]any{
			"token_id":     record.TokenID,
			"access_token": record.AccessToken,
			"issued_at":    record.IssuedAt,
			"expires_at":   record.ExpiresAt,
			"status":       record.Status,
		},
	})
}

func (a *TokenAPI) handleRefresh(w http.ResponseWriter, r *http.Request, tokenID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if requestID == "" || idempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "missing X-Request-Id or Idempotency-Key")
		return
	}

	var req refreshRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if req.TTLSeconds < 60 {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "ttl_seconds must be >= 60")
		return
	}

	before, err := a.runtime.Lookup(r.Context(), tokenID)
	if err != nil {
		before = service.TokenRecord{}
	}

	record, err := a.runtime.Refresh(r.Context(), tokenID, time.Duration(req.TTLSeconds)*time.Second)
	if err != nil {
		status, code := mapRuntimeError(err)
		writeError(w, status, code, err.Error())
		return
	}

	if a.auditor != nil {
		_ = a.auditor.Emit(r.Context(), service.AuditEvent{
			EventName:  service.EventTokenRefreshSuccess,
			RequestID:  requestID,
			TokenID:    record.TokenID,
			SubjectID:  record.SubjectID,
			Route:      tokenBasePath + "/" + tokenID + "/refresh",
			ResultCode: "OK",
			CreatedAt:  a.now(),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id": requestID,
		"data": map[string]any{
			"token_id":            record.TokenID,
			"previous_expires_at": before.ExpiresAt,
			"expires_at":          record.ExpiresAt,
			"status":              record.Status,
		},
	})
}

func (a *TokenAPI) handleRevoke(w http.ResponseWriter, r *http.Request, tokenID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if requestID == "" || idempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "missing X-Request-Id or Idempotency-Key")
		return
	}

	var req revokeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "reason is required")
		return
	}

	introspected, err := a.runtime.Lookup(r.Context(), tokenID)
	subjectID := ""
	if err == nil {
		subjectID = introspected.SubjectID
	}

	record, err := a.runtime.RevokeAndAudit(r.Context(), tokenID, req.Reason, requestID, subjectID, a.auditor)
	if err != nil {
		status, code := mapRuntimeError(err)
		writeError(w, status, code, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id": requestID,
		"data": map[string]any{
			"token_id":   record.TokenID,
			"status":     record.Status,
			"revoked_at": a.now(),
		},
	})
}

func (a *TokenAPI) handleIntrospect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
	if requestID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "missing X-Request-Id")
		return
	}

	var req introspectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if strings.TrimSpace(req.Token) == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "token is required")
		return
	}

	record, err := a.runtime.Introspect(r.Context(), req.Token)
	if err != nil {
		if a.auditor != nil {
			_ = a.auditor.Emit(r.Context(), service.AuditEvent{
				EventName:  service.EventTokenIntrospectFail,
				RequestID:  requestID,
				Route:      tokenBasePath + "/introspect",
				ResultCode: "INVALID_TOKEN",
				CreatedAt:  a.now(),
			})
		}
		writeError(w, http.StatusUnprocessableEntity, "TOKEN_INVALID", err.Error())
		return
	}

	if a.auditor != nil {
		_ = a.auditor.Emit(r.Context(), service.AuditEvent{
			EventName:  service.EventTokenIntrospectSuccess,
			RequestID:  requestID,
			TokenID:    record.TokenID,
			SubjectID:  record.SubjectID,
			Route:      tokenBasePath + "/introspect",
			ResultCode: "OK",
			CreatedAt:  a.now(),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id": requestID,
		"data": map[string]any{
			"token_id":   record.TokenID,
			"subject_id": record.SubjectID,
			"role":       record.Role,
			"status":     record.Status,
			"scope":      record.Scope,
			"issued_at":  record.IssuedAt,
			"expires_at": record.ExpiresAt,
		},
	})
}

func (a *TokenAPI) handleAuditEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
	if requestID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "missing X-Request-Id")
		return
	}

	querier, ok := a.auditor.(service.AuditEventQuerier)
	if !ok {
		writeError(w, http.StatusNotImplemented, "AUDIT_QUERY_NOT_READY", "audit query capability is not available")
		return
	}

	limit := parseLimit(r.URL.Query().Get("limit"))
	filter := service.AuditEventFilter{
		RequestID:  strings.TrimSpace(r.URL.Query().Get("request_id")),
		TokenID:    strings.TrimSpace(r.URL.Query().Get("token_id")),
		SubjectID:  strings.TrimSpace(r.URL.Query().Get("subject_id")),
		EventName:  strings.TrimSpace(r.URL.Query().Get("event_name")),
		ResultCode: strings.TrimSpace(r.URL.Query().Get("result_code")),
		Limit:      limit,
	}
	events, err := querier.QueryEvents(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "AUDIT_QUERY_FAILED", err.Error())
		return
	}

	items := make([]map[string]any, 0, len(events))
	for _, ev := range events {
		items = append(items, map[string]any{
			"event_id":    ev.EventID,
			"event_name":  ev.EventName,
			"request_id":  ev.RequestID,
			"token_id":    ev.TokenID,
			"subject_id":  ev.SubjectID,
			"route":       ev.Route,
			"result_code": ev.ResultCode,
			"client_ip":   ev.ClientIP,
			"created_at":  ev.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id": requestID,
		"data": map[string]any{
			"total": len(items),
			"items": items,
		},
	})
}

func validateIssueRequest(req issueRequest) error {
	if strings.TrimSpace(req.SubjectID) == "" {
		return errors.New("subject_id is required")
	}
	if req.TTLSeconds < 60 {
		return errors.New("ttl_seconds must be >= 60")
	}
	if len(req.Scope) == 0 {
		return errors.New("scope is required")
	}
	switch req.Role {
	case model.RoleOwner, model.RoleViewer, model.RoleAdmin:
		return nil
	default:
		return fmt.Errorf("unsupported role: %s", req.Role)
	}
}

func mapRuntimeError(err error) (int, string) {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "not found"):
		return http.StatusNotFound, "TOKEN_NOT_FOUND"
	case strings.Contains(msg, "not active"):
		return http.StatusConflict, "TOKEN_NOT_ACTIVE"
	case strings.Contains(msg, "idempotency key payload mismatch"):
		return http.StatusConflict, "IDEMPOTENCY_CONFLICT"
	default:
		return http.StatusUnprocessableEntity, "BUSINESS_ERROR"
	}
}

func decodeJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	return nil
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var env errorEnvelope
	env.Error.Code = code
	env.Error.Message = message
	writeJSON(w, status, env)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func parseLimit(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return 100
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return 100
	}
	if n > 500 {
		return 500
	}
	return n
}
