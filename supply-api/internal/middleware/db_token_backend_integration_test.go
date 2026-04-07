//go:build integration
// +build integration

package middleware

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/cache"
	"lijiaoqiao/supply-api/internal/config"
	"lijiaoqiao/supply-api/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// integrationTestDB holds database connection for integration tests
type integrationTestDB struct {
	pool *pgxpool.Pool
	redis *redis.Client
}

// setupIntegrationTest initializes real database connections for testing
func setupIntegrationTest(t *testing.T) (*integrationTestDB, func()) {
	// Get connection strings from environment or use defaults
	pgURL := os.Getenv("SUPPLY_TEST_POSTGRES")
	if pgURL == "" {
		pgURL = "postgres://supply_test:supply_test_pass@localhost:5432/supply_test?sslmode=disable"
	}

	redisAddr := os.Getenv("SUPPLY_TEST_REDIS")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// Connect to PostgreSQL
	ctx := context.Background()
	poolConfig, err := pgxpool.ParseConfig(pgURL)
	if err != nil {
		t.Skipf("Skipping integration test: cannot parse postgres config: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Skipf("Skipping integration test: cannot connect to postgres: %v", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("Skipping integration test: cannot ping postgres: %v", err)
	}

	// Connect to Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	if err := redisClient.Ping(ctx).Err(); err != nil {
		pool.Close()
		t.Skipf("Skipping integration test: cannot connect to redis: %v", err)
	}

	// Setup schema
	setupSchema(t, ctx, pool)

	return &integrationTestDB{
		pool:  pool,
		redis: redisClient,
	}, func() {
		pool.Close()
		redisClient.Close()
	}
}

// setupSchema creates the required tables for testing
func setupSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	// Create enum type
	_, err := pool.Exec(ctx, `
		DO $$ BEGIN
			CREATE TYPE token_status AS ENUM ('active', 'revoked', 'expired');
		EXCEPTION
			WHEN duplicate_object THEN null;
		END $$;
	`)
	if err != nil {
		t.Fatalf("Failed to create enum: %v", err)
	}

	// Create table
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS token_status_registry (
			id              BIGSERIAL PRIMARY KEY,
			token_id        VARCHAR(128) NOT NULL UNIQUE,
			subject_id      BIGINT NOT NULL,
			tenant_id       BIGINT NOT NULL,
			role            VARCHAR(50) NOT NULL DEFAULT 'user',
			status          token_status NOT NULL DEFAULT 'active',
			issued_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at      TIMESTAMPTZ NOT NULL,
			revoked_at      TIMESTAMPTZ,
			revoked_reason  VARCHAR(256),
			revoked_by      BIGINT,
			last_verified_at TIMESTAMPTZ,
			verification_count BIGINT NOT NULL DEFAULT 0,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
}

// cleanupTable cleans up test data
func cleanupTable(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	_, _ = pool.Exec(ctx, "DELETE FROM token_status_registry")
}

// TestDBTokenStatusBackend_Integration_CheckTokenStatus_CacheHit tests with real Redis
func TestDBTokenStatusBackend_Integration_CheckTokenStatus_CacheHit(t *testing.T) {
	db, cleanup := setupIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	cleanupTable(t, ctx, db.pool)

	// Create real Redis cache
	redisCache, err := cache.NewRedisCache(config.RedisConfig{
		Host:     db.redis.Options().Addr,
		Password: "",
		DB:       0,
		PoolSize: 10,
	})
	if err != nil {
		t.Skipf("Skipping: cannot create redis cache: %v", err)
	}

	// Create real repository
	repo := repository.NewTokenStatusRepository(db.pool)

	// Create backend with real dependencies
	backend := NewDBTokenStatusBackend(repo, redisCache, 10*time.Second)

	// Insert test token
	_, err = db.pool.Exec(ctx, `
		INSERT INTO token_status_registry (token_id, subject_id, tenant_id, status, expires_at)
		VALUES ($1, 1, 1, 'active', NOW() + INTERVAL '1 hour')
	`, "integration-test-token-1")
	if err != nil {
		t.Fatalf("Failed to insert test token: %v", err)
	}

	// First call - cache miss
	status1, err := backend.CheckTokenStatus(ctx, "integration-test-token-1")
	if err != nil {
		t.Fatalf("CheckTokenStatus failed: %v", err)
	}
	if status1 != "active" {
		t.Errorf("expected status 'active', got '%s'", status1)
	}

	// Second call - should be cache hit
	status2, err := backend.CheckTokenStatus(ctx, "integration-test-token-1")
	if err != nil {
		t.Fatalf("CheckTokenStatus failed on second call: %v", err)
	}
	if status2 != "active" {
		t.Errorf("expected status 'active' from cache, got '%s'", status2)
	}

	t.Log("Integration test passed: CheckTokenStatus with real Redis cache")
}

