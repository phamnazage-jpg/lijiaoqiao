package middleware

import (
	"context"
	"fmt"
	"time"

	"lijiaoqiao/supply-api/internal/cache"
	"lijiaoqiao/supply-api/internal/repository"
)

// ==================== 接口定义 ====================

// TokenRepository Token状态仓储接口
type TokenRepository interface {
	GetStatus(ctx context.Context, tokenID string) (string, error)
	Revoke(ctx context.Context, tokenID string, reason string) error
	UpdateVerificationCount(ctx context.Context, tokenID string) error
	RevokeBySubjectID(ctx context.Context, subjectID int64, reason string) (int64, error)
	ListActiveBySubjectID(ctx context.Context, subjectID int64) ([]*repository.TokenStatusRecord, error)
}

// TokenCacheBackend Token缓存接口（用于测试mock）
type TokenCacheBackend interface {
	GetTokenStatus(ctx context.Context, tokenID string) (*cache.TokenStatus, error)
	SetTokenStatus(ctx context.Context, status *cache.TokenStatus, ttl time.Duration) error
	InvalidateToken(ctx context.Context, tokenID string) error
	SubscribeTokenRevoked(ctx context.Context, handler func(event *cache.TokenRevokedCacheEvent)) error
	PublishTokenRevoked(ctx context.Context, event *cache.TokenRevokedCacheEvent) error
}

// ==================== DB-backed Token状态后端实现 ====================

// DBTokenStatusBackend DB-backed Token状态后端（P0-03修复）
// 同时实现 TokenStatusBackend 和 TokenRevocationBackend 接口
type DBTokenStatusBackend struct {
	repo        TokenRepository
	redisCache  TokenCacheBackend
	cacheTTL    time.Duration
}

// NewDBTokenStatusBackend 创建DB-backed Token状态后端
func NewDBTokenStatusBackend(repo TokenRepository, redisCache TokenCacheBackend, cacheTTL time.Duration) *DBTokenStatusBackend {
	if cacheTTL == 0 {
		cacheTTL = 10 * time.Second // 默认10s缓存
	}
	return &DBTokenStatusBackend{
		repo:       repo,
		redisCache: redisCache,
		cacheTTL:   cacheTTL,
	}
}

// Ensure interface - compile time check
var _ TokenStatusBackend = (*DBTokenStatusBackend)(nil)
var _ TokenRevocationBackend = (*DBTokenStatusBackend)(nil)

// CheckTokenStatus 检查Token状态（实现 TokenStatusBackend 接口）
// 流程：
// 1. 先查Redis缓存
// 2. 缓存未命中查DB
// 3. 更新缓存和验证计数
func (b *DBTokenStatusBackend) CheckTokenStatus(ctx context.Context, tokenID string) (string, error) {
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
		return "", fmt.Errorf("failed to get token status: %w", err)
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

	// 4. 异步更新验证计数（不阻塞验证流程）
	go func() {
		_ = b.repo.UpdateVerificationCount(context.Background(), tokenID)
	}()

	return status, nil
}

// RevokeToken 吊销Token（实现 TokenRevocationBackend 接口）
func (b *DBTokenStatusBackend) RevokeToken(ctx context.Context, tokenID string, reason string) error {
	// 1. 更新数据库状态
	if err := b.repo.Revoke(ctx, tokenID, reason); err != nil {
		return fmt.Errorf("failed to revoke token in db: %w", err)
	}

	// 2. 失效Redis缓存
	if b.redisCache != nil {
		if err := b.redisCache.InvalidateToken(ctx, tokenID); err != nil {
			// 缓存失效失败不影响业务逻辑
			return nil
		}
	}

	return nil
}

// GetTokenStatus 获取Token状态（实现 TokenRevocationBackend 接口）
// 与 CheckTokenStatus 逻辑相同
func (b *DBTokenStatusBackend) GetTokenStatus(ctx context.Context, tokenID string) (string, error) {
	return b.CheckTokenStatus(ctx, tokenID)
}

// RevokeBySubjectID 根据SubjectID吊销所有Token
func (b *DBTokenStatusBackend) RevokeBySubjectID(ctx context.Context, subjectID int64, reason string) error {
	// 1. 批量更新数据库
	count, err := b.repo.RevokeBySubjectID(ctx, subjectID, reason)
	if err != nil {
		return fmt.Errorf("failed to revoke tokens by subject_id: %w", err)
	}

	if count == 0 {
		return nil
	}

	// 2. 失效所有相关缓存（这里需要查询后逐个失效）
	// 注意：生产环境建议使用Redis的pattern删除或发布事件通知
	if b.redisCache != nil {
		// 查询所有活跃token并失效
		records, err := b.repo.ListActiveBySubjectID(ctx, subjectID)
		if err != nil {
			return nil // 不影响主流程
		}
		for _, record := range records {
			_ = b.redisCache.InvalidateToken(ctx, record.TokenID)
		}
	}

	return nil
}

// StartRevocationSubscriber 启动吊销事件订阅（用于主动失效机制）
// 在应用启动时调用，启动后台goroutine监听吊销事件
func (b *DBTokenStatusBackend) StartRevocationSubscriber(ctx context.Context) error {
	if b.redisCache == nil {
		return fmt.Errorf("redis cache is required for revocation subscriber")
	}

	return b.redisCache.SubscribeTokenRevoked(ctx, func(event *cache.TokenRevokedCacheEvent) {
		// 收到吊销事件，立即失效本地缓存
		_ = b.redisCache.InvalidateToken(ctx, event.TokenID)
	})
}
