package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"lijiaoqiao/platform-token-runtime/internal/auth/model"
)

type TokenRecord struct {
	TokenID       string
	AccessToken   string
	SubjectID     string
	Role          string
	Scope         []string
	IssuedAt      time.Time
	ExpiresAt     time.Time
	Status        TokenStatus
	RequestID     string
	RevokedReason string
}

type IssueTokenInput struct {
	SubjectID      string
	Role           string
	Scope          []string
	TTL            time.Duration
	RequestID      string
	IdempotencyKey string
}

type InMemoryTokenRuntime struct {
	mu               sync.RWMutex
	now              func() time.Time
	records          map[string]*TokenRecord
	tokenToID        map[string]string
	idempotencyByKey map[string]idempotencyEntry
}

type idempotencyEntry struct {
	RequestHash string
	TokenID     string
}

func NewInMemoryTokenRuntime(now func() time.Time) *InMemoryTokenRuntime {
	if now == nil {
		now = time.Now
	}
	return &InMemoryTokenRuntime{
		now:              now,
		records:          make(map[string]*TokenRecord),
		tokenToID:        make(map[string]string),
		idempotencyByKey: make(map[string]idempotencyEntry),
	}
}

func (r *InMemoryTokenRuntime) Issue(_ context.Context, input IssueTokenInput) (TokenRecord, error) {
	if strings.TrimSpace(input.SubjectID) == "" {
		return TokenRecord{}, errors.New("subject_id is required")
	}
	if strings.TrimSpace(input.Role) == "" {
		return TokenRecord{}, errors.New("role is required")
	}
	if input.TTL <= 0 {
		return TokenRecord{}, errors.New("ttl must be positive")
	}
	if len(input.Scope) == 0 {
		return TokenRecord{}, errors.New("scope must not be empty")
	}
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	requestHash := hashIssueInput(input)

	issuedAt := r.now()
	tokenID, err := generateTokenID()
	if err != nil {
		return TokenRecord{}, err
	}
	accessToken, err := generateAccessToken()
	if err != nil {
		return TokenRecord{}, err
	}

	record := TokenRecord{
		TokenID:       tokenID,
		AccessToken:   accessToken,
		SubjectID:     input.SubjectID,
		Role:          input.Role,
		Scope:         append([]string(nil), input.Scope...),
		IssuedAt:      issuedAt,
		ExpiresAt:     issuedAt.Add(input.TTL),
		Status:        TokenStatusActive,
		RequestID:     input.RequestID,
		RevokedReason: "",
	}

	r.mu.Lock()
	if idempotencyKey != "" {
		entry, ok := r.idempotencyByKey[idempotencyKey]
		if ok {
			if entry.RequestHash != requestHash {
				r.mu.Unlock()
				return TokenRecord{}, errors.New("idempotency key payload mismatch")
			}
			existing, exists := r.records[entry.TokenID]
			if exists {
				r.mu.Unlock()
				return cloneRecord(*existing), nil
			}
		}
	}
	r.records[tokenID] = &record
	r.tokenToID[accessToken] = tokenID
	if idempotencyKey != "" {
		r.idempotencyByKey[idempotencyKey] = idempotencyEntry{
			RequestHash: requestHash,
			TokenID:     tokenID,
		}
	}
	r.mu.Unlock()

	return record, nil
}

