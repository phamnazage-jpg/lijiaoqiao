package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"lijiaoqiao/supply-api/internal/config"
)

// RedisCache Redis缓存客户端
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache 创建Redis缓存客户端
func NewRedisCache(cfg config.RedisConfig) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	// 验证连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisCache{client: client}, nil
}

// Close 关闭连接
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// HealthCheck 健康检查
func (r *RedisCache) HealthCheck(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// GetClient 获取原始Redis客户端（用于其他组件）
func (r *RedisCache) GetClient() *redis.Client {
	return r.client
}

// ==================== Token状态缓存 ====================

// TokenStatus Token状态
type TokenStatus struct {
	TokenID       string `json:"token_id"`
	SubjectID     string `json:"subject_id"`
	Role          string `json:"role"`
	Status        string `json:"status"` // active, revoked, expired
	ExpiresAt     int64  `json:"expires_at"`
	RevokedAt     int64  `json:"revoked_at,omitempty"`
	RevokedReason string `json:"revoked_reason,omitempty"`
}

// GetTokenStatus 获取Token状态
func (r *RedisCache) GetTokenStatus(ctx context.Context, tokenID string) (*TokenStatus, error) {
	key := fmt.Sprintf("token:status:%s", tokenID)
	data, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get token status: %w", err)
	}

	var status TokenStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token status: %w", err)
	}

	return &status, nil
}

// SetTokenStatus 设置Token状态
func (r *RedisCache) SetTokenStatus(ctx context.Context, status *TokenStatus, ttl time.Duration) error {
	key := fmt.Sprintf("token:status:%s", status.TokenID)
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to marshal token status: %w", err)
	}

	return r.client.Set(ctx, key, data, ttl).Err()
}

// InvalidateToken 使Token失效
func (r *RedisCache) InvalidateToken(ctx context.Context, tokenID string) error {
	key := fmt.Sprintf("token:status:%s", tokenID)
	return r.client.Del(ctx, key).Err()
}

// PublishTokenRevoked 发布Token吊销事件（用于主动失效机制 P0-03）
func (r *RedisCache) PublishTokenRevoked(ctx context.Context, event *TokenRevokedCacheEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal revocation event: %w", err)
	}
	return r.client.Publish(ctx, "token:revoked", data).Err()
}

// SubscribeTokenRevoked 订阅Token吊销事件（用于主动失效机制 P0-03）
func (r *RedisCache) SubscribeTokenRevoked(ctx context.Context, handler func(*TokenRevokedCacheEvent)) error {
	pubsub := r.client.Subscribe(ctx, "token:revoked")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-ch:
			var event TokenRevokedCacheEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				continue // 忽略解析错误
			}
			handler(&event)
		}
	}
}

// TokenRevokedCacheEvent Token吊销缓存事件
type TokenRevokedCacheEvent struct {
	TokenID   string    `json:"token_id"`
	RevokedAt time.Time `json:"revoked_at"`
	Reason    string    `json:"reason"`
}

// ==================== 限流 ====================

// RateLimitKey 限流键
type RateLimitKey struct {
	TenantID  int64
	Route     string
	LimitType string // rpm, rpd, concurrent
}

// GetRateLimit 获取限流计数
func (r *RedisCache) GetRateLimit(ctx context.Context, key *RateLimitKey, window time.Duration) (int64, error) {
	redisKey := fmt.Sprintf("ratelimit:%d:%s:%s", key.TenantID, key.Route, key.LimitType)

	count, err := r.client.Get(ctx, redisKey).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get rate limit: %w", err)
	}

	return count, nil
}

// IncrRateLimit 增加限流计数
func (r *RedisCache) IncrRateLimit(ctx context.Context, key *RateLimitKey, window time.Duration) (int64, error) {
	redisKey := fmt.Sprintf("ratelimit:%d:%s:%s", key.TenantID, key.Route, key.LimitType)

	pipe := r.client.Pipeline()
	incrCmd := pipe.Incr(ctx, redisKey)
	pipe.Expire(ctx, redisKey, window)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to increment rate limit: %w", err)
	}

	return incrCmd.Val(), nil
}

// CheckRateLimit 检查限流
func (r *RedisCache) CheckRateLimit(ctx context.Context, key *RateLimitKey, limit int64, window time.Duration) (bool, int64, error) {
	count, err := r.IncrRateLimit(ctx, key, window)
	if err != nil {
		return false, 0, err
	}

	return count <= limit, count, nil
}

// ==================== 分布式锁 ====================

// AcquireLock 获取分布式锁
func (r *RedisCache) AcquireLock(ctx context.Context, lockKey string, ttl time.Duration) (bool, error) {
	redisKey := fmt.Sprintf("lock:%s", lockKey)

	ok, err := r.client.SetNX(ctx, redisKey, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	return ok, nil
}

// ReleaseLock 释放分布式锁
func (r *RedisCache) ReleaseLock(ctx context.Context, lockKey string) error {
	redisKey := fmt.Sprintf("lock:%s", lockKey)
	return r.client.Del(ctx, redisKey).Err()
}

// ==================== 幂等缓存 ====================

// IdempotencyCache 幂等缓存（短期）
func (r *RedisCache) GetIdempotency(ctx context.Context, key string) (string, error) {
	redisKey := fmt.Sprintf("idempotency:%s", key)
	val, err := r.client.Get(ctx, redisKey).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get idempotency: %w", err)
	}
	return val, nil
}

func (r *RedisCache) SetIdempotency(ctx context.Context, key, value string, ttl time.Duration) error {
	redisKey := fmt.Sprintf("idempotency:%s", key)
	return r.client.Set(ctx, redisKey, value, ttl).Err()
}

// ==================== Session缓存 ====================

// SessionData Session数据
type SessionData struct {
	UserID    int64  `json:"user_id"`
	TenantID  int64  `json:"tenant_id"`
	Role      string `json:"role"`
	CreatedAt int64  `json:"created_at"`
}

// GetSession 获取Session
func (r *RedisCache) GetSession(ctx context.Context, sessionID string) (*SessionData, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	data, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// SetSession 设置Session
func (r *RedisCache) SetSession(ctx context.Context, sessionID string, session *SessionData, ttl time.Duration) error {
	key := fmt.Sprintf("session:%s", sessionID)
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	return r.client.Set(ctx, key, data, ttl).Err()
}

// DeleteSession 删除Session
func (r *RedisCache) DeleteSession(ctx context.Context, sessionID string) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return r.client.Del(ctx, key).Err()
}
