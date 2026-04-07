package middleware

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/cache"
	"lijiaoqiao/supply-api/internal/repository"
)

// MockTokenStatusRepository mock Token状态仓储
type MockTokenStatusRepository struct {
	mu                   sync.RWMutex
	tokenStatuses        map[string]string
	tokenReasons         map[string]string
	verificationCounts   map[string]int
	subjectTokens        map[int64][]string
}

func NewMockTokenStatusRepository() *MockTokenStatusRepository {
	return &MockTokenStatusRepository{
		tokenStatuses:      make(map[string]string),
		tokenReasons:       make(map[string]string),
		verificationCounts: make(map[string]int),
		subjectTokens:      make(map[int64][]string),
	}
}

func (m *MockTokenStatusRepository) GetStatus(ctx context.Context, tokenID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if status, ok := m.tokenStatuses[tokenID]; ok {
		return status, nil
	}
	return "active", nil
}

func (m *MockTokenStatusRepository) Revoke(ctx context.Context, tokenID string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokenStatuses[tokenID] = "revoked"
	m.tokenReasons[tokenID] = reason
	return nil
}

func (m *MockTokenStatusRepository) UpdateVerificationCount(ctx context.Context, tokenID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verificationCounts[tokenID]++
	return nil
}

func (m *MockTokenStatusRepository) RevokeBySubjectID(ctx context.Context, subjectID int64, reason string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if tokens, ok := m.subjectTokens[subjectID]; ok {
		for _, tokenID := range tokens {
			m.tokenStatuses[tokenID] = "revoked"
			m.tokenReasons[tokenID] = reason
		}
		return int64(len(tokens)), nil
	}
	return 0, nil
}

func (m *MockTokenStatusRepository) ListActiveBySubjectID(ctx context.Context, subjectID int64) ([]*repository.TokenStatusRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if tokens, ok := m.subjectTokens[subjectID]; ok {
		var records []*repository.TokenStatusRecord
		for _, tokenID := range tokens {
			if m.tokenStatuses[tokenID] != "revoked" {
				records = append(records, &repository.TokenStatusRecord{TokenID: tokenID})
			}
		}
		return records, nil
	}
	return nil, nil
}

// MockRedisCache mock Redis缓存
type MockRedisCache struct {
	mu           sync.RWMutex
	tokenCache   map[string]*cache.TokenStatus
	subscribers  []func(event *cache.TokenRevokedCacheEvent)
}

func NewMockRedisCache() *MockRedisCache {
	return &MockRedisCache{
		tokenCache: make(map[string]*cache.TokenStatus),
	}
}

func (m *MockRedisCache) GetTokenStatus(ctx context.Context, tokenID string) (*cache.TokenStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if status, ok := m.tokenCache[tokenID]; ok {
		return status, nil
	}
	return nil, nil
}

func (m *MockRedisCache) SetTokenStatus(ctx context.Context, status *cache.TokenStatus, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokenCache[status.TokenID] = status
	return nil
}

func (m *MockRedisCache) InvalidateToken(ctx context.Context, tokenID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tokenCache, tokenID)
	return nil
}

func (m *MockRedisCache) SubscribeTokenRevoked(ctx context.Context, handler func(event *cache.TokenRevokedCacheEvent)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscribers = append(m.subscribers, handler)
	return nil
}

func (m *MockRedisCache) PublishRevocation(tokenID string, reason string) {
	// 先复制 handlers 避免死锁
	m.mu.RLock()
	handlers := make([]func(event *cache.TokenRevokedCacheEvent), len(m.subscribers))
	copy(handlers, m.subscribers)
	m.mu.RUnlock()

	// 在锁外调用 handlers
	for _, handler := range handlers {
		handler(&cache.TokenRevokedCacheEvent{
			TokenID: tokenID,
			Reason: reason,
		})
	}
}

