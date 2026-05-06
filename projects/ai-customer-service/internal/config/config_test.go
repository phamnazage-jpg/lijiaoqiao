package config

import (
	"strings"
	"testing"
)

func TestGetEnvBool_True(t *testing.T) {
	t.Setenv("TEST_BOOL", "true")
	got := getEnvBool("TEST_BOOL", false)
	if !got {
		t.Error("getEnvBool(true) = false, want true")
	}
}

func TestGetEnvBool_TrueCaseInsensitive(t *testing.T) {
	t.Setenv("TEST_BOOL", "TRUE")
	got := getEnvBool("TEST_BOOL", false)
	if !got {
		t.Error("getEnvBool(TRUE) = false, want true")
	}
}

func TestGetEnvBool_False(t *testing.T) {
	t.Setenv("TEST_BOOL", "false")
	got := getEnvBool("TEST_BOOL", true)
	if got {
		t.Error("getEnvBool(false) = true, want false")
	}
}

func TestGetEnvBool_One(t *testing.T) {
	t.Setenv("TEST_BOOL", "1")
	got := getEnvBool("TEST_BOOL", false)
	if !got {
		t.Error("getEnvBool(1) = false, want true")
	}
}

func TestGetEnvBool_Zero(t *testing.T) {
	t.Setenv("TEST_BOOL", "0")
	got := getEnvBool("TEST_BOOL", true)
	if got {
		t.Error("getEnvBool(0) = true, want false")
	}
}

func TestGetEnvBool_Yes(t *testing.T) {
	t.Setenv("TEST_BOOL", "yes")
	got := getEnvBool("TEST_BOOL", false)
	if !got {
		t.Error("getEnvBool(yes) = false, want true")
	}
}

func TestGetEnvBool_InvalidValueFallsBack(t *testing.T) {
	t.Setenv("TEST_BOOL", "maybe")
	got := getEnvBool("TEST_BOOL", true)
	if !got {
		t.Error("getEnvBool(maybe) did not return fallback, got false, want true")
	}
}

func TestGetEnvInt_ValidValue(t *testing.T) {
	t.Setenv("TEST_INT", "999")
	got := getEnvInt("TEST_INT", 5)
	if got != 999 {
		t.Errorf("getEnvInt(TEST_INT) = %d, want 999", got)
	}
}

func TestGetEnvInt_InvalidValue(t *testing.T) {
	t.Setenv("TEST_INT", "notanumber")
	got := getEnvInt("TEST_INT", 42)
	if got != 42 {
		t.Errorf("getEnvInt(invalid) = %d, want fallback 42", got)
	}
}

func TestGetEnvInt64_ValidValue(t *testing.T) {
	t.Setenv("TEST_INT64", "12345678901234")
	got := getEnvInt64("TEST_INT64", 0)
	if got != 12345678901234 {
		t.Errorf("getEnvInt64(TEST_INT64) = %d, want 12345678901234", got)
	}
}

func TestGetEnvIntList_ValidValue(t *testing.T) {
	t.Setenv("TEST_INT_LIST", "10,30,60")
	got := getEnvIntList("TEST_INT_LIST", []int{1})
	if len(got) != 3 || got[0] != 10 || got[1] != 30 || got[2] != 60 {
		t.Fatalf("getEnvIntList(TEST_INT_LIST) = %v, want [10 30 60]", got)
	}
}