func (r *InMemoryTokenRuntime) Refresh(_ context.Context, tokenID string, ttl time.Duration) (TokenRecord, error) {
	if ttl <= 0 {
		return TokenRecord{}, errors.New("ttl must be positive")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.records[tokenID]
	if !ok {
		return TokenRecord{}, errors.New("token not found")
	}
	r.applyExpiry(record)
	if record.Status != TokenStatusActive {
		return TokenRecord{}, errors.New("token is not active")
	}

	record.ExpiresAt = r.now().Add(ttl)
	return cloneRecord(*record), nil
}

func (r *InMemoryTokenRuntime) Revoke(_ context.Context, tokenID, reason string) (TokenRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.records[tokenID]
	if !ok {
		return TokenRecord{}, errors.New("token not found")
	}
	r.applyExpiry(record)
	record.Status = TokenStatusRevoked
	record.RevokedReason = strings.TrimSpace(reason)
	return cloneRecord(*record), nil
}

func (r *InMemoryTokenRuntime) Introspect(_ context.Context, accessToken string) (TokenRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tokenID, ok := r.tokenToID[accessToken]
	if !ok {
		return TokenRecord{}, errors.New("token not found")
	}
	record := r.records[tokenID]
	r.applyExpiry(record)
	return cloneRecord(*record), nil
}

func (r *InMemoryTokenRuntime) Lookup(_ context.Context, tokenID string) (TokenRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.records[tokenID]
	if !ok {
		return TokenRecord{}, errors.New("token not found")
	}
	r.applyExpiry(record)
	return cloneRecord(*record), nil
}

func (r *InMemoryTokenRuntime) Verify(_ context.Context, rawToken string) (VerifiedToken, error) {
	r.mu.RLock()
	tokenID, ok := r.tokenToID[rawToken]
	if !ok {
		r.mu.RUnlock()
		return VerifiedToken{}, NewAuthError(CodeAuthInvalidToken, errors.New("token not found"))
	}
	record, ok := r.records[tokenID]
	if !ok {
		r.mu.RUnlock()
		return VerifiedToken{}, NewAuthError(CodeAuthInvalidToken, errors.New("token record not found"))
	}
	claims := VerifiedToken{
		TokenID:   record.TokenID,
		SubjectID: record.SubjectID,
		Role:      record.Role,
		Scope:     append([]string(nil), record.Scope...),
		IssuedAt:  record.IssuedAt,
		ExpiresAt: record.ExpiresAt,
	}
	r.mu.RUnlock()
	return claims, nil
}

func (r *InMemoryTokenRuntime) Resolve(_ context.Context, tokenID string) (TokenStatus, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.records[tokenID]
	if !ok {
		return "", NewAuthError(CodeAuthInvalidToken, errors.New("token not found"))
	}
	r.applyExpiry(record)
	return record.Status, nil
}

func (r *InMemoryTokenRuntime) TokenCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.records)
}

func (r *InMemoryTokenRuntime) IssueAndAudit(ctx context.Context, input IssueTokenInput, auditor AuditEmitter) (TokenRecord, error) {
	record, err := r.Issue(ctx, input)
	if err != nil {
		emitAudit(auditor, AuditEvent{
			EventName:  EventTokenIssueFail,
			RequestID:  input.RequestID,
			SubjectID:  input.SubjectID,
			Route:      "/api/v1/platform/tokens/issue",
			ResultCode: "ISSUE_FAILED",
		}, r.now)
		return TokenRecord{}, err
	}
	emitAudit(auditor, AuditEvent{
		EventName:  EventTokenIssueSuccess,
		RequestID:  input.RequestID,
		TokenID:    record.TokenID,
		SubjectID:  record.SubjectID,
		Route:      "/api/v1/platform/tokens/issue",
		ResultCode: "OK",
	}, r.now)
	return record, nil
}

func (r *InMemoryTokenRuntime) RevokeAndAudit(ctx context.Context, tokenID, reason, requestID, subjectID string, auditor AuditEmitter) (TokenRecord, error) {
	record, err := r.Revoke(ctx, tokenID, reason)
	if err != nil {
		emitAudit(auditor, AuditEvent{
			EventName:  EventTokenRevokeFail,
			RequestID:  requestID,
			TokenID:    tokenID,
			SubjectID:  subjectID,
			Route:      "/api/v1/platform/tokens/revoke",
			ResultCode: "REVOKE_FAILED",
		}, r.now)
		return TokenRecord{}, err
	}
	emitAudit(auditor, AuditEvent{
		EventName:  EventTokenRevokeSuccess,
		RequestID:  requestID,
		TokenID:    record.TokenID,
		SubjectID:  record.SubjectID,
		Route:      "/api/v1/platform/tokens/revoke",
		ResultCode: "OK",
	}, r.now)
	return record, nil
}

func (r *InMemoryTokenRuntime) applyExpiry(record *TokenRecord) {
	if record == nil {
		return
	}
	if record.Status == TokenStatusActive && !record.ExpiresAt.IsZero() && !r.now().Before(record.ExpiresAt) {
		record.Status = TokenStatusExpired
	}
}

func cloneRecord(record TokenRecord) TokenRecord {
	record.Scope = append([]string(nil), record.Scope...)
	return record
}