// TestDBTokenStatusBackend_Integration_RevokeToken tests with real DB and Redis
func TestDBTokenStatusBackend_Integration_RevokeToken(t *testing.T) {
	db, cleanup := setupIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	cleanupTable(t, ctx, db.pool)

	// Create real Redis cache
	redisCache, err := cache.NewRedisCache(config.RedisConfig{
		Host:     db.redis.Options().Addr,
		Password: "",
		DB:       0,
		PoolSize: 10,
	})
	if err != nil {
		t.Skipf("Skipping: cannot create redis cache: %v", err)
	}

	// Create real repository
	repo := repository.NewTokenStatusRepository(db.pool)

	// Create backend
	backend := NewDBTokenStatusBackend(repo, redisCache, 10*time.Second)

	// Insert test token
	_, err = db.pool.Exec(ctx, `
		INSERT INTO token_status_registry (token_id, subject_id, tenant_id, status, expires_at)
		VALUES ($1, 1, 1, 'active', NOW() + INTERVAL '1 hour')
	`, "integration-test-revoke-token")
	if err != nil {
		t.Fatalf("Failed to insert test token: %v", err)
	}

	// Verify token is active
	status, err := backend.CheckTokenStatus(ctx, "integration-test-revoke-token")
	if err != nil {
		t.Fatalf("CheckTokenStatus failed: %v", err)
	}
	if status != "active" {
		t.Errorf("expected status 'active', got '%s'", status)
	}

	// Revoke the token
	err = backend.RevokeToken(ctx, "integration-test-revoke-token", "integration test revocation")
	if err != nil {
		t.Fatalf("RevokeToken failed: %v", err)
	}

	// Verify token is revoked
	status, err = backend.CheckTokenStatus(ctx, "integration-test-revoke-token")
	if err != nil {
		t.Fatalf("CheckTokenStatus failed after revocation: %v", err)
	}
	if status != "revoked" {
		t.Errorf("expected status 'revoked', got '%s'", status)
	}

	t.Log("Integration test passed: RevokeToken with real DB and Redis")
}

// TestDBTokenStatusBackend_Integration_RevokeBySubjectID tests batch revocation
func TestDBTokenStatusBackend_Integration_RevokeBySubjectID(t *testing.T) {
	db, cleanup := setupIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	cleanupTable(t, ctx, db.pool)

	// Create real Redis cache
	redisCache, err := cache.NewRedisCache(config.RedisConfig{
		Host:     db.redis.Options().Addr,
		Password: "",
		DB:       0,
		PoolSize: 10,
	})
	if err != nil {
		t.Skipf("Skipping: cannot create redis cache: %v", err)
	}

	// Create real repository
	repo := repository.NewTokenStatusRepository(db.pool)

	// Create backend
	backend := NewDBTokenStatusBackend(repo, redisCache, 10*time.Second)

	// Insert multiple test tokens for same subject
	subjectID := int64(99999)
	for i := 0; i < 5; i++ {
		tokenID := fmt.Sprintf("integration-test-batch-token-%d", i)
		_, err = db.pool.Exec(ctx, `
			INSERT INTO token_status_registry (token_id, subject_id, tenant_id, status, expires_at)
			VALUES ($1, $2, 1, 'active', NOW() + INTERVAL '1 hour')
		`, tokenID, subjectID)
		if err != nil {
			t.Fatalf("Failed to insert test token %s: %v", tokenID, err)
		}
	}

	// Revoke all tokens for subject
	err = backend.RevokeBySubjectID(ctx, subjectID, "batch integration test revocation")
	if err != nil {
		t.Fatalf("RevokeBySubjectID failed: %v", err)
	}

	// Verify all tokens are revoked
	for i := 0; i < 5; i++ {
		tokenID := fmt.Sprintf("integration-test-batch-token-%d", i)
		status, err := backend.CheckTokenStatus(ctx, tokenID)
		if err != nil {
			t.Fatalf("CheckTokenStatus failed for %s: %v", tokenID, err)
		}
		if status != "revoked" {
			t.Errorf("expected status 'revoked' for %s, got '%s'", tokenID, status)
		}
	}

	t.Log("Integration test passed: RevokeBySubjectID with real DB and Redis")
}

// TestTokenRevocationService_Integration tests with real Redis Pub/Sub
func TestTokenRevocationService_Integration(t *testing.T) {
	db, cleanup := setupIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	cleanupTable(t, ctx, db.pool)

	// Create real Redis cache
	redisCache, err := cache.NewRedisCache(config.RedisConfig{
		Host:     db.redis.Options().Addr,
		Password: "",
		DB:       0,
		PoolSize: 10,
	})
	if err != nil {
		t.Skipf("Skipping: cannot create redis cache: %v", err)
	}

	// Create real repository
	repo := repository.NewTokenStatusRepository(db.pool)

	// Create backend
	backend := NewDBTokenStatusBackend(repo, redisCache, 10*time.Second)

	// Create revocation service
	revocationService := NewTokenRevocationService(redisCache, backend)

	// Insert test token
	_, err = db.pool.Exec(ctx, `
		INSERT INTO token_status_registry (token_id, subject_id, tenant_id, status, expires_at)
		VALUES ($1, 1, 1, 'active', NOW() + INTERVAL '1 hour')
	`, "integration-test-revocation-service-token")
	if err != nil {
		t.Fatalf("Failed to insert test token: %v", err)
	}

	// Revoke and publish
	err = revocationService.RevokeAndPublish(ctx, "integration-test-revocation-service-token", "integration test")
	if err != nil {
		t.Fatalf("RevokeAndPublish failed: %v", err)
	}

	// Verify token is revoked
	status, err := backend.CheckTokenStatus(ctx, "integration-test-revocation-service-token")
	if err != nil {
		t.Fatalf("CheckTokenStatus failed: %v", err)
	}
	if status != "revoked" {
		t.Errorf("expected status 'revoked', got '%s'", status)
	}

	t.Log("Integration test passed: RevocationService with real Redis Pub/Sub")
}
