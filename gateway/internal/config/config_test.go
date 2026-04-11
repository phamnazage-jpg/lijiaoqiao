package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestConfig_Struct(t *testing.T) {
	cfg := &Config{}

	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestServerConfig_Struct(t *testing.T) {
	cfg := ServerConfig{
		Host:         "localhost",
		Port:         8080,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if cfg.Host != "localhost" {
		t.Errorf("expected host localhost, got %s", cfg.Host)
	}
	if cfg.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Port)
	}
}

func TestDatabaseConfig_Struct(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "secret",
		Database: "gateway",
		MaxConns: 10,
	}

	if cfg.Host != "localhost" {
		t.Errorf("expected host localhost, got %s", cfg.Host)
	}
	if cfg.Port != 5432 {
		t.Errorf("expected port 5432, got %d", cfg.Port)
	}
	if cfg.MaxConns != 10 {
		t.Errorf("expected max conns 10, got %d", cfg.MaxConns)
	}
}

func TestRedisConfig_Struct(t *testing.T) {
	cfg := RedisConfig{
		Host:     "localhost",
		Port:     6379,
		Password: "",
		DB:       0,
		PoolSize: 10,
	}

	if cfg.Host != "localhost" {
		t.Errorf("expected host localhost, got %s", cfg.Host)
	}
	if cfg.Port != 6379 {
		t.Errorf("expected port 6379, got %d", cfg.Port)
	}
}

func TestRouterConfig_Struct(t *testing.T) {
	cfg := RouterConfig{
		Strategy:            "latency",
		Timeout:             30 * time.Second,
		MaxRetries:          3,
		RetryDelay:          1 * time.Second,
		HealthCheckInterval: 10 * time.Second,
	}

	if cfg.Strategy != "latency" {
		t.Errorf("expected strategy latency, got %s", cfg.Strategy)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", cfg.MaxRetries)
	}
}

func TestRateLimitConfig_Struct(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled:         true,
		Algorithm:       "token_bucket",
		DefaultRPM:      60,
		DefaultTPM:      60000,
		BurstMultiplier: 1.5,
	}

	if !cfg.Enabled {
		t.Error("expected enabled")
	}
	if cfg.Algorithm != "token_bucket" {
		t.Errorf("expected algorithm token_bucket, got %s", cfg.Algorithm)
	}
	if cfg.DefaultRPM != 60 {
		t.Errorf("expected default RPM 60, got %d", cfg.DefaultRPM)
	}
}

func TestAlertConfig_Struct(t *testing.T) {
	cfg := AlertConfig{
		Enabled: true,
		Email: EmailConfig{
			Enabled: false,
			Host:    "smtp.example.com",
			Port:    587,
			From:    "alert@example.com",
			To:      []string{"admin@example.com"},
		},
		DingTalk: DingTalkConfig{
			Enabled: false,
			WebHook: "",
			Secret:  "",
		},
		Feishu: FeishuConfig{
			Enabled: false,
			WebHook: "",
			Secret:  "",
		},
	}

	if !cfg.Enabled {
		t.Error("expected enabled")
	}
	if cfg.Email.Port != 587 {
		t.Errorf("expected email port 587, got %d", cfg.Email.Port)
	}
}

func TestProviderConfig_Struct(t *testing.T) {
	cfg := ProviderConfig{
		Name:     "openai",
		Type:     "openai",
		BaseURL:  "https://api.openai.com",
		APIKey:   "sk-test",
		Models:   []string{"gpt-4", "gpt-3.5-turbo"},
		Priority: 1,
		Weight:   1.0,
	}

	if cfg.Name != "openai" {
		t.Errorf("expected name openai, got %s", cfg.Name)
	}
	if cfg.Type != "openai" {
		t.Errorf("expected type openai, got %s", cfg.Type)
	}
	if len(cfg.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(cfg.Models))
	}
	if cfg.Priority != 1 {
		t.Errorf("expected priority 1, got %d", cfg.Priority)
	}
}

