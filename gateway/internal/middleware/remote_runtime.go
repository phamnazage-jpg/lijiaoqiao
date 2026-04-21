package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type RemoteTokenRuntime struct {
	baseURL    string
	httpClient *http.Client
	now        func() time.Time

	mu      sync.RWMutex
	records map[string]remoteResolvedToken
}

// P3-A design notes:
// - current implementation only caches token status by token_id and still falls back to http.DefaultClient.
// - dedicated client hardening should move to a gateway-owned client with:
//   total timeout=2s, dial timeout=300ms, idle conn timeout=90s, max idle conns per host=32.
// - cache TTL draft:
//   active=30s, expired=2m, revoked=10m.
// - eviction draft:
//   combine TTL expiry with max_entries=10000; evict expired records first, then oldest cache records.
// - metrics draft:
//   cache_hit, cache_miss, cache_evict, upstream_latency_ms histogram.

type remoteResolvedToken struct {
	status    TokenStatus
	expiresAt time.Time
}

type remoteIntrospectResponse struct {
	Data struct {
		TokenID   string    `json:"token_id"`
		SubjectID string    `json:"subject_id"`
		Role      string    `json:"role"`
		Status    string    `json:"status"`
		Scope     []string  `json:"scope"`
		IssuedAt  time.Time `json:"issued_at"`
		ExpiresAt time.Time `json:"expires_at"`
	} `json:"data"`
}

func NewRemoteTokenRuntime(baseURL string, httpClient *http.Client, now func() time.Time) *RemoteTokenRuntime {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if now == nil {
		now = time.Now
	}
	return &RemoteTokenRuntime{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
		now:        now,
		records:    make(map[string]remoteResolvedToken),
	}
}

func (r *RemoteTokenRuntime) Verify(ctx context.Context, rawToken string) (VerifiedToken, error) {
	payload := map[string]string{"token": rawToken}
	body, err := json.Marshal(payload)
	if err != nil {
		return VerifiedToken{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/api/v1/platform/tokens/introspect", bytes.NewReader(body))
	if err != nil {
		return VerifiedToken{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", fmt.Sprintf("gateway-introspect-%d", r.now().UnixNano()))

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return VerifiedToken{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return VerifiedToken{}, fmt.Errorf("token introspection failed with status %d", resp.StatusCode)
	}

	var result remoteIntrospectResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return VerifiedToken{}, err
	}
	if strings.TrimSpace(result.Data.TokenID) == "" {
		return VerifiedToken{}, errors.New("token introspection response missing token_id")
	}

	status := TokenStatus(strings.ToLower(strings.TrimSpace(result.Data.Status)))
	r.mu.Lock()
	r.records[result.Data.TokenID] = remoteResolvedToken{
		status:    status,
		expiresAt: result.Data.ExpiresAt,
	}
	r.mu.Unlock()

	return VerifiedToken{
		TokenID:   result.Data.TokenID,
		SubjectID: result.Data.SubjectID,
		Role:      result.Data.Role,
		Scope:     append([]string(nil), result.Data.Scope...),
		IssuedAt:  result.Data.IssuedAt,
		ExpiresAt: result.Data.ExpiresAt,
	}, nil
}

func (r *RemoteTokenRuntime) Resolve(ctx context.Context, tokenID string) (TokenStatus, error) {
	r.mu.RLock()
	record, ok := r.records[tokenID]
	r.mu.RUnlock()
	if !ok {
		return "", errors.New("token status not cached; verify must run before resolve")
	}
	if !record.expiresAt.IsZero() && !r.now().Before(record.expiresAt) && record.status == TokenStatusActive {
		return TokenStatusExpired, nil
	}
	return record.status, nil
}
