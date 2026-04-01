package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"lijiaoqiao/supply-api/internal/config"
)

// DB 数据库连接池
type DB struct {
	Pool *pgxpool.Pool
}

// NewDB 创建数据库连接池
func NewDB(ctx context.Context, cfg config.DatabaseConfig) (*DB, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.ConnMaxLifetime
	poolConfig.MaxConnIdleTime = cfg.ConnMaxIdleTime
	poolConfig.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// 验证连接
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Close 关闭连接池
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

// HealthCheck 健康检查
func (db *DB) HealthCheck(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

// BeginTx 开始事务
func (db *DB) BeginTx(ctx context.Context) (Transaction, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &txWrapper{tx: tx}, nil
}

// Transaction 事务接口
type Transaction interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type txWrapper struct {
	tx pgxpool.Tx
}

func (t *txWrapper) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t *txWrapper) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}
