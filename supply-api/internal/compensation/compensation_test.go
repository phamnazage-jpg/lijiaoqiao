package compensation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestDefaultCompensationExecutor_DispatchesByOperationType(t *testing.T) {
	executor := NewDefaultCompensationExecutor(ExecutorDependencies{})
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
	executor := NewDefaultCompensationExecutor(ExecutorDependencies{})

	err := executor.Execute(context.Background(), "unknown.operation", json.RawMessage(`{"test":true}`))
	if err == nil {
		t.Fatal("expected unknown operation type to return error")
	}
	if !strings.Contains(err.Error(), "unknown operation type") {
		t.Fatalf("expected unknown operation type error, got %v", err)
	}
}

func TestNewDefaultCompensationExecutor_RegistersBuiltInHandlers(t *testing.T) {
	executor := NewDefaultCompensationExecutor(ExecutorDependencies{})

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

func TestDefaultCompensationExecutor_FailsClosedWithoutRollbackDependencies(t *testing.T) {
	executor := NewDefaultCompensationExecutor(ExecutorDependencies{})

	tests := []struct {
		name          string
		operationType string
		payload       json.RawMessage
		wantFragment  string
	}{
		{
			name:          "account create",
			operationType: "account.create",
			payload:       json.RawMessage(`{"account_id":101,"supplier_id":201}`),
			wantFragment:  "not implemented for production rollback",
		},
		{
			name:          "package publish",
			operationType: "package.publish",
			payload:       json.RawMessage(`{"package_id":102,"supplier_id":202}`),
			wantFragment:  "not implemented for production rollback",
		},
		{
			name:          "settlement withdraw",
			operationType: "settlement.withdraw",
			payload:       json.RawMessage(`{"settlement_id":103,"supplier_id":203}`),
			wantFragment:  "not implemented for production rollback",
		},
		{
			name:          "quota deduct",
			operationType: "quota.deduct",
			payload:       json.RawMessage(`{"package_id":104,"supplier_id":204,"used_quota":12.5}`),
			wantFragment:  "not implemented for production rollback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.Execute(context.Background(), tt.operationType, tt.payload)
			if err == nil {
				t.Fatalf("expected %s compensation to fail closed", tt.operationType)
			}
			if !strings.Contains(err.Error(), tt.wantFragment) {
				t.Fatalf("expected error to contain %q, got %v", tt.wantFragment, err)
			}
		})
	}
}

func TestDefaultCompensationExecutor_RejectsIncompletePayload(t *testing.T) {
	executor := NewDefaultCompensationExecutor(ExecutorDependencies{})

	tests := []struct {
		name          string
		operationType string
		payload       json.RawMessage
		wantFragment  string
	}{
		{
			name:          "account create missing supplier",
			operationType: "account.create",
			payload:       json.RawMessage(`{"account_id":101}`),
			wantFragment:  "supplier_id is required",
		},
		{
			name:          "package publish missing package id",
			operationType: "package.publish",
			payload:       json.RawMessage(`{"supplier_id":202}`),
			wantFragment:  "package_id is required",
		},
		{
			name:          "settlement withdraw missing settlement id",
			operationType: "settlement.withdraw",
			payload:       json.RawMessage(`{"supplier_id":203}`),
			wantFragment:  "settlement_id is required",
		},
		{
			name:          "quota deduct missing used quota",
			operationType: "quota.deduct",
			payload:       json.RawMessage(`{"package_id":104,"supplier_id":204}`),
			wantFragment:  "used_quota must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.Execute(context.Background(), tt.operationType, tt.payload)
			if err == nil {
				t.Fatalf("expected %s compensation to reject incomplete payload", tt.operationType)
			}
			if !strings.Contains(err.Error(), tt.wantFragment) {
				t.Fatalf("expected error to contain %q, got %v", tt.wantFragment, err)
			}
		})
	}
}
