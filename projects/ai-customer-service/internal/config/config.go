package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTP             HTTPConfig
	Postgres         PostgresConfig
	Webhook          WebhookConfig
	PlatformAdapters PlatformAdaptersConfig
	Runtime          RuntimeConfig
}

type RuntimeConfig struct {
	Env string
}

type HTTPConfig struct {
	Addr              string
	ReadHeaderTimeout int
	ReadTimeout       int
	WriteTimeout      int
	IdleTimeout       int
	MaxHeaderBytes    int
	MaxBodyBytes      int64
}

type PostgresConfig struct {
	Enabled         bool
	DSN             string
	MigrationDir    string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int
}

type WebhookConfig struct {
	Secret          string
	TimestampHeader string
	SignatureHeader string
	MaxSkewSeconds  int
}

type PlatformAdaptersConfig struct {
	Enabled bool
	Sub2API PlatformAdapterProfileConfig
	NewAPI  PlatformAdapterProfileConfig
}

type PlatformAdapterProfileConfig struct {
	Enabled            bool
	IngressSecret      string
	CallbackBaseURL    string
	CallbackSecret     string
	CallbackTimeoutMS  int
	CallbackMaxRetries int
}

func Load() (*Config, error) {
	cfg := &Config{
		HTTP: HTTPConfig{
			Addr:              getEnv("AI_CS_ADDR", ":8080"),
			ReadHeaderTimeout: getEnvInt("AI_CS_READ_HEADER_TIMEOUT_SEC", 5),
			ReadTimeout:       getEnvInt("AI_CS_READ_TIMEOUT_SEC", 10),
			WriteTimeout:      getEnvInt("AI_CS_WRITE_TIMEOUT_SEC", 15),
			IdleTimeout:       getEnvInt("AI_CS_IDLE_TIMEOUT_SEC", 60),
			MaxHeaderBytes:    getEnvInt("AI_CS_MAX_HEADER_BYTES", 1<<20),
			MaxBodyBytes:      getEnvInt64("AI_CS_MAX_BODY_BYTES", 1<<20),
		},
		Postgres: PostgresConfig{
			Enabled:         getEnvBool("AI_CS_POSTGRES_ENABLED", false),
			DSN:             getEnv("AI_CS_POSTGRES_DSN", ""),
			MigrationDir:    getEnv("AI_CS_POSTGRES_MIGRATION_DIR", "db/migration"),
			MaxOpenConns:    getEnvInt("AI_CS_POSTGRES_MAX_OPEN_CONNS", 20),
			MaxIdleConns:    getEnvInt("AI_CS_POSTGRES_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvInt("AI_CS_POSTGRES_CONN_MAX_LIFETIME_SEC", 300),
		},
		Webhook: WebhookConfig{
			Secret:          getEnv("AI_CS_WEBHOOK_SECRET", ""),
			TimestampHeader: getEnv("AI_CS_WEBHOOK_TIMESTAMP_HEADER", "X-CS-Timestamp"),
			SignatureHeader: getEnv("AI_CS_WEBHOOK_SIGNATURE_HEADER", "X-CS-Signature"),
			MaxSkewSeconds:  getEnvInt("AI_CS_WEBHOOK_MAX_SKEW_SECONDS", 300),
		},
		PlatformAdapters: PlatformAdaptersConfig{
			Enabled: getEnvBool("AI_CS_PLATFORM_ADAPTERS_ENABLED", false),
			Sub2API: PlatformAdapterProfileConfig{
				Enabled:            getEnvBool("AI_CS_PLATFORM_SUB2API_ENABLED", false),
				IngressSecret:      getEnv("AI_CS_PLATFORM_SUB2API_INGRESS_SECRET", ""),
				CallbackBaseURL:    getEnv("AI_CS_PLATFORM_SUB2API_CALLBACK_BASE_URL", ""),
				CallbackSecret:     getEnv("AI_CS_PLATFORM_SUB2API_CALLBACK_SECRET", ""),
				CallbackTimeoutMS:  getEnvInt("AI_CS_PLATFORM_SUB2API_CALLBACK_TIMEOUT_MS", 3000),
				CallbackMaxRetries: getEnvInt("AI_CS_PLATFORM_SUB2API_CALLBACK_MAX_RETRIES", 5),
			},
			NewAPI: PlatformAdapterProfileConfig{
				Enabled:            getEnvBool("AI_CS_PLATFORM_NEWAPI_ENABLED", false),
				IngressSecret:      getEnv("AI_CS_PLATFORM_NEWAPI_INGRESS_SECRET", ""),
				CallbackBaseURL:    getEnv("AI_CS_PLATFORM_NEWAPI_CALLBACK_BASE_URL", ""),
				CallbackSecret:     getEnv("AI_CS_PLATFORM_NEWAPI_CALLBACK_SECRET", ""),
				CallbackTimeoutMS:  getEnvInt("AI_CS_PLATFORM_NEWAPI_CALLBACK_TIMEOUT_MS", 3000),
				CallbackMaxRetries: getEnvInt("AI_CS_PLATFORM_NEWAPI_CALLBACK_MAX_RETRIES", 5),
			},
		},
		Runtime: RuntimeConfig{
			Env: normalizeRuntimeEnv(getEnv("AI_CS_RUNTIME_ENV", getEnv("AI_CS_ENV", "development"))),
		},
	}
	if strings.TrimSpace(cfg.HTTP.Addr) == "" {
		return nil, fmt.Errorf("AI_CS_ADDR must not be empty")
	}
	if cfg.HTTP.MaxBodyBytes <= 0 {
		return nil, fmt.Errorf("AI_CS_MAX_BODY_BYTES must be positive")
	}
	if cfg.Postgres.Enabled && strings.TrimSpace(cfg.Postgres.DSN) == "" {
		return nil, fmt.Errorf("AI_CS_POSTGRES_DSN must not be empty when postgres is enabled")
	}
	if cfg.Webhook.MaxSkewSeconds <= 0 {
		return nil, fmt.Errorf("AI_CS_WEBHOOK_MAX_SKEW_SECONDS must be positive")
	}
	if err := validatePlatformProfile("sub2api", cfg.PlatformAdapters.Enabled, cfg.PlatformAdapters.Sub2API); err != nil {
		return nil, err
	}
	if err := validatePlatformProfile("newapi", cfg.PlatformAdapters.Enabled, cfg.PlatformAdapters.NewAPI); err != nil {
		return nil, err
	}
	if cfg.Runtime.Env != "production" && cfg.Runtime.Env != "development" && cfg.Runtime.Env != "test" {
		return nil, fmt.Errorf("AI_CS_RUNTIME_ENV must be one of production/development/test, got: %s", cfg.Runtime.Env)
	}
	if cfg.Runtime.Env == "production" && !cfg.Postgres.Enabled {
		return nil, fmt.Errorf("AI_CS_RUNTIME_ENV=production requires AI_CS_POSTGRES_ENABLED=true, but it is false (memory fallback is not allowed in production)")
	}
	if cfg.Runtime.Env == "production" && strings.TrimSpace(cfg.Webhook.Secret) == "" {
		return nil, fmt.Errorf("AI_CS_WEBHOOK_SECRET must not be empty in production")
	}
	return cfg, nil
}

func validatePlatformProfile(platform string, adaptersEnabled bool, profile PlatformAdapterProfileConfig) error {
	if !adaptersEnabled || !profile.Enabled {
		return nil
	}
	upperPlatform := strings.ToUpper(platform)
	if strings.TrimSpace(profile.IngressSecret) == "" {
		return fmt.Errorf("AI_CS_PLATFORM_%s_INGRESS_SECRET must not be empty when platform ingress is enabled", upperPlatform)
	}
	if profile.CallbackTimeoutMS <= 0 {
		return fmt.Errorf("AI_CS_PLATFORM_%s_CALLBACK_TIMEOUT_MS must be positive", upperPlatform)
	}
	if profile.CallbackMaxRetries < 0 {
		return fmt.Errorf("AI_CS_PLATFORM_%s_CALLBACK_MAX_RETRIES must not be negative", upperPlatform)
	}
	return nil
}

func normalizeRuntimeEnv(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", "dev", "development":
		return "development"
	case "prod", "production":
		return "production"
	case "test":
		return "test"
	default:
		return strings.TrimSpace(strings.ToLower(value))
	}
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
