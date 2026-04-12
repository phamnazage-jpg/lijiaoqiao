package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ==================== P1-010 日志规范 ====================

// LogLevel 日志级别
type LogLevel string

const (
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
	LogLevelFatal LogLevel = "FATAL"
)

// LogEntry 标准日志条目（JSON格式）
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`           // ISO8601格式
	Level    string                 `json:"level"`               // DEBUG|INFO|WARN|ERROR|FATAL
	Service  string                 `json:"service"`            // 服务名称
	TraceID  string                 `json:"trace_id,omitempty"` // 追踪ID
	SpanID   string                 `json:"span_id,omitempty"`   // Span ID
	RequestID string                `json:"request_id,omitempty"` // 请求ID
	Message  string                 `json:"message"`             // 日志消息
	Fields   map[string]interface{} `json:"fields,omitempty"`    // 额外字段
}

// Logger 日志接口
type Logger interface {
	Debug(msg string, fields ...map[string]interface{})
	Info(msg string, fields ...map[string]interface{})
	Warn(msg string, fields ...map[string]interface{})
	Error(msg string, fields ...map[string]interface{})
	Fatal(msg string, fields ...map[string]interface{})
}

// jsonLogger JSON格式日志实现
type jsonLogger struct {
	service   string
	minLevel LogLevel
	output   *os.File
}

// NewLogger 创建日志实例
func NewLogger(service string, minLevel LogLevel) *jsonLogger {
	return &jsonLogger{
		service:   service,
		minLevel: minLevel,
		output:   os.Stdout,
	}
}

// shouldLog 检查是否应该记录此级别
func (l *jsonLogger) shouldLog(level LogLevel) bool {
	levels := map[LogLevel]int{
		LogLevelDebug: 0,
		LogLevelInfo:  1,
		LogLevelWarn:  2,
		LogLevelError: 3,
		LogLevelFatal: 4,
	}

	return levels[level] >= levels[l.minLevel]
}

// formatEntry 格式化日志条目
func (l *jsonLogger) formatEntry(level LogLevel, msg string, fields map[string]interface{}) *LogEntry {
	entry := &LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     string(level),
		Service:   l.service,
		Message:   msg,
	}

	// 添加fields
	if fields != nil {
		entry.Fields = sanitizeFields(fields)
	}

	return entry
}

// log 输出日志
func (l *jsonLogger) log(level LogLevel, msg string, fields map[string]interface{}) {
	if !l.shouldLog(level) {
		return
	}

	entry := l.formatEntry(level, msg, fields)

	// 序列化为JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	// 添加换行符
	l.output.Write(append(data, '\n'))
}

func (l *jsonLogger) Debug(msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LogLevelDebug, msg, f)
}

func (l *jsonLogger) Info(msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LogLevelInfo, msg, f)
}

func (l *jsonLogger) Warn(msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LogLevelWarn, msg, f)
}

func (l *jsonLogger) Error(msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LogLevelError, msg, f)
}

func (l *jsonLogger) Fatal(msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LogLevelFatal, msg, f)
}

// Infof 格式化信息日志
func (l *jsonLogger) Infof(format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...), nil)
}

// Errorf 格式化错误日志
func (l *jsonLogger) Errorf(format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...), nil)
}

// Warnf 格式化警告日志
func (l *jsonLogger) Warnf(format string, args ...interface{}) {
	l.Warn(fmt.Sprintf(format, args...), nil)
}

// Debugf 格式化调试日志
func (l *jsonLogger) Debugf(format string, args ...interface{}) {
	l.Debug(fmt.Sprintf(format, args...), nil)
}

// Fatalf 格式化致命日志
func (l *jsonLogger) Fatalf(format string, args ...interface{}) {
	l.Fatal(fmt.Sprintf(format, args...), nil)
	os.Exit(1)
}

// sanitizeFields 敏感字段脱敏
func sanitizeFields(fields map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})

	sensitiveKeys := []string{
		"password", "secret", "token", "api_key", "apikey",
		"credential", "authorization", "private_key",
		"credit_card", "ssn", "passport",
	}

	for k, v := range fields {
		lowerK := toLower(k)
		for _, sensitive := range sensitiveKeys {
			if contains(lowerK, sensitive) {
				sanitized[k] = "[REDACTED]"
				break
			}
		}

		if _, ok := sanitized[k]; !ok {
			// 检查嵌套map
			if nestedMap, ok := v.(map[string]interface{}); ok {
				sanitized[k] = sanitizeFields(nestedMap)
			} else {
				sanitized[k] = v
			}
		}
	}

	return sanitized
}

// String helpers
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// LogFieldKeys 日志字段名常量（防止拼写错误）
const (
	FieldKeyTenantID     = "tenant_id"
	FieldKeyUserID      = "user_id"
	FieldKeyRequestID   = "request_id"
	FieldKeyTraceID     = "trace_id"
	FieldKeySpanID      = "span_id"
	FieldKeyOperation   = "operation"
	FieldKeyDuration    = "duration_ms"
	FieldKeyStatusCode   = "status_code"
	FieldKeyError        = "error"
	FieldKeyErrorCode   = "error_code"
	FieldKeyClientIP     = "client_ip"
	FieldKeyUserAgent   = "user_agent"
	FieldKeyMethod       = "method"
	FieldKeyPath         = "path"
	FieldKeyQuery        = "query"
	FieldKeyRoute        = "route"
)

// 标准字段常量
var SensitiveFields = []string{
	"password",
	"secret",
	"token",
	"api_key",
	"apikey",
	"credential",
	"authorization",
	"private_key",
	"credit_card",
	"ssn",
}
