package logging

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestLoggerEmitsStructuredJSON(t *testing.T) {
	var output bytes.Buffer
	logger := NewLogger("platform-token-runtime", LogLevelInfo)
	logger.output = &output

	logger.Infof("platform-token-runtime listening on %s", ":18081")

	var entry LogEntry
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("expected valid JSON log entry, got %v", err)
	}

	if entry.Level != "INFO" {
		t.Fatalf("expected INFO level, got %s", entry.Level)
	}
	if entry.Service != "platform-token-runtime" {
		t.Fatalf("expected service platform-token-runtime, got %s", entry.Service)
	}
	if entry.Message != "platform-token-runtime listening on :18081" {
		t.Fatalf("unexpected message: %s", entry.Message)
	}
	if entry.Timestamp == "" {
		t.Fatal("expected timestamp")
	}
}

func TestLoggerRedactsSensitiveFields(t *testing.T) {
	var output bytes.Buffer
	logger := NewLogger("platform-token-runtime", LogLevelInfo)
	logger.output = &output

	logger.Info("token runtime request failed", map[string]interface{}{
		"token": "secret-value",
		"env":   "prod",
	})

	var entry LogEntry
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("expected valid JSON log entry, got %v", err)
	}

	if got := entry.Fields["token"]; got != "[REDACTED]" {
		t.Fatalf("expected redacted token, got %v", got)
	}
	if got := entry.Fields["env"]; got != "prod" {
		t.Fatalf("expected env to remain visible, got %v", got)
	}
}

func TestLoggerFatalfLogsAndTriggersExit(t *testing.T) {
	var output bytes.Buffer
	exitCode := 0

	logger := NewLogger("platform-token-runtime", LogLevelInfo)
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