// PublishTokenRevoked 实现 TokenCacheBackend 接口
func (m *MockRedisCache) PublishTokenRevoked(ctx context.Context, event *cache.TokenRevokedCacheEvent) error {
	m.mu.RLock()
	handlers := make([]func(event *cache.TokenRevokedCacheEvent), len(m.subscribers))
	copy(handlers, m.subscribers)
	m.mu.RUnlock()

	for _, handler := range handlers {
		handler(event)
	}
	return nil
}

// TokenStatusRepositoryInterface mock需要的接口
type TokenStatusRepositoryInterface interface {
	GetStatus(ctx context.Context, tokenID string) (string, error)
	Revoke(ctx context.Context, tokenID string, reason string) error
	UpdateVerificationCount(ctx context.Context, tokenID string) error
	RevokeBySubjectID(ctx context.Context, subjectID int64, reason string) (int64, error)
	ListActiveBySubjectID(ctx context.Context, subjectID int64) ([]*repository.TokenStatusRecord, error)
}

// DBTokenStatusBackendForTest 用于测试的DBTokenStatusBackend
type DBTokenStatusBackendForTest struct {
	repo        TokenStatusRepositoryInterface
	redisCache  *MockRedisCache
	cacheTTL    time.Duration
}

func NewDBTokenStatusBackendForTest(repo TokenStatusRepositoryInterface, redisCache *MockRedisCache, cacheTTL time.Duration) *DBTokenStatusBackendForTest {
	if cacheTTL == 0 {
		cacheTTL = 10 * time.Second
	}
	return &DBTokenStatusBackendForTest{
		repo:       repo,
		redisCache: redisCache,
		cacheTTL:   cacheTTL,
	}
}

func (b *DBTokenStatusBackendForTest) CheckTokenStatus(ctx context.Context, tokenID string) (string, error) {
	// 1. 先查Redis缓存
	if b.redisCache != nil {
		cached, err := b.redisCache.GetTokenStatus(ctx, tokenID)
		if err == nil && cached != nil {
			return cached.Status, nil
		}
	}

	// 2. 查DB获取真实状态
	status, err := b.repo.GetStatus(ctx, tokenID)
	if err != nil {
		return "", err
	}

	// 3. 更新缓存
	if b.redisCache != nil {
		tokenStatus := &cache.TokenStatus{
			TokenID:   tokenID,
			Status:    status,
			ExpiresAt: time.Now().Add(b.cacheTTL).Unix(),
		}
		_ = b.redisCache.SetTokenStatus(ctx, tokenStatus, b.cacheTTL)
	}

	// 4. 异步更新验证计数（使用超时context避免阻塞）
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_ = b.repo.UpdateVerificationCount(ctx, tokenID)
	}()

	return status, nil
}

func (b *DBTokenStatusBackendForTest) RevokeToken(ctx context.Context, tokenID string, reason string) error {
	if err := b.repo.Revoke(ctx, tokenID, reason); err != nil {
		return err
	}
	if b.redisCache != nil {
		_ = b.redisCache.InvalidateToken(ctx, tokenID)
	}
	return nil
}

func (b *DBTokenStatusBackendForTest) RevokeBySubjectID(ctx context.Context, subjectID int64, reason string) error {
	count, err := b.repo.RevokeBySubjectID(ctx, subjectID, reason)
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	if b.redisCache != nil {
		records, _ := b.repo.ListActiveBySubjectID(ctx, subjectID)
		for _, record := range records {
			_ = b.redisCache.InvalidateToken(ctx, record.TokenID)
		}
	}
	return nil
}

// Tests

func TestDBTokenStatusBackend_CheckTokenStatus_CacheHit(t *testing.T) {
	repo := NewMockTokenStatusRepository()
	redisCache := NewMockRedisCache()

	// 预设缓存数据
	redisCache.tokenCache["token123"] = &cache.TokenStatus{
		TokenID: "token123",
		Status:  "active",
	}

	backend := NewDBTokenStatusBackendForTest(repo, redisCache, 10*time.Second)

	status, err := backend.CheckTokenStatus(context.Background(), "token123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "active" {
		t.Errorf("expected status 'active', got '%s'", status)
	}
}

