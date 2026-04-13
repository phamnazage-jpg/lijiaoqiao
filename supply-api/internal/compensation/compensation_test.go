package compensation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestDefaultCompensationExecutor_DispatchesByOperationType(t *testing.T) {
	executor := NewDefaultCompensationExecutor()
	payload := json.RawMessage(`{"account":"acc-1"}`)
	called := ""

	executor.handlers = map[string]operationHandler{
		"account.create": func(ctx context.Context, body json.RawMessage) error {
			called = string(body)
			return nil
		},
	}

	if err := executor.Execute(context.Background(), "account.create", payload); err != nil {
		t.Fatalf("expected dispatch to succeed, got %v", err)
	}
	if called != string(payload) {
		t.Fatalf("expected handler to receive %s, got %s", payload, called)
	}
}

func TestDefaultCompensationExecutor_RejectsUnknownOperationType(t *testing.T) {
	executor := NewDefaultCompensationExecutor()

	err := executor.Execute(context.Background(), "unknown.operation", json.RawMessage(`{"test":true}`))
	if err == nil {
		t.Fatal("expected unknown operation type to return error")
	}
	if !strings.Contains(err.Error(), "unknown operation type") {
		t.Fatalf("expected unknown operation type error, got %v", err)
	}
}

func TestNewDefaultCompensationExecutor_RegistersBuiltInHandlers(t *testing.T) {
	executor := NewDefaultCompensationExecutor()

	for _, operationType := range []string{
		"account.create",
		"package.publish",
		"settlement.withdraw",
		"quota.deduct",
	} {
		if _, ok := executor.handlers[operationType]; !ok {
			t.Fatalf("expected built-in handler for %s", operationType)
		}
	}
}
