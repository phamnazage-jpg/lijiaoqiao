// Package logging — pkg/logging 兼容适配层
//
// 将原有实现迁移至 shared/logging，本包保留以免破坏现有导入。
// 所有类型和函数均为 shared/logging 的重新导出。
package logging

import (
	sharedlogging "lijiaoqiao/gateway/internal/shared/logging"
)

// 日志级别 — 从 shared/logging 重新导出
type LogLevel = sharedlogging.LogLevel

const (
	LogLevelDebug = sharedlogging.LogLevelDebug
	LogLevelInfo  = sharedlogging.LogLevelInfo
	LogLevelWarn  = sharedlogging.LogLevelWarn
	LogLevelError = sharedlogging.LogLevelError
	LogLevelFatal = sharedlogging.LogLevelFatal
)

// LogEntry — 从 shared/logging 重新导出
type LogEntry = sharedlogging.LogEntry

// Logger — 从 shared/logging 重新导出
type Logger = sharedlogging.Logger

// SensitiveFields — 从 shared/logging 重新导出
var SensitiveFields = sharedlogging.SensitiveFields

// NewLogger 创建统一 JSON logger — 转发至 shared/logging
func NewLogger(service string, minLevel LogLevel) *Logger {
	return sharedlogging.NewLogger(service, minLevel)
}
