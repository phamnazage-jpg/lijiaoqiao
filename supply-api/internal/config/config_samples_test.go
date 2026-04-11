package config

import (
	"path/filepath"
	"testing"
)

func TestDevSampleConfigLoads(t *testing.T) {
	configPath := filepath.Join("..", "..", "config", "config.dev.yaml")

	cfg, err := LoadFromPath("dev", configPath)
	if err != nil {
		t.Fatalf("expected dev sample config to load, got error: %v", err)
	}

	if cfg.Server.Addr != ":18082" {
		t.Fatalf("expected addr :18082, got %s", cfg.Server.Addr)
	}
	if cfg.Database.Host != "localhost" {
		t.Fatalf("expected database host localhost, got %s", cfg.Database.Host)
	}
	if cfg.Redis.Host != "localhost" {
		t.Fatalf("expected redis host localhost, got %s", cfg.Redis.Host)
	}
	if cfg.Token.Issuer != "lijiaoqiao/supply-api" {
		t.Fatalf("expected issuer lijiaoqiao/supply-api, got %s", cfg.Token.Issuer)
	}
	if cfg.Settlement.WithdrawEnabled {
		t.Fatal("expected dev sample config to keep withdraw disabled by default")
	}
}
