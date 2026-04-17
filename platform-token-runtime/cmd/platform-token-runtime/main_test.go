package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestMain_ProdRejectsInMemoryRuntime(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestMainHelperProcess")
	cmd.Env = append(os.Environ(),
		"GO_WANT_HELPER_PROCESS=1",
		"TOKEN_RUNTIME_ENV=prod",
		"TOKEN_RUNTIME_ADDR=127.0.0.1:0",
	)
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("expected prod startup to fail fast, but process timed out. output=%s", string(output))
	}
	if err == nil {
		t.Fatalf("expected prod startup to fail, but process exited successfully. output=%s", string(output))
	}
	if !strings.Contains(string(output), "runtime store is required") {
		t.Fatalf("expected startup failure output to mention runtime store is required, got: %s", string(output))
	}
}

func TestMain_ProdWithDatabaseURLDoesNotFailForMissingStore(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestMainHelperProcess")
	cmd.Env = append(os.Environ(),
		"GO_WANT_HELPER_PROCESS=1",
		"TOKEN_RUNTIME_ENV=prod",
		"TOKEN_RUNTIME_ADDR=127.0.0.1:0",
		"TOKEN_RUNTIME_DATABASE_URL=postgres://postgres:secret@127.0.0.1:1/token_runtime?sslmode=disable",
	)
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("expected prod startup with database url to fail fast, but process timed out. output=%s", string(output))
	}
	if err == nil {
		t.Fatalf("expected prod startup to fail because database is unavailable. output=%s", string(output))
	}
	if strings.Contains(string(output), "runtime store is required") {
		t.Fatalf("expected startup to attempt postgres store wiring before missing-store check, got: %s", string(output))
	}
}

func TestMainHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	main()
	os.Exit(0)
}
