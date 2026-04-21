package sharedlogging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// LogLevel 日志级别
type LogLevel string

const (
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
	LogLevelFatal LogLevel = "FATAL"
)

// LogEntry 统一 JSON 日志 schema — 三服务共享
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Service   string                 `json:"service"`
	TraceID   string                 `json:"trace_id,omitempty"`
	SpanID    string                 `json:"span_id,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// Logger 输出 JSON 结构化日志 — 三服务共享实现
type Logger struct {
	service  string
	minLevel LogLevel
	output   io.Writer
	exit     func(int)
}

// SensitiveFields 需要自动脱敏的字段关键字（三服务共享）
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
	"passport", // supply-api 额外字段
}

// NewLogger 创建统一 JSON logger
func NewLogger(service string, minLevel LogLevel) *Logger {
	return &Logger{
		service:  service,
		minLevel: minLevel,
		output:   os.Stdout,
		exit:     os.Exit,
	}
}

// NewLoggerWithOutput 创建带自定义输出的 logger（用于测试）
func NewLoggerWithOutput(service string, minLevel LogLevel, output io.Writer) *Logger {
	return &Logger{
		service:  service,
		minLevel: minLevel,
		output:   output,
		exit:     func(int) {},
	}
}

func (l *Logger) shouldLog(level LogLevel) bool {
	levels := map[LogLevel]int{
		LogLevelDebug: 0,
		LogLevelInfo:  1,
		LogLevelWarn:  2,
		LogLevelError: 3,
		LogLevelFatal: 4,
	}
	return levels[level] >= levels[l.minLevel]
}

func (l *Logger) log(level LogLevel, msg string, fields map[string]interface{}) {
	if !l.shouldLog(level) {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     string(level),
		Service:   l.service,
		Message:   msg,
	}
	if len(fields) > 0 {
		entry.Fields = sanitizeFields(fields)
	}

	encoder := json.NewEncoder(l.output)
	_ = encoder.Encode(entry)
}

func (l *Logger) Debug(msg string, fields ...map[string]interface{}) {
	l.log(LogLevelDebug, msg, firstFields(fields))
}

func (l *Logger) Info(msg string, fields ...map[string]interface{}) {
	l.log(LogLevelInfo, msg, firstFields(fields))
}

func (l *Logger) Warn(msg string, fields ...map[string]interface{}) {
	l.log(LogLevelWarn, msg, firstFields(fields))
}

func (l *Logger) Error(msg string, fields ...map[string]interface{}) {
	l.log(LogLevelError, msg, firstFields(fields))
}

// Fatal 记录日志后调用 os.Exit(1)
func (l *Logger) Fatal(msg string, fields ...map[string]interface{}) {
	l.log(LogLevelFatal, msg, firstFields(fields))
	if l.exit != nil {
		l.exit(1)
	}
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Debug(fmt.Sprintf(format, args...))
}

func (l *Logger) Infof(format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...))
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Warn(fmt.Sprintf(format, args...))
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...))
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.Fatal(fmt.Sprintf(format, args...))
}

func firstFields(fields []map[string]interface{}) map[string]interface{} {
	if len(fields) == 0 {
		return nil
	}
	return fields[0]
}

// sanitizeFields 敏感字段脱敏（三服务共享）
func sanitizeFields(fields map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{}, len(fields))
	for k, v := range fields {
		lowerKey := toLower(k)
		redacted := false
		for _, sensitive := range SensitiveFields {
			if contains(lowerKey, sensitive) {
				sanitized[k] = "[REDACTED]"
				redacted = true
				break
			}
		}
		if redacted {
			continue
		}
		if nestedMap, ok := v.(map[string]interface{}); ok {
			sanitized[k] = sanitizeFields(nestedMap)
			continue
		}
		sanitized[k] = v
	}
	return sanitized
}

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
	if len(substr) == 0 || len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
