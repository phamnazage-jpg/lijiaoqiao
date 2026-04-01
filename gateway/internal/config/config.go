package config

import (
	"os"
	"time"
)

// Config 网关配置
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Router   RouterConfig
	RateLimit RateLimitConfig
	Alert    AlertConfig
	Providers []ProviderConfig
}

// ServerConfig 服务配置
type ServerConfig struct {
	Host string
	Port int
	ReadTimeout time.Duration
	WriteTimeout time.Duration
	IdleTimeout time.Duration
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	MaxConns int
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
}

// RouterConfig 路由配置
type RouterConfig struct {
	Strategy        string // "latency", "cost", "availability", "weighted"
	Timeout         time.Duration
	MaxRetries      int
	RetryDelay      time.Duration
	HealthCheckInterval time.Duration
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled       bool
	Algorithm     string // "token_bucket", "sliding_window", "fixed_window"
	DefaultRPM    int    // 请求数/分钟
	DefaultTPM    int    // Token数/分钟
	BurstMultiplier float64
}

// AlertConfig 告警配置
type AlertConfig struct {
	Enabled    bool
	Email      EmailConfig
	DingTalk   DingTalkConfig
	Feishu     FeishuConfig
}

// EmailConfig 邮件配置
type EmailConfig struct {
	Enabled  bool
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       []string
}

// DingTalkConfig 钉钉配置
type DingTalkConfig struct {
	Enabled bool
	WebHook string
	Secret  string
}

// FeishuConfig 飞书配置
type FeishuConfig struct {
	Enabled bool
	WebHook string
	Secret  string
}

// ProviderConfig Provider配置
type ProviderConfig struct {
	Name    string
	Type    string // "openai", "anthropic", "google", "custom"
	BaseURL string
	APIKey  string
	Models  []string
	Priority int
	Weight   float64
}

// LoadConfig 加载配置
func LoadConfig(path string) (*Config, error) {
	// 简化实现，实际应使用viper或类似库
	cfg := &Config{
		Server: ServerConfig{
			Host:         getEnv("GATEWAY_HOST", "0.0.0.0"),
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		Router: RouterConfig{
			Strategy:        "latency",
			Timeout:         30 * time.Second,
			MaxRetries:      3,
			RetryDelay:      1 * time.Second,
			HealthCheckInterval: 10 * time.Second,
		},
		RateLimit: RateLimitConfig{
			Enabled:       true,
			Algorithm:     "token_bucket",
			DefaultRPM:    60,
			DefaultTPM:    60000,
			BurstMultiplier: 1.5,
		},
		Alert: AlertConfig{
			Enabled: true,
			Email: EmailConfig{
				Enabled: false,
				Host:   getEnv("SMTP_HOST", "smtp.example.com"),
				Port:   587,
			},
			DingTalk: DingTalkConfig{
				Enabled: getEnv("DINGTALK_ENABLED", "false") == "true",
				WebHook: getEnv("DINGTALK_WEBHOOK", ""),
				Secret:  getEnv("DINGTALK_SECRET", ""),
			},
			Feishu: FeishuConfig{
				Enabled: getEnv("FEISHU_ENABLED", "false") == "true",
				WebHook: getEnv("FEISHU_WEBHOOK", ""),
				Secret:  getEnv("FEISHU_SECRET", ""),
			},
		},
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
