package repository

import "errors"

// 仓储层错误定义
var (
	// ErrNotFound 资源不存在
	ErrNotFound = errors.New("resource not found")

	// ErrConcurrencyConflict 并发冲突（乐观锁失败）
	ErrConcurrencyConflict = errors.New("concurrency conflict: resource was modified by another transaction")
)
