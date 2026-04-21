package sharedlogging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestLogLevels_GoldenOutput 验证各日志级别的输出格式符合规范
func TestLogLevels_GoldenOutput(t *testing.T) {
	testCases := []struct {
		level       string
		logFunc     func(*Logger, string)
		expectLevel string
	}{
		{"INFO", func(l *Logger, msg string) { l.Info(msg) }, "INFO"},
		{"WARN", func(l *Logger, msg string) { l.Warn(msg) }, "WARN"},
		{"ERROR", func(l *Logger, msg string) { l.Error(msg) }, "ERROR"},
	}

	for _, tc := range testCases {
		t.Run(tc.level, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLoggerWithOutput("test-service", LogLevelDebug, &buf)

			tc.logFunc(logger, "test message")

			output := strings.TrimSpace(buf.String())
			if output == "" {
				t.Fatalf("empty output for level %s", tc.level)
			}

			// 解析 JSON 验证格式
			var entry LogEntry
			if err := json.Unmarshal([]byte(output), &entry); err != nil {
				t.Fatalf("invalid JSON output: %v\noutput: %s", err, output)
			}

			// Golden assertions: 固定字段
			if entry.Level != tc.expectLevel {
				t.Errorf("level = %q, want %q", entry.Level, tc.expectLevel)
			}
			if entry.Service != "test-service" {
				t.Errorf("service = %q, want %q", entry.Service, "test-service")
			}
			if entry.Message != "test message" {
				t.Errorf("message = %q, want %q", entry.Message, "test message")
			}
			if entry.Timestamp == "" {
				t.Error("timestamp should not be empty")
			}
		})
	}
}

// TestLogLevels_WithFields 验证带 fields 的日志输出
func TestLogLevels_WithFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithOutput("test-service", LogLevelDebug, &buf)

	logger.Info("request completed", map[string]interface{}{
		"request_id": "req-123",
		"status":     200,
		"duration":   45.6,
	})

	output := strings.TrimSpace(buf.String())
	var entry LogEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if entry.Level != "INFO" {
		t.Errorf("level = %q, want INFO", entry.Level)
	}
	if entry.Fields == nil {
		t.Fatal("fields should not be nil")
	}
	if entry.Fields["request_id"] != "req-123" {
		t.Errorf("fields[request_id] = %v, want req-123", entry.Fields["request_id"])
	}
	if entry.Fields["status"].(float64) != 200 {
		t.Errorf("fields[status] = %v, want 200", entry.Fields["status"])
	}
}

// TestSensitiveFields_GoldenOutput 验证敏感字段被正确脱敏
func TestSensitiveFields_GoldenOutput(t *testing.T) {
	sensitive := []struct {
		field string
		value interface{}
	}{
		{"password", "supersecret"},
		{"api_key", "sk-live-xxxxx"},
		{"authorization", "Bearer eyJ..."},
		{"token", "tok_xxxxx"},
		{"credit_card", "4111111111111111"},
	}

	for _, tc := range sensitive {
		t.Run(tc.field, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLoggerWithOutput("test-service", LogLevelDebug, &buf)

			logger.Info("auth attempt", map[string]interface{}{
				tc.field: tc.value,
				"user":   "alice",
			})

			output := strings.TrimSpace(buf.String())
			var entry LogEntry
			if err := json.Unmarshal([]byte(output), &entry); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}

			if entry.Fields[tc.field] != "[REDACTED]" {
				t.Errorf("field %q = %v, want [REDACTED]", tc.field, entry.Fields[tc.field])
			}
			if entry.Fields["user"] != "alice" {
				t.Errorf("field user = %v, want alice", entry.Fields["user"])
			}
		})
	}
}

// TestLogLevelFiltering 验证日志级别过滤
func TestLogLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithOutput("test-service", LogLevelError, &buf)

	logger.Info("should not appear")
	logger.Warn("should not appear either")
	logger.Error("should appear")

	output := strings.TrimSpace(buf.String())
	if output == "" {
		t.Fatal("expected output for ERROR level")
	}

	var entry LogEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry.Level != "ERROR" {
		t.Errorf("level = %q, want ERROR", entry.Level)
	}
	if entry.Message != "should appear" {
		t.Errorf("message = %q, want 'should appear'", entry.Message)
	}
}

// TestFatal_GoldenOutput 验证 FATAL 日志格式正确
func TestFatal_GoldenOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithOutput("test-service", LogLevelDebug, &buf)
	var exited int
	logger.exit = func(code int) { exited = code }

	logger.Fatal("service crashed", map[string]interface{}{"error": "OOM"})

	output := strings.TrimSpace(buf.String())
	var entry LogEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if entry.Level != "FATAL" {
		t.Errorf("level = %q, want FATAL", entry.Level)
	}
	if entry.Message != "service crashed" {
		t.Errorf("message = %q, want 'service crashed'", entry.Message)
	}
	if entry.Fields["error"] != "OOM" {
		t.Errorf("fields[error] = %v, want OOM", entry.Fields["error"])
	}
	if exited != 1 {
		t.Errorf("exit code = %d, want 1", exited)
	}
}

// TestNestedFields_GoldenOutput 验证嵌套字段脱敏
func TestNestedFields_GoldenOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithOutput("test-service", LogLevelDebug, &buf)

	logger.Info("nested test", map[string]interface{}{
		"user": map[string]interface{}{
			"name":     "alice",
			"password": "secret123",
		},
	})

	output := strings.TrimSpace(buf.String())
	var entry LogEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	nested, ok := entry.Fields["user"].(map[string]interface{})
	if !ok {
		t.Fatal("user field not a nested map")
	}
	if nested["password"] != "[REDACTED]" {
		t.Errorf("nested password = %v, want [REDACTED]", nested["password"])
	}
	if nested["name"] != "alice" {
		t.Errorf("nested name = %v, want alice", nested["name"])
	}
}

// TestFormatMethods 验证格式化方法（Infof/Errorf 等）
func TestFormatMethods(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithOutput("test-service", LogLevelDebug, &buf)

	logger.Infof("user %s logged in at %d", "alice", 1609459200)
	logger.Errorf("request to %s failed: %v", "/api/v1/test", "timeout")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	var infoEntry, errorEntry LogEntry
	if err := json.Unmarshal([]byte(lines[0]), &infoEntry); err != nil {
		t.Fatalf("invalid INFO JSON: %v", err)
	}
	if infoEntry.Level != "INFO" {
		t.Errorf("info level = %q, want INFO", infoEntry.Level)
	}
	if infoEntry.Message != "user alice logged in at 1609459200" {
		t.Errorf("info message = %q, want 'user alice logged in at 1609459200'", infoEntry.Message)
	}

	if err := json.Unmarshal([]byte(lines[1]), &errorEntry); err != nil {
		t.Fatalf("invalid ERROR JSON: %v", err)
	}
	if errorEntry.Level != "ERROR" {
		t.Errorf("error level = %q, want ERROR", errorEntry.Level)
	}
}
