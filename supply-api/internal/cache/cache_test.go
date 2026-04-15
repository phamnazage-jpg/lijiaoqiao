package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type fakeRedisClient struct {
	values      map[string]string
	expirations map[string]time.Duration
	published   []publishedMessage
}

type publishedMessage struct {
	channel string
	payload string
}

type fakeRedisPipeline struct {
	client *fakeRedisClient
	cmds   []redis.Cmder
}

func newFakeRedisClient() *fakeRedisClient {
	return &fakeRedisClient{
		values:      make(map[string]string),
		expirations: make(map[string]time.Duration),
	}
}

func (c *fakeRedisClient) Close() error {
	return nil
}

func (c *fakeRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	return redis.NewStatusResult("PONG", nil)
}

func (c *fakeRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	val, ok := c.values[key]
	if !ok {
		return redis.NewStringResult("", redis.Nil)
	}
	return redis.NewStringResult(val, nil)
}

func (c *fakeRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	c.values[key] = stringifyRedisValue(value)
	c.expirations[key] = expiration
	return redis.NewStatusResult("OK", nil)
}

func (c *fakeRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	var removed int64
	for _, key := range keys {
		if _, ok := c.values[key]; ok {
			delete(c.values, key)
			delete(c.expirations, key)
			removed++
		}
	}
	return redis.NewIntResult(removed, nil)
}

func (c *fakeRedisClient) Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd {
	c.published = append(c.published, publishedMessage{
		channel: channel,
		payload: stringifyRedisValue(message),
	})
	return redis.NewIntResult(1, nil)
}

func (c *fakeRedisClient) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	panic("Subscribe should not be called in cache unit tests")
}

func (c *fakeRedisClient) Pipeline() redisPipeline {
	return &fakeRedisPipeline{client: c}
}

func (c *fakeRedisClient) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
	if _, exists := c.values[key]; exists {
		return redis.NewBoolResult(false, nil)
	}
	c.values[key] = stringifyRedisValue(value)
	c.expirations[key] = expiration
	return redis.NewBoolResult(true, nil)
}

func (p *fakeRedisPipeline) Incr(ctx context.Context, key string) *redis.IntCmd {
	var current int64
	if raw, ok := p.client.values[key]; ok {
		if _, err := fmt.Sscanf(raw, "%d", &current); err != nil {
			return redis.NewIntResult(0, err)
		}
	}
	current++
	p.client.values[key] = fmt.Sprintf("%d", current)
	cmd := redis.NewIntResult(current, nil)
	p.cmds = append(p.cmds, cmd)
	return cmd
}

func (p *fakeRedisPipeline) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	p.client.expirations[key] = expiration
	cmd := redis.NewBoolResult(true, nil)
	p.cmds = append(p.cmds, cmd)
	return cmd
}

func (p *fakeRedisPipeline) Exec(ctx context.Context) ([]redis.Cmder, error) {
	return p.cmds, nil
}

func stringifyRedisValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}

func TestRedisCacheTokenStatusAndSessionRoundTrip(t *testing.T) {
	ctx := context.Background()
	client := newFakeRedisClient()
	cache := newRedisCacheWithClient(client)

	status := &TokenStatus{
		TokenID:   "tok-1",
		SubjectID: "sub-1",
		Role:      "owner",
		Status:    "active",
		ExpiresAt: 1710000000,
	}
	if err := cache.SetTokenStatus(ctx, status, 5*time.Minute); err != nil {
		t.Fatalf("SetTokenStatus() error = %v", err)
	}

	gotStatus, err := cache.GetTokenStatus(ctx, "tok-1")
	if err != nil {
		t.Fatalf("GetTokenStatus() error = %v", err)
	}
	if gotStatus == nil || gotStatus.TokenID != status.TokenID || gotStatus.SubjectID != status.SubjectID {
		t.Fatalf("unexpected token status: %#v", gotStatus)
	}
	if client.expirations["token:status:tok-1"] != 5*time.Minute {
		t.Fatalf("token status ttl = %s, want %s", client.expirations["token:status:tok-1"], 5*time.Minute)
	}

	session := &SessionData{
		UserID:    7,
		TenantID:  9,
		Role:      "viewer",
		CreatedAt: 1710000001,
	}
	if err := cache.SetSession(ctx, "sess-1", session, time.Hour); err != nil {
		t.Fatalf("SetSession() error = %v", err)
	}

	gotSession, err := cache.GetSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if gotSession == nil || gotSession.UserID != session.UserID || gotSession.Role != session.Role {
		t.Fatalf("unexpected session: %#v", gotSession)
	}
	if client.expirations["session:sess-1"] != time.Hour {
		t.Fatalf("session ttl = %s, want %s", client.expirations["session:sess-1"], time.Hour)
	}
}