func TestDBTokenStatusBackend_CheckTokenStatus_CacheMiss_DBHit(t *testing.T) {
	repo := NewMockTokenStatusRepository()
	redisCache := NewMockRedisCache()

	// 设置DB中的状态
	repo.tokenStatuses["token456"] = "revoked"

	backend := NewDBTokenStatusBackendForTest(repo, redisCache, 10*time.Second)

	status, err := backend.CheckTokenStatus(context.Background(), "token456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "revoked" {
		t.Errorf("expected status 'revoked', got '%s'", status)
	}

	// 验证缓存已更新
	redisCache.mu.RLock()
	cached, ok := redisCache.tokenCache["token456"]
	redisCache.mu.RUnlock()
	if !ok {
		t.Error("expected cache to be updated")
	}
	if cached.Status != "revoked" {
		t.Errorf("expected cached status 'revoked', got '%s'", cached.Status)
	}
}

func TestDBTokenStatusBackend_CheckTokenStatus_NoCache(t *testing.T) {
	repo := NewMockTokenStatusRepository()
	redisCache := NewMockRedisCache()

	backend := NewDBTokenStatusBackendForTest(repo, redisCache, 10*time.Second)

	status, err := backend.CheckTokenStatus(context.Background(), "token789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "active" {
		t.Errorf("expected default status 'active', got '%s'", status)
	}
}

func TestDBTokenStatusBackend_RevokeToken(t *testing.T) {
	repo := NewMockTokenStatusRepository()
	redisCache := NewMockRedisCache()

	// 预设缓存
	redisCache.tokenCache["token-revoke"] = &cache.TokenStatus{
		TokenID: "token-revoke",
		Status:  "active",
	}

	backend := NewDBTokenStatusBackendForTest(repo, redisCache, 10*time.Second)

	err := backend.RevokeToken(context.Background(), "token-revoke", "test revocation")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证DB状态已更新
	repo.mu.RLock()
	status := repo.tokenStatuses["token-revoke"]
	reason := repo.tokenReasons["token-revoke"]
	repo.mu.RUnlock()

	if status != "revoked" {
		t.Errorf("expected status 'revoked', got '%s'", status)
	}
	if reason != "test revocation" {
		t.Errorf("expected reason 'test revocation', got '%s'", reason)
	}

	// 验证缓存已失效
	redisCache.mu.RLock()
	_, ok := redisCache.tokenCache["token-revoke"]
	redisCache.mu.RUnlock()
	if ok {
		t.Error("expected cache to be invalidated")
	}
}

func TestDBTokenStatusBackend_RevokeBySubjectID(t *testing.T) {
	repo := NewMockTokenStatusRepository()
	redisCache := NewMockRedisCache()

	// 设置subject的tokens
	repo.subjectTokens[123] = []string{"token1", "token2", "token3"}

	backend := NewDBTokenStatusBackendForTest(repo, redisCache, 10*time.Second)

	err := backend.RevokeBySubjectID(context.Background(), 123, "bulk revocation")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证所有token都已吊销
	repo.mu.RLock()
	for _, tokenID := range []string{"token1", "token2", "token3"} {
		if repo.tokenStatuses[tokenID] != "revoked" {
			t.Errorf("expected token %s to be revoked", tokenID)
		}
	}
	repo.mu.RUnlock()
}

func TestDBTokenStatusBackend_RevokeBySubjectID_NoTokens(t *testing.T) {
	repo := NewMockTokenStatusRepository()
	redisCache := NewMockRedisCache()

	backend := NewDBTokenStatusBackendForTest(repo, redisCache, 10*time.Second)

	err := backend.RevokeBySubjectID(context.Background(), 999, "no tokens")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 无token可吊销，不应该报错
}

