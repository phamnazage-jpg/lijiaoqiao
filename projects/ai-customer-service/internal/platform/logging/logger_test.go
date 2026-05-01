package logging

import (
	"log/slog"
	"testing"
)

func TestNew_ReturnsNonNil(t *testing.T) {
	logger := New()
	if logger == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNew_ReturnsSlogLogger(t *testing.T) {
	logger := New()
	if logger == nil {
		t.Fatal("logger is nil")
	}
	// Verify it's a *slog.Logger by using it
	var _ *slog.Logger = logger
}

func TestNew_InfoLevel(t *testing.T) {
	logger := New()
	logger.Info("test info message")
}

func TestNew_WithAttr(t *testing.T) {
	logger := New()
	logger.Info("test with attrs", slog.String("key", "value"))
}

func TestNew_Error(t *testing.T) {
	logger := New()
	logger.Error("test error message")
}

func TestNew_Debug(t *testing.T) {
	logger := New()
	logger.Debug("test debug message")
}
