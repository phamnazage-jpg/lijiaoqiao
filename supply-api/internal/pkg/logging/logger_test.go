package logging

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// captureLogger 捕获日志输出的测试Logger
type captureLogger struct {
	*jsonLogger
	outputBuffer *strings.Builder
}

func newCaptureLogger() *captureLogger {
	buf := &strings.Builder{}
	return &captureLogger{
		jsonLogger: &jsonLogger{
			service:   "test-service",
			minLevel:  LogLevelDebug,
			output:   os.Stdout, // 实际输出到stdout但我们可以捕获
		},
		outputBuffer: buf,
	}
}

// 重写Info方法以捕获输出
func (l *captureLogger) Info(msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LogLevelInfo, msg, f)
}

// 重写Debug方法以捕获输出
func (l *captureLogger) Debug(msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LogLevelDebug, msg, f)
}

// 重写Warn方法以捕获输出
func (l *captureLogger) Warn(msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LogLevelWarn, msg, f)
}

// 重写Error方法以捕获输出
func (l *captureLogger) Error(msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LogLevelError, msg, f)
}

// 重写Fatal方法以捕获输出
func (l *captureLogger) Fatal(msg string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LogLevelFatal, msg, f)
}

// log 方法实际写入 outputBuffer
func (l *captureLogger) log(level LogLevel, msg string, fields map[string]interface{}) {
	if !l.shouldLog(level) {
		return
	}

	entry := l.formatEntry(level, msg, fields)

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	l.outputBuffer.Write(data)
	l.outputBuffer.WriteString("\n")
}

// TestP110_LogLevels 日志级别
func TestP110_LogLevels(t *testing.T) {
	logger := &jsonLogger{
		service:   "test",
		minLevel:  LogLevelInfo,
		output:   os.Stdout,
	}

	// Info及以上应该记录
	if !logger.shouldLog(LogLevelInfo) {
		t.Error("Info should be logged")
	}
	if !logger.shouldLog(LogLevelWarn) {
		t.Error("Warn should be logged")
	}
	if !logger.shouldLog(LogLevelError) {
		t.Error("Error should be logged")
	}

	// Debug不应该记录
	if logger.shouldLog(LogLevelDebug) {
		t.Error("Debug should not be logged when minLevel is Info")
	}

	t.Log("P1-10: 日志级别验证通过")
}

// TestP110_JSONFormat JSON格式验证
func TestP110_JSONFormat(t *testing.T) {
	logger := newCaptureLogger()
	logger.Info("test message", map[string]interface{}{
		FieldKeyTenantID: 123,
		FieldKeyRequestID: "req-123",
	})

	output := logger.outputBuffer.String()

	// 验证是有效JSON
	var entry LogEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	// 验证字段
	if entry.Level != "INFO" {
		t.Errorf("expected level INFO, got %s", entry.Level)
	}
	if entry.Service != "test-service" {
		t.Errorf("expected service test-service, got %s", entry.Service)
	}
	if entry.Message != "test message" {
		t.Errorf("expected message 'test message', got %s", entry.Message)
	}

	t.Log("P1-10: JSON格式验证通过")
}

// TestP110_TimestampFormat 时间戳格式
func TestP110_TimestampFormat(t *testing.T) {
	logger := newCaptureLogger()
	logger.Info("test")

	var entry LogEntry
	json.Unmarshal([]byte(logger.outputBuffer.String()), &entry)

	// 验证是RFC3339格式
	if !strings.Contains(entry.Timestamp, "T") {
		t.Error("timestamp should be in RFC3339 format")
	}

	t.Log("P1-10: 时间戳格式验证通过")
}