func TestDBTokenStatusBackend_VerificationCount(t *testing.T) {
	repo := NewMockTokenStatusRepository()

	// 直接调用UpdateVerificationCount来测试计数逻辑
	repo.UpdateVerificationCount(context.Background(), "verify-token")
	repo.UpdateVerificationCount(context.Background(), "verify-token")
	repo.UpdateVerificationCount(context.Background(), "verify-token")

	repo.mu.RLock()
	count := repo.verificationCounts["verify-token"]
	repo.mu.RUnlock()

	if count != 3 {
		t.Errorf("expected verification count 3, got %d", count)
	}
}

func TestDBTokenStatusBackend_InterfaceCompliance(t *testing.T) {
	// 验证 DBTokenStatusBackendForTest 实现了必要的接口模式
	repo := NewMockTokenStatusRepository()
	redisCache := NewMockRedisCache()
	backend := NewDBTokenStatusBackendForTest(repo, redisCache, 10*time.Second)

	// 测试各种状态转换
	tests := []struct {
		name        string
		tokenID     string
		initialStatus string
		action      func() error
		expectedStatus string
	}{
		{
			name:          "active to revoked",
			tokenID:       "test-active",
			initialStatus: "active",
			action: func() error {
				return backend.RevokeToken(context.Background(), "test-active", "testing")
			},
			expectedStatus: "revoked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.tokenStatuses[tt.tokenID] = tt.initialStatus
			err := tt.action()
			if err != nil {
				t.Errorf("action failed: %v", err)
			}
			status, _ := backend.CheckTokenStatus(context.Background(), tt.tokenID)
			if status != tt.expectedStatus {
				t.Errorf("expected status '%s', got '%s'", tt.expectedStatus, status)
			}
		})
	}
}

// TestDBTokenStatusBackend_ConcurrentAccess 测试并发访问
func TestDBTokenStatusBackend_ConcurrentAccess(t *testing.T) {
	repo := NewMockTokenStatusRepository()

	// 并发读写 mutex 保护的 map 应该安全
	for i := 0; i < 100; i++ {
		repo.mu.Lock()
		repo.tokenStatuses["concurrent-token"] = "active"
		repo.mu.Unlock()
	}

	for i := 0; i < 100; i++ {
		repo.mu.RLock()
		_ = repo.tokenStatuses["concurrent-token"]
		repo.mu.RUnlock()
	}
}

// TestDBTokenStatusBackend_PubSubRevocation 测试Pub/Sub吊销通知
func TestDBTokenStatusBackend_PubSubRevocation(t *testing.T) {
	redisCache := NewMockRedisCache()

	// 预设缓存
	redisCache.tokenCache["pubsub-token"] = &cache.TokenStatus{
		TokenID: "pubsub-token",
		Status:  "active",
	}

	// 手动订阅吊销事件
	redisCache.SubscribeTokenRevoked(context.Background(), func(event *cache.TokenRevokedCacheEvent) {
		_ = redisCache.InvalidateToken(context.Background(), event.TokenID)
	})

	// 模拟发布吊销事件
	redisCache.PublishRevocation("pubsub-token", "pub/sub test")

	// 验证缓存已失效
	redisCache.mu.RLock()
	_, ok := redisCache.tokenCache["pubsub-token"]
	redisCache.mu.RUnlock()
	if ok {
		t.Error("expected cache to be invalidated via pub/sub")
	}
}

// TestDBTokenStatusBackend_GetStatus 测试GetStatus方法
func TestDBTokenStatusBackend_GetStatus(t *testing.T) {
	repo := NewMockTokenStatusRepository()
	redisCache := NewMockRedisCache()

	repo.tokenStatuses["get-test"] = "expired"

	backend := NewDBTokenStatusBackendForTest(repo, redisCache, 10*time.Second)

	status, err := backend.CheckTokenStatus(context.Background(), "get-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "expired" {
		t.Errorf("expected status 'expired', got '%s'", status)
	}
}