func TestGetEnvIntList_InvalidValueFallsBack(t *testing.T) {
	t.Setenv("TEST_INT_LIST", "10,oops,60")
	got := getEnvIntList("TEST_INT_LIST", []int{1, 2})
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("getEnvIntList(invalid) = %v, want [1 2]", got)
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("AI_CS_ADDR", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTP.Addr != ":8080" {
		t.Fatalf("addr = %s, want :8080", cfg.HTTP.Addr)
	}
	if cfg.HTTP.MaxBodyBytes <= 0 {
		t.Fatalf("expected positive max body bytes")
	}
	if cfg.Webhook.TimestampHeader != "X-CS-Timestamp" {
		t.Fatalf("timestamp header = %s", cfg.Webhook.TimestampHeader)
	}
	if cfg.Runtime.Env != "development" {
		t.Fatalf("runtime env = %s, want development", cfg.Runtime.Env)
	}
}

func TestLoadOverride(t *testing.T) {
	t.Setenv("AI_CS_ADDR", ":18080")
	t.Setenv("AI_CS_MAX_BODY_BYTES", "2048")
	t.Setenv("AI_CS_WEBHOOK_SECRET", "secret")
	t.Setenv("AI_CS_WEBHOOK_MAX_SKEW_SECONDS", "60")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTP.Addr != ":18080" {
		t.Fatalf("addr = %s, want :18080", cfg.HTTP.Addr)
	}
	if cfg.HTTP.MaxBodyBytes != 2048 {
		t.Fatalf("max body bytes = %d, want 2048", cfg.HTTP.MaxBodyBytes)
	}
	if cfg.Webhook.Secret != "secret" {
		t.Fatalf("expected webhook secret")
	}
	if cfg.Webhook.MaxSkewSeconds != 60 {
		t.Fatalf("skew = %d, want 60", cfg.Webhook.MaxSkewSeconds)
	}
}

func TestLoad_RuntimeEnvFallsBackToLegacyEnv(t *testing.T) {
	t.Setenv("AI_CS_RUNTIME_ENV", "")
	t.Setenv("AI_CS_ENV", "prod")
	t.Setenv("AI_CS_POSTGRES_ENABLED", "true")
	t.Setenv("AI_CS_POSTGRES_DSN", "postgres://user:***@localhost:5432/db?sslmode=disable")
	t.Setenv("AI_CS_WEBHOOK_SECRET", "secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Runtime.Env != "production" {
		t.Fatalf("runtime env = %s, want production", cfg.Runtime.Env)
	}
}

func TestLoad_RuntimeEnvOverridesLegacyEnv(t *testing.T) {
	t.Setenv("AI_CS_RUNTIME_ENV", "test")
	t.Setenv("AI_CS_ENV", "prod")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Runtime.Env != "test" {
		t.Fatalf("runtime env = %s, want test", cfg.Runtime.Env)
	}
}

func TestLoad_RuntimeEnvNormalizesAliases(t *testing.T) {
	t.Setenv("AI_CS_RUNTIME_ENV", "dev")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Runtime.Env != "development" {
		t.Fatalf("runtime env = %s, want development", cfg.Runtime.Env)
	}
}

func TestLoad_RejectsInvalidRuntimeEnv(t *testing.T) {
	t.Setenv("AI_CS_RUNTIME_ENV", "staging")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid runtime env")
	}
	if !strings.Contains(err.Error(), "AI_CS_RUNTIME_ENV") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_RejectsProdWhenPostgresDisabled(t *testing.T) {
	t.Setenv("AI_CS_RUNTIME_ENV", "prod")
	t.Setenv("AI_CS_POSTGRES_ENABLED", "false")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when prod runs without postgres")
	}
	if !strings.Contains(err.Error(), "AI_CS_POSTGRES_ENABLED") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_RejectsProdWhenWebhookSecretMissing(t *testing.T) {
	t.Setenv("AI_CS_RUNTIME_ENV", "production")
	t.Setenv("AI_CS_POSTGRES_ENABLED", "true")
	t.Setenv("AI_CS_POSTGRES_DSN", "postgres://user:***@localhost:5432/db?sslmode=disable")
	t.Setenv("AI_CS_WEBHOOK_SECRET", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when prod runs without webhook secret")
	}
	if !strings.Contains(err.Error(), "AI_CS_WEBHOOK_SECRET") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_PlatformAdaptersDisabled_IgnoresPlatformSecrets(t *testing.T) {
	t.Setenv("AI_CS_PLATFORM_ADAPTERS_ENABLED", "false")
	t.Setenv("AI_CS_PLATFORM_SUB2API_ENABLED", "true")
	t.Setenv("AI_CS_PLATFORM_SUB2API_INGRESS_SECRET", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.PlatformAdapters.Enabled {
		t.Fatalf("platform adapters enabled = true, want false")
	}
	if !cfg.PlatformAdapters.Sub2API.Enabled {
		t.Fatalf("sub2api enabled = false, want true")
	}
}

func TestLoad_RejectsEnabledSub2APIWithoutIngressSecret(t *testing.T) {
	t.Setenv("AI_CS_PLATFORM_ADAPTERS_ENABLED", "true")
	t.Setenv("AI_CS_PLATFORM_SUB2API_ENABLED", "true")
	t.Setenv("AI_CS_PLATFORM_SUB2API_INGRESS_SECRET", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when sub2api ingress secret is missing")
	}
	if !strings.Contains(err.Error(), "AI_CS_PLATFORM_SUB2API_INGRESS_SECRET") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_RejectsEnabledSub2APIWithInvalidWorkerPollingConfig(t *testing.T) {
	t.Setenv("AI_CS_PLATFORM_ADAPTERS_ENABLED", "true")
	t.Setenv("AI_CS_PLATFORM_SUB2API_ENABLED", "true")
	t.Setenv("AI_CS_PLATFORM_SUB2API_INGRESS_SECRET", "sub2api-secret")
	t.Setenv("AI_CS_PLATFORM_SUB2API_CALLBACK_POLL_INTERVAL_MS", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when sub2api callback poll interval is invalid")
	}
	if !strings.Contains(err.Error(), "AI_CS_PLATFORM_SUB2API_CALLBACK_POLL_INTERVAL_MS") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_PlatformAdapterOverrides(t *testing.T) {
	t.Setenv("AI_CS_PLATFORM_ADAPTERS_ENABLED", "true")
	t.Setenv("AI_CS_PLATFORM_SUB2API_ENABLED", "true")
	t.Setenv("AI_CS_PLATFORM_SUB2API_INGRESS_SECRET", "sub2api-secret")
	t.Setenv("AI_CS_PLATFORM_SUB2API_CALLBACK_BASE_URL", "https://callback.example.com/sub2api")
	t.Setenv("AI_CS_PLATFORM_SUB2API_CALLBACK_SECRET", "cb-secret")
	t.Setenv("AI_CS_PLATFORM_SUB2API_CALLBACK_TIMEOUT_MS", "4000")
	t.Setenv("AI_CS_PLATFORM_SUB2API_CALLBACK_MAX_RETRIES", "7")
	t.Setenv("AI_CS_PLATFORM_SUB2API_CALLBACK_POLL_INTERVAL_MS", "2500")
	t.Setenv("AI_CS_PLATFORM_SUB2API_CALLBACK_BATCH_SIZE", "12")
	t.Setenv("AI_CS_PLATFORM_SUB2API_CALLBACK_RETRY_SCHEDULE_SEC", "5,15,45")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.PlatformAdapters.Enabled {
		t.Fatalf("platform adapters enabled = false, want true")
	}
	if !cfg.PlatformAdapters.Sub2API.Enabled {
		t.Fatalf("sub2api enabled = false, want true")
	}
	if cfg.PlatformAdapters.Sub2API.IngressSecret != "sub2api-secret" {
		t.Fatalf("sub2api ingress secret = %s, want sub2api-secret", cfg.PlatformAdapters.Sub2API.IngressSecret)
	}
	if cfg.PlatformAdapters.Sub2API.CallbackBaseURL != "https://callback.example.com/sub2api" {
		t.Fatalf("sub2api callback base url = %s", cfg.PlatformAdapters.Sub2API.CallbackBaseURL)
	}
	if cfg.PlatformAdapters.Sub2API.CallbackSecret != "cb-secret" {
		t.Fatalf("sub2api callback secret = %s", cfg.PlatformAdapters.Sub2API.CallbackSecret)
	}
	if cfg.PlatformAdapters.Sub2API.CallbackTimeoutMS != 4000 {
		t.Fatalf("sub2api callback timeout ms = %d, want 4000", cfg.PlatformAdapters.Sub2API.CallbackTimeoutMS)
	}
	if cfg.PlatformAdapters.Sub2API.CallbackMaxRetries != 7 {
		t.Fatalf("sub2api callback max retries = %d, want 7", cfg.PlatformAdapters.Sub2API.CallbackMaxRetries)
	}
	if cfg.PlatformAdapters.Sub2API.CallbackPollIntervalMS != 2500 {
		t.Fatalf("sub2api callback poll interval ms = %d, want 2500", cfg.PlatformAdapters.Sub2API.CallbackPollIntervalMS)
	}
	if cfg.PlatformAdapters.Sub2API.CallbackBatchSize != 12 {
		t.Fatalf("sub2api callback batch size = %d, want 12", cfg.PlatformAdapters.Sub2API.CallbackBatchSize)
	}
	if len(cfg.PlatformAdapters.Sub2API.CallbackRetrySchedule) != 3 || cfg.PlatformAdapters.Sub2API.CallbackRetrySchedule[0] != 5 || cfg.PlatformAdapters.Sub2API.CallbackRetrySchedule[1] != 15 || cfg.PlatformAdapters.Sub2API.CallbackRetrySchedule[2] != 45 {
		t.Fatalf("sub2api callback retry schedule = %v, want [5 15 45]", cfg.PlatformAdapters.Sub2API.CallbackRetrySchedule)
	}
}
