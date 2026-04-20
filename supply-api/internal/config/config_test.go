package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_UsesDefaultsWithoutConfigFile(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}
	defer func() {
		if chdirErr := os.Chdir(wd); chdirErr != nil {
			t.Fatalf("failed to restore working directory: %v", chdirErr)
		}
	}()

	t.Setenv("SUPPLY_API_SERVER_ADDR", "")
	t.Setenv("SUPPLY_API_TOKEN_ISSUER", "")

	cfg, err := Load("dev")
	if err != nil {
		t.Fatalf("expected defaults to load without config file, got error: %v", err)
	}
	if cfg.Server.Addr != ":18082" {
		t.Fatalf("expected default addr :18082, got %s", cfg.Server.Addr)
	}
	if cfg.Token.Issuer != "lijiaoqiao/supply-api" {
		t.Fatalf("expected default issuer lijiaoqiao/supply-api, got %s", cfg.Token.Issuer)
	}
}

func TestDatabaseConfigDSN_UsesUnixSocketFormat(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "/var/run/postgresql",
		Port:     5432,
		User:     "long",
		Password: "secret",
		Database: "supply_api",
	}

	got := cfg.DSN()
	want := "host=/var/run/postgresql user=long dbname=supply_api sslmode=disable"
	if got != want {
		t.Fatalf("expected DSN %q, got %q", want, got)
	}
}

func TestDatabaseConfigSafeDSN_UsesUnixSocketFormat(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "/var/run/postgresql",
		Port:     5432,
		User:     "long",
		Password: "secret",
		Database: "supply_api",
	}

	got := cfg.SafeDSN()
	want := "host=/var/run/postgresql user=long dbname=supply_api sslmode=disable"
	if got != want {
		t.Fatalf("expected safe DSN %q, got %q", want, got)
	}
}

func TestLoadFromPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "custom.yaml")
	content := []byte(`
server:
  addr: ":19090"
database:
  host: "db.internal"
token:
  issuer: "custom-issuer"
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromPath("dev", configPath)
	if err != nil {
		t.Fatalf("LoadFromPath returned error: %v", err)
	}

	if cfg.Server.Addr != ":19090" {
		t.Fatalf("expected addr :19090, got %s", cfg.Server.Addr)
	}
	if cfg.Database.Host != "db.internal" {
		t.Fatalf("expected database host db.internal, got %s", cfg.Database.Host)
	}
	if cfg.Token.Issuer != "custom-issuer" {
		t.Fatalf("expected token issuer custom-issuer, got %s", cfg.Token.Issuer)
	}
}

func TestLoadFromPath_MissingFile(t *testing.T) {
	_, err := LoadFromPath("dev", filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("expected missing config file to return error")
	}
}

func TestLoadFromPath_ProdRejectsDefaultSupplierIDFallback(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "prod.yaml")
	content := []byte(`
server:
  addr: ":19090"
database:
  host: "db.internal"
  user: "postgres"
  password: "secret"
  database: "supply_db"
token:
  issuer: "prod-issuer"
  secret_key: "prod-secret"
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := LoadFromPath("prod", configPath)
	if err == nil {
		t.Fatal("expected prod config with default supplier fallback to return error")
	}
	if !strings.Contains(err.Error(), "default_supplier_id") {
		t.Fatalf("expected error to mention default_supplier_id, got %v", err)
	}
}

func TestLoadFromPath_ProdRejectsHS256Algorithm(t *testing.T) {
	// 清除环境变量以确保测试隔离
	origVal := os.Getenv("SUPPLY_TOKEN_SECRET_KEY")
	os.Unsetenv("SUPPLY_TOKEN_SECRET_KEY")
	defer func() {
		if origVal != "" {
			os.Setenv("SUPPLY_TOKEN_SECRET_KEY", origVal)
		}
	}()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "prod.yaml")
	content := []byte(`
server:
  addr: ":19090"
  default_supplier_id: 0
database:
  host: "db.internal"
  user: "postgres"
  password: "secret"
  database: "supply_db"
token:
  issuer: "prod-issuer"
  algorithm: "HS256"
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := LoadFromPath("prod", configPath)
	if err == nil {
		t.Fatal("expected prod config with HS256 algorithm to return error")
	}
	if !strings.Contains(err.Error(), "token.algorithm") {
		t.Fatalf("expected error to mention token.algorithm, got %v", err)
	}
}

func TestLoadFromPath_ProdRejectsWithdrawEnabledUntilSMSIntegrated(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "prod.yaml")
	content := []byte(`
server:
  addr: ":19090"
  default_supplier_id: 0
database:
  host: "db.internal"
  user: "postgres"
  password: "secret"
  database: "supply_db"
token:
  issuer: "prod-issuer"
  algorithm: "HS256"
  secret_key: "prod-secret"
settlement:
  withdraw_enabled: true
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := LoadFromPath("prod", configPath)
	if err == nil {
		t.Fatal("expected prod config with withdraw enabled but no integrated SMS path to return error")
	}
	if !strings.Contains(err.Error(), "withdraw_enabled") {
		t.Fatalf("expected error to mention withdraw_enabled, got %v", err)
	}
}

// ─── P0-02: DSN/SafeDSN 格式化 BUG TDD 测试 ─────────────────────────────────

// TestDSN_TCPConnection_ContainsRealPassword 红色测试：
// 验证 DSN() 输出格式为 postgres://user:password@host:port/db，密码在正确位置
func TestDSN_TCPConnection_ContainsRealPassword(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "db.internal",
		Port:     5432,
		User:     "app_user",
		Password: "super_secret_123",
		Database: "supply_api",
	}

	got := cfg.DSN()
	t.Logf("DSN actual output: %s", got)

	// DSN() 应包含真实密码
	if !strings.Contains(got, "super_secret_123") {
		t.Fatalf("DSN() should contain real password, got: %s", got)
	}
	// 正确格式: postgres://app_user:super_secret_123@db.internal:5432/supply_api?sslmode=disable
	want := "postgres://app_user:super_secret_123@db.internal:5432/supply_api"
	if !strings.HasPrefix(got, want) {
		t.Fatalf("DSN() format incorrect.\n  want prefix: %s\n  got:         %s", want, got)
	}
}

// TestSafeDSN_TCPConnection_MasksPassword 红色测试：
// 验证 SafeDSN() 输出格式为 postgres://user:***@host:port/db，密码被正确脱敏
func TestSafeDSN_TCPConnection_MasksPassword(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "db.internal",
		Port:     5432,
		User:     "app_user",
		Password: "super_secret_123",
		Database: "supply_api",
	}

	got := cfg.SafeDSN()

	// SafeDSN() 不应包含真实密码
	if strings.Contains(got, "super_secret_123") {
		t.Fatalf("SafeDSN() should NOT contain real password, got: %s", got)
	}
	// 应包含脱敏密码 *** 字样
	if !strings.Contains(got, ":***@") {
		t.Fatalf("SafeDSN() should contain :***@ (masked password), got: %s", got)
	}
	// 正确格式: postgres://app_user:***@db.internal:5432/supply_api?sslmode=disable
	want := "postgres://app_user:***@db.internal:5432/supply_api"
	if !strings.HasPrefix(got, want) {
		t.Fatalf("SafeDSN() format incorrect.\n  want prefix: %s\n  got:         %s", want, got)
	}
}
