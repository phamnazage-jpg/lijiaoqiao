package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"lijiaoqiao/supply-api/internal/config"
)

// DB 数据库连接池
type DB struct {
	Pool *pgxpool.Pool
}

// NewDB 创建数据库连接池
func NewDB(ctx context.Context, cfg config.DatabaseConfig) (*DB, error) {
	dsn := cfg.DSN()
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		// P2-05: 使用SafeDSN替代DSN，避免在错误信息中泄露密码
		return nil, fmt.Errorf("failed to parse database config for %s: %v", cfg.SafeDSN(), sanitizeErrorPassword(err, cfg.Password))
	}

	poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.ConnMaxLifetime
	poolConfig.MaxConnIdleTime = cfg.ConnMaxIdleTime
	poolConfig.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		// P2-05: 清理错误信息中的密码
		return nil, fmt.Errorf("failed to create connection pool for %s: %v", cfg.SafeDSN(), sanitizeErrorPassword(err, cfg.Password))
	}

	// 验证连接
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database at %s:%d: %v", cfg.Host, cfg.Port, err)
	}

	return &DB{Pool: pool}, nil
}

// sanitizeErrorPassword 从错误信息中清理密码
// P2-05: pgxpool.ParseConfig的错误信息可能包含完整的DSN，需要清理
func sanitizeErrorPassword(err error, password string) error {
	if err == nil || password == "" {
		return err
	}
	// 将错误信息中的密码替换为***
	errStr := err.Error()
	safeErrStr := strings.ReplaceAll(errStr, password, "***")
	if safeErrStr != errStr {
		return fmt.Errorf("%s (password sanitized)", safeErrStr)
	}
	return err
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
	tx pgx.Tx
}

func (t *txWrapper) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t *txWrapper) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}
