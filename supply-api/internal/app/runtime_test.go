package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/cache"
	"lijiaoqiao/supply-api/internal/config"
	"lijiaoqiao/supply-api/internal/messaging"
	"lijiaoqiao/supply-api/internal/repository"
)

func TestBuildRuntime_ProdRequiresDatabase(t *testing.T) {
	_, err := buildRuntimeWithFactory(RuntimeOptions{
		Env:         "prod",
		Config:      testRuntimeConfig(),
		Logger:      testLogger{},
		InitContext: context.Background(),
		Now: func() time.Time {
			return time.Unix(1712800000, 0).UTC()
		},
	}, runtimeFactory{
		newDB: func(context.Context, config.DatabaseConfig) (*repository.DB, error) {
			return nil, errors.New("db down")
		},
		newRedisCache: func(config.RedisConfig) (*cache.RedisCache, error) {
			return nil, nil
		},
	})
	if err == nil {
		t.Fatal("expected prod runtime build to reject database outage")
	}
	if !strings.Contains(err.Error(), "database unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildRuntime_DevFallsBackToInMemoryDependencies(t *testing.T) {
	runtime, err := buildRuntimeWithFactory(RuntimeOptions{
		Env:         "dev",
		Config:      testRuntimeConfig(),
		Logger:      testLogger{},
		InitContext: context.Background(),
		Now: func() time.Time {
			return time.Unix(1712800000, 0).UTC()
		},
	}, runtimeFactory{
		newDB: func(context.Context, config.DatabaseConfig) (*repository.DB, error) {
			return nil, errors.New("db down")
		},
		newRedisCache: func(config.RedisConfig) (*cache.RedisCache, error) {
			return nil, errors.New("redis down")
		},
	})
	if err != nil {
		t.Fatalf("expected dev runtime to fall back to in-memory dependencies, got %v", err)
	}
	if runtime == nil {
		t.Fatal("expected runtime")
	}
	if runtime.db != nil {
		t.Fatal("expected nil db after dev fallback")
	}
	if runtime.redisCache != nil {
		t.Fatal("expected nil redis cache after dev fallback")
	}
	if runtime.supplyAPI == nil || runtime.alertAPI == nil {
		t.Fatal("expected apis to be initialized")
	}
	if runtime.authMiddleware == nil {
		t.Fatal("expected auth middleware to be initialized")
	}
	if runtime.rateLimitConfig == nil {
		t.Fatal("expected rate limit config to be initialized")
	}
}

func TestRuntime_StartBackgroundWorkers_WithoutDatabaseIsNoop(t *testing.T) {
	var outboxRepoCalled bool

	err := startBackgroundWorkersWithFactory(context.Background(), context.Background(), &Runtime{
		env:    "dev",
		logger: testLogger{},
	}, backgroundFactory{
		newOutboxRepository: func(*repository.DB) outboxRepository {
			outboxRepoCalled = true
			return stubOutboxRepository{}
		},
	})
	if err != nil {
		t.Fatalf("expected nil error when database is unavailable, got %v", err)
	}
	if outboxRepoCalled {
		t.Fatal("expected background workers to skip db-backed startup when db is nil")
	}
}

func TestRuntime_StartBackgroundWorkers_ProdRequiresOutboxBroker(t *testing.T) {
	err := startBackgroundWorkersWithFactory(context.Background(), context.Background(), &Runtime{
		env:    "prod",
		logger: testLogger{},
		db:     &repository.DB{},
	}, backgroundFactory{
		newOutboxRepository: func(*repository.DB) outboxRepository {
			return stubOutboxRepository{}
		},
		newMessageBroker: func(*cache.RedisCache) messaging.MessageBroker {
			return nil
		},
	})
	if err == nil {
		t.Fatal("expected missing outbox broker to fail in prod")
	}
	if !strings.Contains(err.Error(), "outbox message broker unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func testRuntimeConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Addr:              ":18082",
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       30 * time.Second,
			ShutdownTimeout:   5 * time.Second,
			DefaultSupplierID: 1,
			StatementBaseURL:  "https://statements.example.com",
		},
		Database: config.DatabaseConfig{
			Host:            "127.0.0.1",
			Port:            5432,
			User:            "test",
			Password:        "test",
			Database:        "supply",
			MaxOpenConns:    4,
			MaxIdleConns:    2,
			ConnMaxLifetime: time.Minute,
			ConnMaxIdleTime: time.Minute,
		},
		Redis: config.RedisConfig{
			Host:     "127.0.0.1",
			Port:     6379,
			Password: "",
			DB:       0,
			PoolSize: 2,
		},
		Token: config.TokenConfig{
			SecretKey:          "runtime-test-secret",
			Algorithm:          "HS256",
			Issuer:             "runtime-test",
			RevocationCacheTTL: 10 * time.Second,
		},
		Settlement: config.SettlementConfig{
			WithdrawEnabled: true,
		},
	}
}

type stubOutboxRepository struct{}

func (stubOutboxRepository) FetchAndLock(context.Context, int) ([]*repository.OutboxEvent, error) {
	return nil, nil
}

func (stubOutboxRepository) MarkCompleted(context.Context, string) error {
	return nil
}

func (stubOutboxRepository) MarkFailed(context.Context, string, string, *time.Time) error {
	return nil
}

func (stubOutboxRepository) MoveToDeadLetter(context.Context, *repository.OutboxEvent, string) error {
	return nil
}
