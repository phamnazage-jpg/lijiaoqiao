package middleware

import (
	"context"
	"time"

	"lijiaoqiao/supply-api/internal/cache"
)

// TokenRevocationService Token吊销服务（P0-03修复）
// 实现主动失效机制，确保吊销传播延迟 <= 5s
type TokenRevocationService struct {
	redisCache TokenCacheBackend
	dbBackend  TokenRevocationBackend
}

// TokenRevocationBackend Token吊销数据库后端接口
type TokenRevocationBackend interface {
	// RevokeToken 在数据库中吊销token
	RevokeToken(ctx context.Context, tokenID string, reason string) error
	// GetTokenStatus 获取token状态
	GetTokenStatus(ctx context.Context, tokenID string) (string, error)
}

// NewTokenRevocationService 创建Token吊销服务
func NewTokenRevocationService(redisCache TokenCacheBackend, dbBackend TokenRevocationBackend) *TokenRevocationService {
	return &TokenRevocationService{
		redisCache: redisCache,
		dbBackend:  dbBackend,
	}
}

// RevokeAndPublish 吊销token并发布吊销事件（主动失效机制核心）
// 步骤：
// 1. 更新数据库状态
// 2. 发布吊销事件到Redis Pub/Sub
// 3. 返回成功（异步传播到所有缓存节点）
func (s *TokenRevocationService) RevokeAndPublish(ctx context.Context, tokenID string, reason string) error {
	// 1. 更新数据库状态（同步）
	if err := s.dbBackend.RevokeToken(ctx, tokenID, reason); err != nil {
		return err
	}

	// 2. 发布吊销事件到Redis Pub/Sub（异步触发主动失效）
	revokeEvent := &cache.TokenRevokedCacheEvent{
		TokenID:   tokenID,
		RevokedAt: time.Now(),
		Reason:    reason,
	}

	// 发布操作需要成功，否则缓存可能不会及时失效
	if err := s.redisCache.PublishTokenRevoked(ctx, revokeEvent); err != nil {
		// 发布失败时，至少要确保本地缓存失效
		s.redisCache.InvalidateToken(ctx, tokenID)
		return err
	}

	return nil
}

// StartRevocationSubscriber 启动吊销事件订阅者
// 在应用启动时调用，启动后台goroutine监听吊销事件
func (s *TokenRevocationService) StartRevocationSubscriber(ctx context.Context) error {
	return s.redisCache.SubscribeTokenRevoked(ctx, func(event *cache.TokenRevokedCacheEvent) {
		// 收到吊销事件，立即失效本地缓存
		s.redisCache.InvalidateToken(ctx, event.TokenID)
	})
}

// RevokeLocalOnly 仅在本地缓存失效（不发布事件，用于测试或特殊场景）
func (s *TokenRevocationService) RevokeLocalOnly(ctx context.Context, tokenID string) error {
	s.redisCache.InvalidateToken(ctx, tokenID)
	return nil
}