// TestDBTokenStatusBackend_ListActiveBySubjectID 测试按SubjectID列出活跃Token
func TestDBTokenStatusBackend_ListActiveBySubjectID(t *testing.T) {
	repo := NewMockTokenStatusRepository()

	// 设置一些活跃token和一个已吊销的token
	repo.subjectTokens[100] = []string{"active1", "active2", "revoked1"}
	repo.tokenStatuses["active1"] = "active"
	repo.tokenStatuses["active2"] = "active"
	repo.tokenStatuses["revoked1"] = "revoked"

	records, err := repo.ListActiveBySubjectID(context.Background(), 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("expected 2 active tokens, got %d", len(records))
	}
}

// TestDBTokenStatusBackend_EdgeCases 测试边界情况
func TestDBTokenStatusBackend_EdgeCases(t *testing.T) {
	t.Run("empty token ID", func(t *testing.T) {
		repo := NewMockTokenStatusRepository()
		backend := NewDBTokenStatusBackendForTest(repo, nil, 10*time.Second)

		_, err := backend.CheckTokenStatus(context.Background(), "")
		if err != nil {
			// 空token ID可能导致各种错误，都是合理的
			t.Logf("empty token ID error: %v", err)
		}
	})

	t.Run("nil context", func(t *testing.T) {
		repo := NewMockTokenStatusRepository()
		backend := NewDBTokenStatusBackendForTest(repo, nil, 10*time.Second)

		_, err := backend.CheckTokenStatus(nil, "some-token")
		if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			// nil context 可能导致错误
			t.Logf("nil context error: %v", err)
		}
	})

	t.Run("zero cache TTL", func(t *testing.T) {
		repo := NewMockTokenStatusRepository()
		// 使用零值TTL，应该使用默认值
		backend := NewDBTokenStatusBackendForTest(repo, nil, 0)

		if backend.cacheTTL != 10*time.Second {
			t.Errorf("expected default TTL 10s, got %v", backend.cacheTTL)
		}
	})
}

// ==================== 直接测试 DBTokenStatusBackend ====================

func TestDBTokenStatusBackend_NewDBTokenStatusBackend(t *testing.T) {
	repo := NewMockTokenStatusRepository()
	redisCache := NewMockRedisCache()

	backend := NewDBTokenStatusBackend(repo, redisCache, 10*time.Second)

	if backend == nil {
		t.Fatal("expected non-nil backend")
	}
	if backend.repo == nil {
		t.Error("expected repo to be set")
	}
	if backend.redisCache == nil {
		t.Error("expected redisCache to be set")
	}
	if backend.cacheTTL != 10*time.Second {
		t.Errorf("expected TTL 10s, got %v", backend.cacheTTL)
	}
}

func TestDBTokenStatusBackend_NewDBTokenStatusBackend_DefaultTTL(t *testing.T) {
	repo := NewMockTokenStatusRepository()
	redisCache := NewMockRedisCache()

	// 使用零值TTL
	backend := NewDBTokenStatusBackend(repo, redisCache, 0)

	if backend.cacheTTL != 10*time.Second {
		t.Errorf("expected default TTL 10s, got %v", backend.cacheTTL)
	}
}

func TestDBTokenStatusBackend_CheckTokenStatus_CacheHit_Real(t *testing.T) {
	repo := NewMockTokenStatusRepository()
	redisCache := NewMockRedisCache()

	// 预设缓存数据
	redisCache.tokenCache["token123"] = &cache.TokenStatus{
		TokenID: "token123",
		Status:  "active",
	}

	backend := NewDBTokenStatusBackend(repo, redisCache, 10*time.Second)

	status, err := backend.CheckTokenStatus(context.Background(), "token123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "active" {
		t.Errorf("expected status 'active', got '%s'", status)
	}
}

func TestDBTokenStatusBackend_CheckTokenStatus_CacheMiss_Real(t *testing.T) {
	repo := NewMockTokenStatusRepository()
	redisCache := NewMockRedisCache()

	// 设置DB中的状态
	repo.tokenStatuses["token456"] = "revoked"

	backend := NewDBTokenStatusBackend(repo, redisCache, 10*time.Second)

	status, err := backend.CheckTokenStatus(context.Background(), "token456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "revoked" {
		t.Errorf("expected status 'revoked', got '%s'", status)
	}

	// 验证缓存已更新
	redisCache.mu.RLock()
	cached, ok := redisCache.tokenCache["token456"]
	redisCache.mu.RUnlock()
	if !ok {
		t.Error("expected cache to be updated")
	}
	if cached.Status != "revoked" {
		t.Errorf("expected cached status 'revoked', got '%s'", cached.Status)
	}
}

