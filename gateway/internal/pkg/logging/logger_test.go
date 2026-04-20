package logging

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestLoggerEmitsStructuredJSON(t *testing.T) {
	var output bytes.Buffer
	logger := NewLogger("gateway", LogLevelInfo)
	logger.output = &output

	logger.Infof("starting gateway server on %s", ":8080")

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
	logger := NewLogger("gateway", LogLevelInfo)
	logger.output = &output

	logger.Info("provider request failed", map[string]interface{}{
		"api_key": "secret-value",
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
	exitCode := 0

	logger := NewLogger("gateway", LogLevelInfo)
	logger.output = &output
	logger.exit = func(code int) {
		exitCode = code
	}

	logger.Fatalf("server failed: %v", "boom")

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}

	var entry LogEntry
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("expected valid JSON log entry, got %v", err)
	}
	if entry.Level != "FATAL" {
		t.Fatalf("expected FATAL level, got %s", entry.Level)
	}
}
