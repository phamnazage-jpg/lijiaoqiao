package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain_ProdStartupFailsWhenDatabaseUnavailable(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.prod.yaml")
	content := []byte(fmt.Sprintf(`
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
  algorithm: "RS256"
  public_key: |
%s
`, indentYAMLBlock(mustGenerateRSAPublicKeyPEM(t), "    ")))
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

func TestMain_RejectsUnsupportedEnvBeforeLoadingConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestMainHelperProcess", "--", "-env", "qa")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("expected unsupported env to fail fast, but process timed out. output=%s", string(output))
	}
	if err == nil {
		t.Fatalf("expected unsupported env to fail, but process exited successfully. output=%s", string(output))
	}
	if !strings.Contains(string(output), "unsupported env") {
		t.Fatalf("expected unsupported env error, got: %s", string(output))
	}
}

func TestMain_RejectsDevConfigPathForStaging(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.dev.yaml")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestMainHelperProcess", "--", "-env", "staging", "-config", configPath)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("expected staging + dev config mismatch to fail fast, but process timed out. output=%s", string(output))
	}
	if err == nil {
		t.Fatalf("expected staging + dev config mismatch to fail, but process exited successfully. output=%s", string(output))
	}
	if !strings.Contains(string(output), "dev template") {
		t.Fatalf("expected output to mention dev template mismatch, got: %s", string(output))
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

func mustGenerateRSAPublicKeyPEM(t *testing.T) string {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal RSA public key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func indentYAMLBlock(value, indent string) string {
	lines := strings.Split(strings.TrimRight(value, "\n"), "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}
