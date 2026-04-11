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
	if !strings.Contains(string(output), "in-memory token runtime is not allowed") {
		t.Fatalf("expected startup failure output to mention in-memory token runtime is not allowed, got: %s", string(output))
	}
}

func TestMainHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	main()
	os.Exit(0)
}