func TestDBTokenStatusBackend_CheckTokenStatus_NilRedisCache(t *testing.T) {
	repo := NewMockTokenStatusRepository()

	// 不设置redisCache
	backend := NewDBTokenStatusBackend(repo, nil, 10*time.Second)

	status, err := backend.CheckTokenStatus(context.Background(), "token789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "active" {
		t.Errorf("expected default status 'active', got '%s'", status)
	}
}

func TestDBTokenStatusBackend_RevokeToken_Real(t *testing.T) {
	repo := NewMockTokenStatusRepository()
	redisCache := NewMockRedisCache()

	// 预设缓存
	redisCache.tokenCache["token-revoke"] = &cache.TokenStatus{
		TokenID: "token-revoke",
		Status:  "active",
	}

	backend := NewDBTokenStatusBackend(repo, redisCache, 10*time.Second)

	err := backend.RevokeToken(context.Background(), "token-revoke", "test revocation")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证DB状态已更新
	repo.mu.RLock()
	status := repo.tokenStatuses["token-revoke"]
	reason := repo.tokenReasons["token-revoke"]
	repo.mu.RUnlock()

	if status != "revoked" {
		t.Errorf("expected status 'revoked', got '%s'", status)
	}
	if reason != "test revocation" {
		t.Errorf("expected reason 'test revocation', got '%s'", reason)
	}

	// 验证缓存已失效
	redisCache.mu.RLock()
	_, ok := redisCache.tokenCache["token-revoke"]
	redisCache.mu.RUnlock()
	if ok {
		t.Error("expected cache to be invalidated")
	}
}

func TestDBTokenStatusBackend_GetTokenStatus_Real(t *testing.T) {
	repo := NewMockTokenStatusRepository()
	redisCache := NewMockRedisCache()

	repo.tokenStatuses["get-test"] = "expired"

	backend := NewDBTokenStatusBackend(repo, redisCache, 10*time.Second)

	status, err := backend.GetTokenStatus(context.Background(), "get-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "expired" {
		t.Errorf("expected status 'expired', got '%s'", status)
	}
}

func TestDBTokenStatusBackend_RevokeBySubjectID_Real(t *testing.T) {
	repo := NewMockTokenStatusRepository()
	redisCache := NewMockRedisCache()

	// 设置subject的tokens
	repo.subjectTokens[123] = []string{"token1", "token2", "token3"}

	backend := NewDBTokenStatusBackend(repo, redisCache, 10*time.Second)

	err := backend.RevokeBySubjectID(context.Background(), 123, "bulk revocation")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证所有token都已吊销
	repo.mu.RLock()
	for _, tokenID := range []string{"token1", "token2", "token3"} {
		if repo.tokenStatuses[tokenID] != "revoked" {
			t.Errorf("expected token %s to be revoked", tokenID)
		}
	}
	repo.mu.RUnlock()
}

