package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestLoadFromPath_ProdRejectsMissingHS256SecretKey(t *testing.T) {
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
		t.Fatal("expected prod config without HS256 secret key to return error")
	}
	if !strings.Contains(err.Error(), "token.secret_key") {
		t.Fatalf("expected error to mention token.secret_key, got %v", err)
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