func TestGetEnv(t *testing.T) {
	// 设置环境变量
	os.Setenv("TEST_KEY", "test_value")
	defer os.Unsetenv("TEST_KEY")

	tests := []struct {
		key          string
		defaultValue string
		expected     string
	}{
		{"TEST_KEY", "default", "test_value"},
		{"NON_EXISTENT_KEY", "default", "default"},
	}

	for _, tt := range tests {
		result := getEnv(tt.key, tt.defaultValue)
		if result != tt.expected {
			t.Errorf("getEnv(%s, %s) = %s, want %s", tt.key, tt.defaultValue, result, tt.expected)
		}
	}
}

func TestGetEnv_EmptyString(t *testing.T) {
	// 设置环境变量为空字符串
	os.Setenv("EMPTY_KEY", "")
	defer os.Unsetenv("EMPTY_KEY")

	// 空字符串环境变量应该返回默认值
	result := getEnv("EMPTY_KEY", "default")
	if result != "default" {
		t.Errorf("expected default, got %s", result)
	}
}

func TestLoadConfig(t *testing.T) {
	// 设置测试环境变量
	os.Setenv("GATEWAY_HOST", "127.0.0.1")
	os.Setenv("DINGTALK_ENABLED", "true")
	os.Setenv("DINGTALK_WEBHOOK", "https://test.com/webhook")
	os.Setenv("DINGTALK_SECRET", "test-secret")
	defer func() {
		os.Unsetenv("GATEWAY_HOST")
		os.Unsetenv("DINGTALK_ENABLED")
		os.Unsetenv("DINGTALK_WEBHOOK")
		os.Unsetenv("DINGTALK_SECRET")
	}()

	cfg, err := LoadConfig("/tmp/test.yaml")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证Server配置
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("expected read timeout 30s, got %v", cfg.Server.ReadTimeout)
	}

	// 验证Router配置
	if cfg.Router.Strategy != "latency" {
		t.Errorf("expected strategy latency, got %s", cfg.Router.Strategy)
	}
	if cfg.Router.MaxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", cfg.Router.MaxRetries)
	}

	// 验证RateLimit配置
	if !cfg.RateLimit.Enabled {
		t.Error("expected rate limit enabled")
	}
	if cfg.RateLimit.Algorithm != "token_bucket" {
		t.Errorf("expected token_bucket, got %s", cfg.RateLimit.Algorithm)
	}
	if cfg.RateLimit.DefaultRPM != 60 {
		t.Errorf("expected RPM 60, got %d", cfg.RateLimit.DefaultRPM)
	}
	if cfg.RateLimit.BurstMultiplier != 1.5 {
		t.Errorf("expected burst multiplier 1.5, got %f", cfg.RateLimit.BurstMultiplier)
	}

	// 验证Alert配置
	if !cfg.Alert.Enabled {
		t.Error("expected alert enabled")
	}
	if !cfg.Alert.DingTalk.Enabled {
		t.Error("expected DingTalk enabled")
	}
	if cfg.Alert.DingTalk.WebHook != "https://test.com/webhook" {
		t.Errorf("unexpected DingTalk webhook: %s", cfg.Alert.DingTalk.WebHook)
	}
}

func TestLoadConfig_DefaultValues(t *testing.T) {
	// 确保默认环境变量未设置
	os.Unsetenv("GATEWAY_HOST")
	os.Unsetenv("DINGTALK_ENABLED")
	os.Unsetenv("DINGTALK_WEBHOOK")
	os.Unsetenv("DINGTALK_SECRET")

	cfg, err := LoadConfig("/tmp/test.yaml")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected default host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
}

func TestEmailConfig_Empty(t *testing.T) {
	cfg := EmailConfig{}

	if cfg.Enabled {
		t.Error("expected not enabled")
	}
	if cfg.Host != "" {
		t.Errorf("expected empty host, got %s", cfg.Host)
	}
	if len(cfg.To) != 0 {
		t.Errorf("expected empty To slice, got %d", len(cfg.To))
	}
}