// TestP110_SensitiveFieldRedaction 敏感字段脱敏
func TestP110_SensitiveFieldRedaction(t *testing.T) {
	logger := newCaptureLogger()
	logger.Info("test", map[string]interface{}{
		"password":     "secret123",
		"api_key":      "sk-abc123",
		"user_name":    "john", // 非敏感字段
		"access_token": "tok-xyz",
	})

	var entry LogEntry
	json.Unmarshal([]byte(logger.outputBuffer.String()), &entry)

	fields := entry.Fields

	// 验证敏感字段被脱敏
	if fields["password"] != "[REDACTED]" {
		t.Errorf("password should be redacted, got %v", fields["password"])
	}
	if fields["api_key"] != "[REDACTED]" {
		t.Errorf("api_key should be redacted, got %v", fields["api_key"])
	}
	if fields["access_token"] != "[REDACTED]" {
		t.Errorf("access_token should be redacted, got %v", fields["access_token"])
	}

	// 非敏感字段不应被脱敏
	if fields["user_name"] != "john" {
		t.Errorf("user_name should not be redacted, got %v", fields["user_name"])
	}

	t.Log("P1-10: 敏感字段脱敏验证通过")
}

// TestP110_NestedSensitiveFields 嵌套敏感字段
func TestP110_NestedSensitiveFields(t *testing.T) {
	logger := newCaptureLogger()
	logger.Info("test", map[string]interface{}{
		"user": map[string]interface{}{
			"name":     "john",
			"password": "secret",
		},
	})

	var entry LogEntry
	json.Unmarshal([]byte(logger.outputBuffer.String()), &entry)

	fields := entry.Fields
	user := fields["user"].(map[string]interface{})

	if user["password"] != "[REDACTED]" {
		t.Errorf("nested password should be redacted, got %v", user["password"])
	}
	if user["name"] != "john" {
		t.Errorf("nested name should not be redacted, got %v", user["name"])
	}

	t.Log("P1-10: 嵌套敏感字段验证通过")
}

// TestP110_LogFieldsConstants 日志字段常量
func TestP110_LogFieldsConstants(t *testing.T) {
	// 验证字段常量定义正确
	if FieldKeyTenantID != "tenant_id" {
		t.Errorf("FieldKeyTenantID should be tenant_id")
	}
	if FieldKeyUserID != "user_id" {
		t.Errorf("FieldKeyUserID should be user_id")
	}
	if FieldKeyRequestID != "request_id" {
		t.Errorf("FieldKeyRequestID should be request_id")
	}
	if FieldKeyTraceID != "trace_id" {
		t.Errorf("FieldKeyTraceID should be trace_id")
	}
	if FieldKeyDuration != "duration_ms" {
		t.Errorf("FieldKeyDuration should be duration_ms")
	}

	t.Log("P1-10: 日志字段常量验证通过")
}

// TestP110_SensitiveFieldsList 敏感字段列表
func TestP110_SensitiveFieldsList(t *testing.T) {
	expected := []string{
		"password", "secret", "token", "api_key", "apikey",
		"credential", "authorization", "private_key",
		"credit_card", "ssn",
	}

	for _, exp := range expected {
		found := false
		for _, sens := range SensitiveFields {
			if sens == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected sensitive field %s not found", exp)
		}
	}

	t.Log("P1-10: 敏感字段列表验证通过")
}

// TestP110_Summary 测试总结
func TestP110_Summary(t *testing.T) {
	t.Log("=== P1-010 日志规范测试总结 ===")
	t.Log("问题: 所有文档均未定义日志级别、格式、结构化日志规范")
	t.Log("")
	t.Log("修复方案:")
	t.Log("  - JSON结构化日志")
	t.Log("  - 字段: timestamp, level, service, trace_id, request_id, message, fields")
	t.Log("  - 级别: DEBUG, INFO, WARN, ERROR, FATAL")
	t.Log("  - 敏感字段自动脱敏")
	t.Log("  - 时间戳: RFC3339Nano格式")
	t.Log("")
	t.Log("JSON示例:")
	t.Log(`{"timestamp":"2026-04-07T10:30:00.123Z","level":"INFO","service":"supply-api","request_id":"req-123","message":"request completed","fields":{"duration_ms":50,"status_code":200}}`)
}
