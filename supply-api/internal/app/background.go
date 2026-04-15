package app

import (
	"context"
	"errors"
	"time"

	"lijiaoqiao/supply-api/internal/cache"
	"lijiaoqiao/supply-api/internal/compensation"
	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/messaging"
	"lijiaoqiao/supply-api/internal/outbox"
	"lijiaoqiao/supply-api/internal/pkg/logging"
	"lijiaoqiao/supply-api/internal/repository"
)

type outboxRepository interface {
	FetchAndLock(ctx context.Context, limit int) ([]*repository.OutboxEvent, error)
	MarkCompleted(ctx context.Context, eventID string) error
	MarkFailed(ctx context.Context, eventID string, errorMsg string, nextRetryAt *time.Time) error
	MoveToDeadLetter(ctx context.Context, event *repository.OutboxEvent, errorMsg string) error
}

type outboxRunner interface {
	Start(ctx context.Context)
}

type partitionManager interface {
	EnsureFuturePartitions(ctx context.Context) error
	DropOldPartitions(ctx context.Context, tableName string) (int, error)
}

type compensationWorker interface {
	StartBackgroundWorker(ctx context.Context, interval time.Duration) context.Context
}

type backgroundFactory struct {
	newOutboxRepository      func(db *repository.DB) outboxRepository
	newMessageBroker         func(redisCache *cache.RedisCache) messaging.MessageBroker
	newOutboxRunner          func(repo outboxRepository, broker messaging.MessageBroker, stats messaging.OutboxStats) outboxRunner
	newPartitionManager      func(db *repository.DB) partitionManager
	newCompensationStore     func(db *repository.DB) domain.CompensationStore
	newCompensationExecutor  func() domain.OperationExecutor
	newCompensationProcessor func(
		store domain.CompensationStore,
		executor domain.OperationExecutor,
		stats domain.CompensationStats,
	) compensationWorker
}

// StartBackgroundWorkers 启动后台依赖服务。
func (r *Runtime) StartBackgroundWorkers(rootCtx, initCtx context.Context) error {
	return startBackgroundWorkersWithFactory(rootCtx, initCtx, r, backgroundFactory{})
}

func startBackgroundWorkersWithFactory(
	rootCtx context.Context,
	initCtx context.Context,
	runtime *Runtime,
	factory backgroundFactory,
) error {
	if runtime == nil {
		return errors.New("runtime is required")
	}
	if runtime.logger == nil {
		return errors.New("runtime logger is required")
	}
	if rootCtx == nil {
		rootCtx = context.Background()
	}
	if initCtx == nil {
		initCtx = rootCtx
	}

	factory = withDefaultBackgroundFactory(factory)

	if runtime.revocationSubscriber != nil && runtime.redisCache != nil {
		if err := runtime.revocationSubscriber.StartRevocationSubscriber(rootCtx); err != nil {
			infof(runtime.logger, "警告: 启动主动吊销订阅失败: %v", err)
		} else {
			runtime.logger.Info("主动吊销机制: 已启动 (Redis Pub/Sub)", nil)
		}
	}

	if runtime.db == nil {
		return nil
	}

	outboxRepo := factory.newOutboxRepository(runtime.db)
	msgBroker := factory.newMessageBroker(runtime.redisCache)
	if msgBroker == nil {
		if runtime.env == "prod" {
			return errors.New("outbox message broker unavailable")
		}
		runtime.logger.Info("警告: OutboxProcessor未启动 (message broker不可用)", nil)
	} else {
		stats := &messaging.NoOpOutboxStats{}
		runner := factory.newOutboxRunner(outboxRepo, msgBroker, stats)
		go runner.Start(rootCtx)
		runtime.logger.Info("OutboxProcessor已启动", nil)
	}

	partitionManager := factory.newPartitionManager(runtime.db)
	if err := partitionManager.EnsureFuturePartitions(initCtx); err != nil {
		infof(runtime.logger, "警告: 预创建未来分区失败: %v", err)
	} else {
		runtime.logger.Info("分区管理: 未来分区已确保存在", nil)
	}

	go startPartitionMaintenance(rootCtx, runtime.logger, partitionManager)

	compensationStore := factory.newCompensationStore(runtime.db)
	compensationStats := &domain.NoOpCompensationStats{}
	compensationExecutor := factory.newCompensationExecutor()
	compensationProcessor := factory.newCompensationProcessor(compensationStore, compensationExecutor, compensationStats)
	runtime.logger.Info("批量补偿处理器: 已初始化", nil)
	compensationProcessor.StartBackgroundWorker(rootCtx, 5*time.Minute)
	runtime.logger.Info("批量补偿处理器: 后台worker已启动 (每5分钟检查一次)", nil)

	return nil
}

func withDefaultBackgroundFactory(factory backgroundFactory) backgroundFactory {
	if factory.newOutboxRepository == nil {
		factory.newOutboxRepository = func(db *repository.DB) outboxRepository {
			return repository.NewOutboxRepository(db.Pool)
		}
	}
	if factory.newMessageBroker == nil {
		factory.newMessageBroker = func(redisCache *cache.RedisCache) messaging.MessageBroker {
			if redisCache == nil {
				return nil
			}
			return messaging.NewOutboxMessageBroker(redisCache.GetClient(), "supply:outbox:stream", "outbox-processor")
		}
	}
	if factory.newOutboxRunner == nil {
		factory.newOutboxRunner = func(repo outboxRepository, broker messaging.MessageBroker, stats messaging.OutboxStats) outboxRunner {
			return outbox.NewOutboxProcessorRunner(repo, broker, stats)
		}
	}
	if factory.newPartitionManager == nil {
		factory.newPartitionManager = func(db *repository.DB) partitionManager {
			return repository.NewPartitionManager(db.Pool)
		}
	}
	if factory.newCompensationStore == nil {
		factory.newCompensationStore = func(db *repository.DB) domain.CompensationStore {
			return domain.NewSQLCompensationStore(db.Pool)
		}
	}
	if factory.newCompensationExecutor == nil {
		factory.newCompensationExecutor = compensationNewDefaultExecutor
	}
	if factory.newCompensationProcessor == nil {
		factory.newCompensationProcessor = func(
			store domain.CompensationStore,
			executor domain.OperationExecutor,
			stats domain.CompensationStats,
		) compensationWorker {
			return domain.NewCompensationProcessor(store, executor, stats)
		}
	}
	return factory
}

var compensationNewDefaultExecutor = func() domain.OperationExecutor {
	return compensation.NewDefaultCompensationExecutor()
}

func startPartitionMaintenance(ctx context.Context, logger logging.Logger, manager partitionManager) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := manager.EnsureFuturePartitions(context.Background()); err != nil {
				infof(logger, "分区维护: 预创建未来分区失败: %v", err)
			}
			for _, tableName := range []string{"audit_events", "supply_usage_records", "supply_idempotency_records"} {
				if _, err := manager.DropOldPartitions(context.Background(), tableName); err != nil {
					infof(logger, "分区维护: 清理过期分区失败 (%s): %v", tableName, err)
				}
			}
		}
	}
}