func hashIssueInput(input IssueTokenInput) string {
	scope := append([]string(nil), input.Scope...)
	sort.Strings(scope)
	joined := strings.Join(scope, ",")
	data := strings.TrimSpace(input.SubjectID) + "|" +
		strings.TrimSpace(input.Role) + "|" +
		joined + "|" +
		input.TTL.String()
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}

func generateAccessToken() (string, error) {
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", err
	}
	return "ptk_" + hex.EncodeToString(entropy[:]), nil
}

func generateTokenID() (string, error) {
	var entropy [8]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", err
	}
	return "tok_" + hex.EncodeToString(entropy[:]), nil
}

type ScopeRoleAuthorizer struct{}

func NewScopeRoleAuthorizer() *ScopeRoleAuthorizer {
	return &ScopeRoleAuthorizer{}
}

func (a *ScopeRoleAuthorizer) Authorize(path, method string, scopes []string, role string) bool {
	if role == model.RoleAdmin {
		return true
	}

	requiredScope := requiredScopeForRoute(path, method)
	if requiredScope == "" {
		return true
	}
	return hasScope(scopes, requiredScope)
}

func requiredScopeForRoute(path, method string) string {
	if path == "/api/v1/supply" || strings.HasPrefix(path, "/api/v1/supply/") {
		switch method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			return "supply:read"
		default:
			return "supply:write"
		}
	}
	if path == "/api/v1/platform" || strings.HasPrefix(path, "/api/v1/platform/") {
		return "platform:admin"
	}
	return ""
}

func hasScope(scopes []string, required string) bool {
	for _, scope := range scopes {
		if scope == required {
			return true
		}
		if strings.HasSuffix(scope, ":*") {
			prefix := strings.TrimSuffix(scope, "*")
			if strings.HasPrefix(required, prefix) {
				return true
			}
		}
	}
	return false
}

type MemoryAuditEmitter struct {
	mu     sync.RWMutex
	events []AuditEvent
	now    func() time.Time
}

func NewMemoryAuditEmitter() *MemoryAuditEmitter {
	return &MemoryAuditEmitter{now: time.Now}
}

func (e *MemoryAuditEmitter) Emit(_ context.Context, event AuditEvent) error {
	if event.EventID == "" {
		eventID, err := generateEventID()
		if err != nil {
			return err
		}
		event.EventID = eventID
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = e.now()
	}
	e.mu.Lock()
	e.events = append(e.events, event)
	e.mu.Unlock()
	return nil
}

func (e *MemoryAuditEmitter) Events() []AuditEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()
	copied := make([]AuditEvent, len(e.events))
	copy(copied, e.events)
	return copied
}

func (e *MemoryAuditEmitter) QueryEvents(_ context.Context, filter AuditEventFilter) ([]AuditEvent, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	result := make([]AuditEvent, 0, minInt(limit, len(e.events)))
	for idx := len(e.events) - 1; idx >= 0; idx-- {
		ev := e.events[idx]
		if !matchAuditFilter(ev, filter) {
			continue
		}
		result = append(result, ev)
		if len(result) >= limit {
			break
		}
	}

	// 按时间正序返回，便于前端/审计系统展示时间线。
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, nil
}

func (e *MemoryAuditEmitter) LastEvent() (AuditEvent, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if len(e.events) == 0 {
		return AuditEvent{}, false
	}
	return e.events[len(e.events)-1], true
}

func emitAudit(emitter AuditEmitter, event AuditEvent, now func() time.Time) {
	if emitter == nil {
		return
	}
	if now == nil {
		now = time.Now
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now()
	}
	_ = emitter.Emit(context.Background(), event)
}

func matchAuditFilter(ev AuditEvent, filter AuditEventFilter) bool {
	if filter.RequestID != "" && ev.RequestID != filter.RequestID {
		return false
	}
	if filter.TokenID != "" && ev.TokenID != filter.TokenID {
		return false
	}
	if filter.SubjectID != "" && ev.SubjectID != filter.SubjectID {
		return false
	}
	if filter.EventName != "" && ev.EventName != filter.EventName {
		return false
	}
	if filter.ResultCode != "" && ev.ResultCode != filter.ResultCode {
		return false
	}
	return true
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func generateEventID() (string, error) {
	var entropy [8]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", err
	}
	return "evt_" + hex.EncodeToString(entropy[:]), nil
}
