package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain_ProdStartupFailsWhenDatabaseUnavailable(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.prod.yaml")
	content := []byte(`
server:
  addr: "127.0.0.1:0"
  shutdown_timeout: 1s
  default_supplier_id: 0
database:
  host: "127.0.0.1"
  port: 1
  user: "postgres"
  password: "secret"
  database: "supply_db"
redis:
  host: "127.0.0.1"
  port: 1
token:
  issuer: "prod-issuer"
  secret_key: "prod-secret"
  algorithm: "HS256"
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestMainHelperProcess", "--", "-env", "prod", "-config", configPath)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("expected prod startup to fail fast, but process timed out. output=%s", string(output))
	}
	if err == nil {
		t.Fatalf("expected prod startup to fail, but process exited successfully. output=%s", string(output))
	}
	if !strings.Contains(string(output), "production startup requirement failed") {
		t.Fatalf("expected startup failure output to mention production startup requirement failed, got: %s", string(output))
	}
}

func TestMainHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	for i, arg := range os.Args {
		if arg != "--" {
			continue
		}
		os.Args = append([]string{os.Args[0]}, os.Args[i+1:]...)
		break
	}

	main()
	os.Exit(0)
}