func TestRedisCacheMissingValueSemantics(t *testing.T) {
	ctx := context.Background()
	cache := newRedisCacheWithClient(newFakeRedisClient())

	tokenStatus, err := cache.GetTokenStatus(ctx, "missing-token")
	if err != nil {
		t.Fatalf("GetTokenStatus() error = %v", err)
	}
	if tokenStatus != nil {
		t.Fatalf("expected nil token status, got %#v", tokenStatus)
	}

	idempotency, err := cache.GetIdempotency(ctx, "missing-key")
	if err != nil {
		t.Fatalf("GetIdempotency() error = %v", err)
	}
	if idempotency != "" {
		t.Fatalf("expected empty idempotency value, got %q", idempotency)
	}

	count, err := cache.GetRateLimit(ctx, &RateLimitKey{TenantID: 1, Route: "/v1/chat", LimitType: "rpm"}, time.Minute)
	if err != nil {
		t.Fatalf("GetRateLimit() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("rate limit count = %d, want 0", count)
	}
}

func TestRedisCacheRateLimitAndLockSemantics(t *testing.T) {
	ctx := context.Background()
	client := newFakeRedisClient()
	cache := newRedisCacheWithClient(client)
	key := &RateLimitKey{TenantID: 42, Route: "/v1/chat", LimitType: "rpm"}

	count, err := cache.IncrRateLimit(ctx, key, time.Minute)
	if err != nil {
		t.Fatalf("IncrRateLimit() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("first count = %d, want 1", count)
	}

	allowed, count, err := cache.CheckRateLimit(ctx, key, 2, time.Minute)
	if err != nil {
		t.Fatalf("CheckRateLimit() error = %v", err)
	}
	if !allowed || count != 2 {
		t.Fatalf("second check = (%v, %d), want (true, 2)", allowed, count)
	}

	allowed, count, err = cache.CheckRateLimit(ctx, key, 2, time.Minute)
	if err != nil {
		t.Fatalf("CheckRateLimit() error = %v", err)
	}
	if allowed || count != 3 {
		t.Fatalf("third check = (%v, %d), want (false, 3)", allowed, count)
	}

	if client.expirations["ratelimit:42:/v1/chat:rpm"] != time.Minute {
		t.Fatalf("rate limit ttl = %s, want %s", client.expirations["ratelimit:42:/v1/chat:rpm"], time.Minute)
	}

	acquired, err := cache.AcquireLock(ctx, "sync-job", 30*time.Second)
	if err != nil {
		t.Fatalf("AcquireLock() error = %v", err)
	}
	if !acquired {
		t.Fatal("first AcquireLock() should succeed")
	}

	acquired, err = cache.AcquireLock(ctx, "sync-job", 30*time.Second)
	if err != nil {
		t.Fatalf("AcquireLock() error = %v", err)
	}
	if acquired {
		t.Fatal("second AcquireLock() should fail while lock exists")
	}

	if err := cache.ReleaseLock(ctx, "sync-job"); err != nil {
		t.Fatalf("ReleaseLock() error = %v", err)
	}
	if _, exists := client.values["lock:sync-job"]; exists {
		t.Fatal("lock key should be deleted after ReleaseLock()")
	}
}

func TestRedisCachePublishTokenRevokedMarshalsEvent(t *testing.T) {
	ctx := context.Background()
	client := newFakeRedisClient()
	cache := newRedisCacheWithClient(client)
	event := &TokenRevokedCacheEvent{
		TokenID:   "tok-9",
		RevokedAt: time.Unix(1710001111, 0).UTC(),
		Reason:    "manual revoke",
	}

	if err := cache.PublishTokenRevoked(ctx, event); err != nil {
		t.Fatalf("PublishTokenRevoked() error = %v", err)
	}
	if len(client.published) != 1 {
		t.Fatalf("published message count = %d, want 1", len(client.published))
	}
	if client.published[0].channel != "token:revoked" {
		t.Fatalf("published channel = %q, want %q", client.published[0].channel, "token:revoked")
	}

	var got TokenRevokedCacheEvent
	if err := json.Unmarshal([]byte(client.published[0].payload), &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.TokenID != event.TokenID || got.Reason != event.Reason || !got.RevokedAt.Equal(event.RevokedAt) {
		t.Fatalf("unexpected event payload: %#v", got)
	}
}
