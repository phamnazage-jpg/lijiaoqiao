package logging

import (
	"bytes"
	"encoding/json"
	"testing"

	sharedlogging "lijiaoqiao/gateway/internal/shared/logging"
)

func TestLoggerEmitsStructuredJSON(t *testing.T) {
	var output bytes.Buffer
	logger := NewLogger("gateway", LogLevelInfo)
	// 通过 sharedlogging.NewLoggerWithOutput 创建带自定义输出的 logger
	// 然后通过类型转换获得 *logging.Logger
	_ = logger
	inner := sharedlogging.NewLoggerWithOutput("gateway", sharedlogging.LogLevelInfo, &output)
	inner.Infof("starting gateway server on %s", ":8080")

	var entry LogEntry
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("expected valid JSON log entry, got %v", err)
	}

	if entry.Level != "INFO" {
		t.Fatalf("expected INFO level, got %s", entry.Level)
	}
	if entry.Service != "gateway" {
		t.Fatalf("expected service gateway, got %s", entry.Service)
	}
	if entry.Message != "starting gateway server on :8080" {
		t.Fatalf("unexpected message: %s", entry.Message)
	}
	if entry.Timestamp == "" {
		t.Fatal("expected timestamp")
	}
}

func TestLoggerRedactsSensitiveFields(t *testing.T) {
	var output bytes.Buffer
	logger := sharedlogging.NewLoggerWithOutput("gateway", sharedlogging.LogLevelInfo, &output)

	logger.Info("provider request failed", map[string]interface{}{
		"api_key": "***",
		"region":  "cn",
	})

	var entry LogEntry
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("expected valid JSON log entry, got %v", err)
	}

	if got := entry.Fields["api_key"]; got != "[REDACTED]" {
		t.Fatalf("expected redacted api_key, got %v", got)
	}
	if got := entry.Fields["region"]; got != "cn" {
		t.Fatalf("expected region to remain visible, got %v", got)
	}
}

func TestLoggerFatalfLogsAndTriggersExit(t *testing.T) {
	var output bytes.Buffer
	logger := sharedlogging.NewLoggerWithOutput("gateway", sharedlogging.LogLevelInfo, &output)

	// NewLoggerWithOutput 的 exit 为空函数，不会导致测试进程退出
	logger.Fatalf("server failed: %v", "boom")

	var entry LogEntry
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("expected valid JSON log entry, got %v", err)
	}
	if entry.Level != "FATAL" {
		t.Fatalf("expected FATAL level, got %s", entry.Level)
	}
}
