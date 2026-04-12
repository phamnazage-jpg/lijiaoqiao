package logging

import (
	"context"
	"log/slog"
	"os"
)

// SlogLogger 基于 Go 1.21+ slog 的结构化日志实现
type SlogLogger struct {
	*slog.Logger
}

// NewSlogLogger 创建 slog-based 日志实例
func NewSlogLogger(service string, minLevel LogLevel) *SlogLogger {
	opts := &slog.HandlerOptions{
		Level: levelToSlog(minLevel),
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)
	logger = logger.With("service", service)

	return &SlogLogger{Logger: logger}
}

// levelToSlog 转换日志级别
func levelToSlog(l LogLevel) slog.Level {
	switch l {
	case LogLevelDebug:
		return slog.LevelDebug
	case LogLevelInfo:
		return slog.LevelInfo
	case LogLevelWarn:
		return slog.LevelWarn
	case LogLevelError:
		return slog.LevelError
	case LogLevelFatal:
		return slog.LevelError // slog 没有 fatal，用 error 代替
	default:
		return slog.LevelInfo
	}
}

// WithTraceID 返回带 trace_id 的日志实例
func (l *SlogLogger) WithTraceID(ctx context.Context) *SlogLogger {
	traceID := GetTraceIDFromContext(ctx)
	if traceID == "" {
		return l
	}
	return &SlogLogger{Logger: l.Logger.With("trace_id", traceID)}
}

// WithRequestID 返回带 request_id 的日志实例
func (l *SlogLogger) WithRequestID(requestID string) *SlogLogger {
	if requestID == "" {
		return l
	}
	return &SlogLogger{Logger: l.Logger.With("request_id", requestID)}
}

// WithTenantID 返回带 tenant_id 的日志实例
func (l *SlogLogger) WithTenantID(tenantID int64) *SlogLogger {
	return &SlogLogger{Logger: l.Logger.With("tenant_id", tenantID)}
}

// GetTraceIDFromContext 从 context 提取 trace_id
func GetTraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	// 尝试从 W3C traceparent 提取
	if traceparent, ok := ctx.Value("traceparent").(string); ok && traceparent != "" {
		// 格式: 00-{trace-id}-{span-id}-{trace-flags}
		return extractTraceID(traceparent)
	}
	return ""
}

// extractTraceID 从 traceparent 提取 trace_id
func extractTraceID(traceparent string) string {
	// 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
	if len(traceparent) < 55 {
		return ""
	}
	return traceparent[3:35] // 0af7651916cd43dd8448eb211c80319c
}

// Fields 便捷方法：使用 map 创建带字段的日志
func (l *SlogLogger) Fields(fields map[string]interface{}) *SlogLogger {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return &SlogLogger{Logger: l.Logger.With(args...)}
}

// DebugInfo 记录调试信息
func (l *SlogLogger) DebugInfo(msg string) {
	l.Debug(msg)
}
