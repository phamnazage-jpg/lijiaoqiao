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

type runtimeBackgroundView struct {
	env                  string
	logger               logging.Logger
	tuning               runtimeTuning
	db                   *repository.DB
	redisCache           *cache.RedisCache
	revocationSubscriber revocationSubscriber
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
	view, err := buildRuntimeBackgroundView(runtime)
	if err != nil {
		return err
	}
	return startBackgroundWorkersWithViewAndFactory(rootCtx, initCtx, view, factory)
}

func buildRuntimeBackgroundView(runtime *Runtime) (runtimeBackgroundView, error) {
	if runtime == nil {
		return runtimeBackgroundView{}, errors.New("runtime is required")
	}
	if runtime.logger == nil {
		return runtimeBackgroundView{}, errors.New("runtime logger is required")
	}

	view := runtimeBackgroundView{
		env:                  runtime.env,
		logger:               runtime.logger,
		tuning:               runtime.tuning,
		db:                   runtime.db,
		redisCache:           runtime.redisCache,
		revocationSubscriber: runtime.revocationSubscriber,
	}
	if view.tuning.outboxStreamName == "" {
		view.tuning = defaultRuntimeTuning()
	}
	return view, nil
}

func startBackgroundWorkersWithViewAndFactory(
	rootCtx context.Context,
	initCtx context.Context,
	view runtimeBackgroundView,
	factory backgroundFactory,
) error {
	if rootCtx == nil {
		rootCtx = context.Background()
	}
	if initCtx == nil {
		initCtx = rootCtx
	}

	factory = withDefaultBackgroundFactory(factory, view.tuning)

	startRevocationSubscriber(rootCtx, view)

	if view.db == nil {
		return nil
	}

	if err := startOutboxProcessor(rootCtx, view, factory); err != nil {
		return err
	}
	startPartitionMaintenanceWorker(rootCtx, initCtx, view, factory)
	startCompensationWorker(rootCtx, view, factory)

	return nil
}

func withDefaultBackgroundFactory(factory backgroundFactory, tuning runtimeTuning) backgroundFactory {
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
			return messaging.NewOutboxMessageBroker(redisCache.GetClient(), tuning.outboxStreamName, tuning.outboxConsumerGroup)
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

func startRevocationSubscriber(ctx context.Context, view runtimeBackgroundView) {
	if view.revocationSubscriber == nil || view.redisCache == nil {
		return
	}
	if err := view.revocationSubscriber.StartRevocationSubscriber(ctx); err != nil {
		warnf(view.logger, "启动主动吊销订阅失败: %v", err)
		return
	}
	view.logger.Info("主动吊销机制: 已启动 (Redis Pub/Sub)", nil)
}

func startOutboxProcessor(ctx context.Context, view runtimeBackgroundView, factory backgroundFactory) error {
	if view.db == nil {
		return nil
	}

	factory = withDefaultBackgroundFactory(factory, view.tuning)

	outboxRepo := factory.newOutboxRepository(view.db)
	msgBroker := factory.newMessageBroker(view.redisCache)
	if msgBroker == nil {
		if view.env == "prod" {
			return errors.New("outbox message broker unavailable")
		}
		view.logger.Warn("OutboxProcessor未启动 (message broker不可用)", nil)
		return nil
	}

	stats := &messaging.NoOpOutboxStats{}
	runner := factory.newOutboxRunner(outboxRepo, msgBroker, stats)
	go runner.Start(ctx)
	view.logger.Info("OutboxProcessor已启动", nil)
	return nil
}

func startPartitionMaintenanceWorker(
	rootCtx context.Context,
	initCtx context.Context,
	view runtimeBackgroundView,
	factory backgroundFactory,
) {
	if view.db == nil {
		return
	}

	factory = withDefaultBackgroundFactory(factory, view.tuning)
	manager := factory.newPartitionManager(view.db)
	if err := manager.EnsureFuturePartitions(initCtx); err != nil {
		warnf(view.logger, "预创建未来分区失败: %v", err)
	} else {
		view.logger.Info("分区管理: 未来分区已确保存在", nil)
	}

	go runPartitionMaintenanceLoop(rootCtx, view.logger, manager, view.tuning)
}

func startCompensationWorker(ctx context.Context, view runtimeBackgroundView, factory backgroundFactory) {
	if view.db == nil {
		return
	}

	factory = withDefaultBackgroundFactory(factory, view.tuning)
	compensationStore := factory.newCompensationStore(view.db)
	compensationStats := &domain.NoOpCompensationStats{}
	compensationExecutor := factory.newCompensationExecutor()
	compensationProcessor := factory.newCompensationProcessor(compensationStore, compensationExecutor, compensationStats)

	view.logger.Info("批量补偿处理器: 已初始化", nil)
	compensationProcessor.StartBackgroundWorker(ctx, view.tuning.compensationCheckInterval)
	infof(view.logger, "批量补偿处理器: 后台worker已启动 (每%s检查一次)", view.tuning.compensationCheckInterval)
}

func runPartitionMaintenanceLoop(ctx context.Context, logger logging.Logger, manager partitionManager, tuning runtimeTuning) {
	ticker := time.NewTicker(tuning.partitionMaintenanceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := manager.EnsureFuturePartitions(context.Background()); err != nil {
				warnf(logger, "分区维护: 预创建未来分区失败: %v", err)
			}
			for _, tableName := range tuning.partitionedTables {
				if _, err := manager.DropOldPartitions(context.Background(), tableName); err != nil {
					warnf(logger, "分区维护: 清理过期分区失败 (%s): %v", tableName, err)
				}
			}
		}
	}
}