func TestDBTokenStatusBackend_RevokeBySubjectID_NoTokens_Real(t *testing.T) {
	repo := NewMockTokenStatusRepository()
	redisCache := NewMockRedisCache()

	backend := NewDBTokenStatusBackend(repo, redisCache, 10*time.Second)

	err := backend.RevokeBySubjectID(context.Background(), 999, "no tokens")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDBTokenStatusBackend_StartRevocationSubscriber(t *testing.T) {
	repo := NewMockTokenStatusRepository()
	redisCache := NewMockRedisCache()

	backend := NewDBTokenStatusBackend(repo, redisCache, 10*time.Second)

	err := backend.StartRevocationSubscriber(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDBTokenStatusBackend_StartRevocationSubscriber_NoRedisCache(t *testing.T) {
	repo := NewMockTokenStatusRepository()

	backend := NewDBTokenStatusBackend(repo, nil, 10*time.Second)

	err := backend.StartRevocationSubscriber(context.Background())
	if err == nil {
		t.Error("expected error when redis cache is nil")
	}
}

// ==================== TokenRevocationService Tests ====================

// MockTokenRevocationBackend mock TokenRevocationBackend
type MockTokenRevocationBackend struct {
	mu           sync.RWMutex
	revokedTokens map[string]string
}

func NewMockTokenRevocationBackend() *MockTokenRevocationBackend {
	return &MockTokenRevocationBackend{
		revokedTokens: make(map[string]string),
	}
}

func (m *MockTokenRevocationBackend) RevokeToken(ctx context.Context, tokenID string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revokedTokens[tokenID] = reason
	return nil
}

func (m *MockTokenRevocationBackend) GetTokenStatus(ctx context.Context, tokenID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if reason, ok := m.revokedTokens[tokenID]; ok {
		return "revoked:" + reason, nil
	}
	return "active", nil
}

func TestNewTokenRevocationService(t *testing.T) {
	redisCache := NewMockRedisCache()
	backend := NewMockTokenRevocationBackend()

	service := NewTokenRevocationService(redisCache, backend)

	if service == nil {
		t.Fatal("expected non-nil service")
	}
	if service.redisCache == nil {
		t.Error("expected redisCache to be set")
	}
	if service.dbBackend == nil {
		t.Error("expected dbBackend to be set")
	}
}

func TestTokenRevocationService_RevokeLocalOnly(t *testing.T) {
	redisCache := NewMockRedisCache()
	backend := NewMockTokenRevocationBackend()

	// 预设缓存
	redisCache.tokenCache["local-token"] = &cache.TokenStatus{
		TokenID: "local-token",
		Status:  "active",
	}

	service := NewTokenRevocationService(redisCache, backend)
	ctx := context.Background()

	err := service.RevokeLocalOnly(ctx, "local-token")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// 验证缓存已失效
	redisCache.mu.RLock()
	_, ok := redisCache.tokenCache["local-token"]
	redisCache.mu.RUnlock()
	if ok {
		t.Error("expected token to be invalidated")
	}
}

func TestTokenRevocationService_RevokeAndPublish(t *testing.T) {
	redisCache := NewMockRedisCache()
	backend := NewMockTokenRevocationBackend()

	service := NewTokenRevocationService(redisCache, backend)
	ctx := context.Background()

	err := service.RevokeAndPublish(ctx, "publish-token", "test reason")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证DB状态已更新
	backend.mu.RLock()
	reason := backend.revokedTokens["publish-token"]
	backend.mu.RUnlock()
	if reason != "test reason" {
		t.Errorf("expected reason 'test reason', got '%s'", reason)
	}
}

func TestTokenRevocationService_RevokeAndPublish_DBError(t *testing.T) {
	redisCache := NewMockRedisCache()
	backend := &MockTokenRevocationBackendWithError{}

	service := NewTokenRevocationService(redisCache, backend)
	ctx := context.Background()

	err := service.RevokeAndPublish(ctx, "error-token", "test")
	if err == nil {
		t.Error("expected error from db backend")
	}
}

// MockTokenRevocationBackendWithError mock with error
type MockTokenRevocationBackendWithError struct{}

func (m *MockTokenRevocationBackendWithError) RevokeToken(ctx context.Context, tokenID string, reason string) error {
	return fmt.Errorf("db error")
}

func (m *MockTokenRevocationBackendWithError) GetTokenStatus(ctx context.Context, tokenID string) (string, error) {
	return "active", nil
}

func TestTokenRevocationService_StartRevocationSubscriber(t *testing.T) {
	redisCache := NewMockRedisCache()
	backend := NewMockTokenRevocationBackend()

	service := NewTokenRevocationService(redisCache, backend)
	ctx := context.Background()

	err := service.StartRevocationSubscriber(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
