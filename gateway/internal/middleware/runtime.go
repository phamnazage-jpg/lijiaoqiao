package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

// InMemoryTokenRuntime 内存中的Token运行时实现
type InMemoryTokenRuntime struct {
	mu          sync.RWMutex
	now         func() time.Time
	records     map[string]*tokenRecord
	tokenToID   map[string]string
}

type tokenRecord struct {
	TokenID    string
	AccessToken string
	SubjectID   string
	Role        string
	Scope       []string
	IssuedAt    time.Time
	ExpiresAt   time.Time
	Status      TokenStatus
}

// NewInMemoryTokenRuntime 创建内存Token运行时
func NewInMemoryTokenRuntime(now func() time.Time) *InMemoryTokenRuntime {
	if now == nil {
		now = time.Now
	}
	return &InMemoryTokenRuntime{
		now:       now,
		records:   make(map[string]*tokenRecord),
		tokenToID: make(map[string]string),
	}
}

// Issue 颁发Token
func (r *InMemoryTokenRuntime) Issue(_ context.Context, subjectID, role string, scopes []string, ttl time.Duration) (string, error) {
	if strings.TrimSpace(subjectID) == "" {
		return "", errors.New("subject_id is required")
	}
	if strings.TrimSpace(role) == "" {
		return "", errors.New("role is required")
	}
	if len(scopes) == 0 {
		return "", errors.New("scope must not be empty")
	}
	if ttl <= 0 {
		return "", errors.New("ttl must be positive")
	}

	issuedAt := r.now()
	tokenID, _ := generateTokenID()
	accessToken, _ := generateAccessToken()

	record := &tokenRecord{
		TokenID:    tokenID,
		AccessToken: accessToken,
		SubjectID:  subjectID,
		Role:       role,
		Scope:      append([]string(nil), scopes...),
		IssuedAt:   issuedAt,
		ExpiresAt:  issuedAt.Add(ttl),
		Status:     TokenStatusActive,
	}

	r.mu.Lock()
	r.records[tokenID] = record
	r.tokenToID[accessToken] = tokenID
	r.mu.Unlock()

	return accessToken, nil
}

// Verify 验证Token
func (r *InMemoryTokenRuntime) Verify(_ context.Context, rawToken string) (VerifiedToken, error) {
	r.mu.RLock()
	tokenID, ok := r.tokenToID[rawToken]
	if !ok {
		r.mu.RUnlock()
		return VerifiedToken{}, errors.New("token not found")
	}
	record, ok := r.records[tokenID]
	if !ok {
		r.mu.RUnlock()
		return VerifiedToken{}, errors.New("token record not found")
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

// Resolve 解析Token状态
func (r *InMemoryTokenRuntime) Resolve(_ context.Context, tokenID string) (TokenStatus, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.records[tokenID]
	if !ok {
		return "", errors.New("token not found")
	}
	r.applyExpiry(record)
	return record.Status, nil
}

// Revoke 吊销Token
func (r *InMemoryTokenRuntime) Revoke(_ context.Context, tokenID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.records[tokenID]
	if !ok {
		return errors.New("token not found")
	}
	record.Status = TokenStatusRevoked
	return nil
}

func (r *InMemoryTokenRuntime) applyExpiry(record *tokenRecord) {
	if record == nil {
		return
	}
	if record.Status == TokenStatusActive && !record.ExpiresAt.IsZero() && !r.now().Before(record.ExpiresAt) {
		record.Status = TokenStatusExpired
	}
}

// ScopeRoleAuthorizer 基于Scope和Role的授权器
type ScopeRoleAuthorizer struct{}

func NewScopeRoleAuthorizer() *ScopeRoleAuthorizer {
	return &ScopeRoleAuthorizer{}
}

func (a *ScopeRoleAuthorizer) Authorize(path, method string, scopes []string, role string) bool {
	if role == "admin" {
		return true
	}

	requiredScope := requiredScopeForRoute(path, method)
	if requiredScope == "" {
		return true
	}
	return hasScope(scopes, requiredScope)
}

func requiredScopeForRoute(path, method string) string {
	// Handle /api/v1/supply (with or without trailing slash)
	if path == "/api/v1/supply" || strings.HasPrefix(path, "/api/v1/supply/") {
		switch method {
		case "GET", "HEAD", "OPTIONS":
			return "supply:read"
		default:
			return "supply:write"
		}
	}
	// Handle /api/v1/platform (with or without trailing slash)
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
			prefix := strings.TrimSuffix(scope, ":*")
			if strings.HasPrefix(required, prefix) {
				return true
			}
		}
	}
	return false
}

// MemoryAuditEmitter 内存审计发射器
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
		event.EventID, _ = generateEventID()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = e.now()
	}
	e.mu.Lock()
	e.events = append(e.events, event)
	e.mu.Unlock()
	return nil
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

func generateEventID() (string, error) {
	var entropy [8]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", err
	}
	return "evt_" + hex.EncodeToString(entropy[:]), nil
}