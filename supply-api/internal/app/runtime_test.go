package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/cache"
	"lijiaoqiao/supply-api/internal/config"
	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/messaging"
	"lijiaoqiao/supply-api/internal/repository"
)

type captureLogger struct {
	infoMessages  []string
	warnMessages  []string
	errorMessages []string
}

func (l *captureLogger) Debug(string, ...map[string]interface{}) {}

func (l *captureLogger) Info(msg string, _ ...map[string]interface{}) {
	l.infoMessages = append(l.infoMessages, msg)
}

func (l *captureLogger) Warn(msg string, _ ...map[string]interface{}) {
	l.warnMessages = append(l.warnMessages, msg)
}

func (l *captureLogger) Error(msg string, _ ...map[string]interface{}) {
	l.errorMessages = append(l.errorMessages, msg)
}

func (l *captureLogger) Fatal(msg string, _ ...map[string]interface{}) {
	l.errorMessages = append(l.errorMessages, msg)
}

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

func TestResolveEnv_RejectsUnsupportedValue(t *testing.T) {
	_, err := ResolveEnv("qa")
	if err == nil {
		t.Fatal("expected unsupported env to fail")
	}
	if !strings.Contains(err.Error(), "unsupported env") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildRuntime_RejectsUnsupportedEnv(t *testing.T) {
	_, err := buildRuntimeWithFactory(RuntimeOptions{
		Env:         "qa",
		Config:      testRuntimeConfig(),
		Logger:      testLogger{},
		InitContext: context.Background(),
	}, runtimeFactory{
		newDB: func(context.Context, config.DatabaseConfig) (*repository.DB, error) {
			return nil, errors.New("db down")
		},
		newRedisCache: func(config.RedisConfig) (*cache.RedisCache, error) {
			return nil, nil
		},
	})
	if err == nil {
		t.Fatal("expected unsupported env to fail")
	}
	if !strings.Contains(err.Error(), "unsupported env") {
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

func TestBuildRuntime_NormalizesServerConfigDefaults(t *testing.T) {
	cfg := testRuntimeConfig()
	cfg.Server = config.ServerConfig{}

	runtime, err := buildRuntimeWithFactory(RuntimeOptions{
		Env:         "dev",
		Config:      cfg,
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
		t.Fatalf("expected runtime build to succeed, got %v", err)
	}
	if runtime.serverConfig.Addr != ":18082" {
		t.Fatalf("unexpected addr: %s", runtime.serverConfig.Addr)
	}
	if runtime.ShutdownTimeout() != 5*time.Second {
		t.Fatalf("unexpected shutdown timeout: %s", runtime.ShutdownTimeout())
	}
}

func TestBuildRuntime_SeedsDefaultTuning(t *testing.T) {
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
		t.Fatalf("expected runtime build to succeed, got %v", err)
	}
	if runtime.tuning.outboxStreamName != "supply:outbox:stream" {
		t.Fatalf("unexpected outbox stream: %s", runtime.tuning.outboxStreamName)
	}
	if runtime.tuning.outboxConsumerGroup != "outbox-processor" {
		t.Fatalf("unexpected outbox group: %s", runtime.tuning.outboxConsumerGroup)
	}
	if runtime.tuning.idempotencyTTL != 24*time.Hour {
		t.Fatalf("unexpected idempotency ttl: %s", runtime.tuning.idempotencyTTL)
	}
	if runtime.tuning.partitionMaintenanceInterval != time.Hour {
		t.Fatalf("unexpected partition maintenance interval: %s", runtime.tuning.partitionMaintenanceInterval)
	}
	if runtime.tuning.compensationCheckInterval != 5*time.Minute {
		t.Fatalf("unexpected compensation interval: %s", runtime.tuning.compensationCheckInterval)
	}
}

func TestBuildRuntime_DevFallbackLogsWarnings(t *testing.T) {
	logger := &captureLogger{}

	_, err := buildRuntimeWithFactory(RuntimeOptions{
		Env:         "dev",
		Config:      testRuntimeConfig(),
		Logger:      logger,
		InitContext: context.Background(),
	}, runtimeFactory{
		newDB: func(context.Context, config.DatabaseConfig) (*repository.DB, error) {
			return nil, errors.New("db down")
		},
		newRedisCache: func(config.RedisConfig) (*cache.RedisCache, error) {
			return nil, errors.New("redis down")
		},
	})
	if err != nil {
		t.Fatalf("expected runtime build to succeed, got %v", err)
	}
	if len(logger.warnMessages) == 0 {
		t.Fatal("expected warning logs during dev fallback")
	}
	if len(logger.infoMessages) == 0 {
		t.Fatal("expected info logs during successful in-memory runtime initialization")
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

func TestRuntime_StartBackgroundWorkers_UsesDefaultCompensationInterval(t *testing.T) {
	var gotInterval time.Duration

	err := startBackgroundWorkersWithFactory(context.Background(), context.Background(), &Runtime{
		env:    "dev",
		logger: testLogger{},
		db:     &repository.DB{},
		tuning: defaultRuntimeTuning(),
	}, backgroundFactory{
		newOutboxRepository: func(*repository.DB) outboxRepository {
			return stubOutboxRepository{}
		},
		newMessageBroker: func(*cache.RedisCache) messaging.MessageBroker {
			return stubMessageBroker{}
		},
		newOutboxRunner: func(outboxRepository, messaging.MessageBroker, messaging.OutboxStats) outboxRunner {
			return stubOutboxRunner{}
		},
		newPartitionManager: func(*repository.DB) partitionManager {
			return stubPartitionManager{}
		},
		newCompensationStore: func(*repository.DB) domain.CompensationStore {
			return stubCompensationStore{}
		},
		newCompensationExecutor: func() domain.OperationExecutor {
			return stubOperationExecutor{}
		},
		newCompensationProcessor: func(
			domain.CompensationStore,
			domain.OperationExecutor,
			domain.CompensationStats,
		) compensationWorker {
			return stubCompensationWorker{
				start: func(_ context.Context, interval time.Duration) {
					gotInterval = interval
				},
			}
		},
	})
	if err != nil {
		t.Fatalf("expected background startup to succeed, got %v", err)
	}
	if gotInterval != 5*time.Minute {
		t.Fatalf("unexpected compensation interval: %s", gotInterval)
	}
}

func TestRuntime_StartBackgroundWorkers_DevMissingOutboxBrokerLogsWarning(t *testing.T) {
	logger := &captureLogger{}

	err := startBackgroundWorkersWithFactory(context.Background(), context.Background(), &Runtime{
		env:    "dev",
		logger: logger,
		db:     &repository.DB{},
		tuning: defaultRuntimeTuning(),
	}, backgroundFactory{
		newOutboxRepository: func(*repository.DB) outboxRepository {
			return stubOutboxRepository{}
		},
		newMessageBroker: func(*cache.RedisCache) messaging.MessageBroker {
			return nil
		},
		newPartitionManager: func(*repository.DB) partitionManager {
			return stubPartitionManager{}
		},
		newCompensationStore: func(*repository.DB) domain.CompensationStore {
			return stubCompensationStore{}
		},
		newCompensationExecutor: func() domain.OperationExecutor {
			return stubOperationExecutor{}
		},
		newCompensationProcessor: func(
			domain.CompensationStore,
			domain.OperationExecutor,
			domain.CompensationStats,
		) compensationWorker {
			return stubCompensationWorker{}
		},
	})
	if err != nil {
		t.Fatalf("expected background startup to succeed, got %v", err)
	}
	if len(logger.warnMessages) == 0 {
		t.Fatal("expected warning log when outbox broker is unavailable in dev")
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

type stubMessageBroker struct{}

func (stubMessageBroker) Publish(context.Context, *repository.OutboxEvent) error {
	return nil
}

type stubOutboxRunner struct{}

func (stubOutboxRunner) Start(context.Context) {}

type stubPartitionManager struct{}

func (stubPartitionManager) EnsureFuturePartitions(context.Context) error {
	return nil
}

func (stubPartitionManager) DropOldPartitions(context.Context, string) (int, error) {
	return 0, nil
}

type stubCompensationStore struct{}

func (stubCompensationStore) Create(context.Context, *domain.BatchCompensation) (int64, error) {
	return 0, nil
}

func (stubCompensationStore) GetByBatchID(context.Context, string) ([]*domain.BatchCompensation, error) {
	return nil, nil
}

func (stubCompensationStore) GetPending(context.Context) ([]*domain.BatchCompensation, error) {
	return nil, nil
}

func (stubCompensationStore) UpdateStatus(context.Context, int64, string) error {
	return nil
}

func (stubCompensationStore) Resolve(context.Context, int64, int64, string) error {
	return nil
}

func (stubCompensationStore) MarkManualRequired(context.Context, int64, string) error {
	return nil
}

type stubOperationExecutor struct{}

func (stubOperationExecutor) Execute(context.Context, string, json.RawMessage) error {
	return nil
}

type stubCompensationWorker struct {
	start func(context.Context, time.Duration)
}

func (w stubCompensationWorker) StartBackgroundWorker(ctx context.Context, interval time.Duration) context.Context {
	if w.start != nil {
		w.start(ctx, interval)
	}
	return ctx
}