func TestDingTalkConfig_Empty(t *testing.T) {
	cfg := DingTalkConfig{}

	if cfg.Enabled {
		t.Error("expected not enabled")
	}
	if cfg.WebHook != "" {
		t.Errorf("expected empty webhook, got %s", cfg.WebHook)
	}
	if cfg.Secret != "" {
		t.Errorf("expected empty secret, got %s", cfg.Secret)
	}
}

func TestFeishuConfig_Empty(t *testing.T) {
	cfg := FeishuConfig{}

	if cfg.Enabled {
		t.Error("expected not enabled")
	}
	if cfg.WebHook != "" {
		t.Errorf("expected empty webhook, got %s", cfg.WebHook)
	}
}

func TestConfig_AllFields(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Host:         "localhost",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "postgres",
			Password: "secret",
			Database: "gateway",
			MaxConns: 10,
		},
		Redis: RedisConfig{
			Host:     "localhost",
			Port:     6379,
			Password: "",
			DB:       0,
			PoolSize: 10,
		},
		Router: RouterConfig{
			Strategy:            "latency",
			Timeout:             30 * time.Second,
			MaxRetries:          3,
			RetryDelay:          1 * time.Second,
			HealthCheckInterval: 10 * time.Second,
		},
		RateLimit: RateLimitConfig{
			Enabled:         true,
			Algorithm:       "token_bucket",
			DefaultRPM:      60,
			DefaultTPM:      60000,
			BurstMultiplier: 1.5,
		},
		Alert: AlertConfig{
			Enabled: true,
			Email: EmailConfig{
				Enabled: false,
				Host:    "smtp.example.com",
				Port:    587,
			},
		},
		Providers: []ProviderConfig{
			{
				Name:     "openai",
				Type:     "openai",
				BaseURL:  "https://api.openai.com",
				APIKey:   "sk-test",
				Models:   []string{"gpt-4"},
				Priority: 1,
				Weight:   1.0,
			},
		},
	}

	if len(cfg.Providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(cfg.Providers))
	}
	if cfg.Providers[0].Name != "openai" {
		t.Errorf("expected provider name openai, got %s", cfg.Providers[0].Name)
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	// 测试环境变量覆盖
	os.Setenv("SMTP_HOST", "custom.smtp.com")
	os.Setenv("SMTP_PORT", "465")
	defer func() {
		os.Unsetenv("SMTP_HOST")
		os.Unsetenv("SMTP_PORT")
	}()

	cfg, err := LoadConfig("/tmp/test.yaml")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Alert.Email.Host != "custom.smtp.com" {
		t.Errorf("expected custom.smtp.com, got %s", cfg.Alert.Email.Host)
	}
}

func TestLoadConfig_ProdRejectsInMemoryTokenRuntime(t *testing.T) {
	t.Setenv("GATEWAY_ENV", "prod")
	t.Setenv("GATEWAY_TOKEN_RUNTIME_MODE", "inmemory")

	_, err := LoadConfig("")
	if err == nil {
		t.Fatal("expected prod config with in-memory token runtime to return error")
	}
	if !strings.Contains(err.Error(), "inmemory") {
		t.Fatalf("expected error to mention inmemory runtime, got %v", err)
	}
}

func TestLoadConfig_ProdRequiresTokenRuntimeURL(t *testing.T) {
	t.Setenv("GATEWAY_ENV", "prod")
	t.Setenv("GATEWAY_TOKEN_RUNTIME_MODE", "remote_introspection")
	t.Setenv("GATEWAY_TOKEN_RUNTIME_URL", "")

	_, err := LoadConfig("")
	if err == nil {
		t.Fatal("expected prod config without token runtime URL to return error")
	}
	if !strings.Contains(err.Error(), "TOKEN_RUNTIME_URL") {
		t.Fatalf("expected error to mention TOKEN_RUNTIME_URL, got %v", err)
	}
}
