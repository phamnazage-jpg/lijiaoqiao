package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Encryption key should be provided via environment variable or secure key management
// In production, use a proper key management system (KMS)
// Must be 16, 24, or 32 bytes for AES-128, AES-192, or AES-256
var encryptionKey = []byte(getEnv("PASSWORD_ENCRYPTION_KEY", "default-key-32-bytes-long!!!!!!!"))

// Config 网关配置
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Auth      AuthConfig
	Router    RouterConfig
	RateLimit RateLimitConfig
	Alert     AlertConfig
	Providers []ProviderConfig
}

// ServerConfig 服务配置
type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// AuthConfig 鉴权运行时配置
type AuthConfig struct {
	Env               string
	TokenRuntimeMode  string
	TokenRuntimeURL   string
	TrustedProxies    []string // 可信的代理IP列表，用于IP伪造防护
	CORSAllowOrigins  []string // 允许的CORS来源，为空则使用默认通配符
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host              string
	Port              int
	User              string
	Password          string // 兼容旧版本，仍可直接使用明文密码（不推荐）
	EncryptedPassword string // 加密后的密码，优先级高于Password字段
	Database          string
	MaxConns          int
}

// GetPassword 返回解密后的数据库密码
// 优先使用EncryptedPassword，如果为空则返回Password字段（兼容旧版本）
func (c *DatabaseConfig) GetPassword() string {
	if c.EncryptedPassword != "" {
		decrypted, err := decryptPassword(c.EncryptedPassword)
		if err != nil {
			// 解密失败时返回原始加密字符串，让后续逻辑处理错误
			return c.EncryptedPassword
		}
		return decrypted
	}
	return c.Password
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host              string
	Port              int
	Password          string // 兼容旧版本
	EncryptedPassword string // 加密后的密码
	DB                int
	PoolSize          int
}

// GetPassword 返回解密后的Redis密码
func (c *RedisConfig) GetPassword() string {
	if c.EncryptedPassword != "" {
		decrypted, err := decryptPassword(c.EncryptedPassword)
		if err != nil {
			return c.EncryptedPassword
		}
		return decrypted
	}
	return c.Password
}

// RouterConfig 路由配置
type RouterConfig struct {
	Strategy            string // "latency", "cost", "availability", "weighted"
	Timeout             time.Duration
	MaxRetries          int
	RetryDelay          time.Duration
	HealthCheckInterval time.Duration
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled         bool
	Algorithm       string // "token_bucket", "sliding_window", "fixed_window"
	DefaultRPM      int    // 请求数/分钟
	DefaultTPM      int    // Token数/分钟
	BurstMultiplier float64
}

// AlertConfig 告警配置
type AlertConfig struct {
	Enabled  bool
	Email    EmailConfig
	DingTalk DingTalkConfig
	Feishu   FeishuConfig
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
	Name     string
	Type     string // "openai", "anthropic", "google", "custom"
	BaseURL  string
	APIKey   string
	Models   []string
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
		Auth: AuthConfig{
			Env:              strings.ToLower(getEnv("GATEWAY_ENV", "dev")),
			TokenRuntimeMode: strings.ToLower(getEnv("GATEWAY_TOKEN_RUNTIME_MODE", "inmemory")),
			TokenRuntimeURL:  strings.TrimSpace(getEnv("GATEWAY_TOKEN_RUNTIME_URL", "")),
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
				Host:    getEnv("SMTP_HOST", "smtp.example.com"),
				Port:    587,
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
		Providers: []ProviderConfig{
			{
				Name:     "openai",
				Type:     "openai",
				BaseURL:  strings.TrimSpace(getEnv("OPENAI_BASE_URL", "https://api.openai.com")),
				APIKey:   getEnv("OPENAI_API_KEY", ""),
				Models:   splitCSV(getEnv("OPENAI_MODELS", "gpt-4,gpt-3.5-turbo")),
				Priority: 1,
				Weight:   1.0,
			},
		},
	}

	if err := ValidateAuthConfig(cfg.Auth); err != nil {
		return nil, err
	}

	return cfg, nil
}

func ValidateAuthConfig(cfg AuthConfig) error {
	mode := strings.ToLower(strings.TrimSpace(cfg.TokenRuntimeMode))
	env := strings.ToLower(strings.TrimSpace(cfg.Env))

	switch mode {
	case "inmemory", "remote_introspection":
	default:
		return fmt.Errorf("unsupported token runtime mode %q", cfg.TokenRuntimeMode)
	}

	if (env == "prod" || env == "staging") && mode == "inmemory" {
		return fmt.Errorf("inmemory token runtime is not allowed in %s, use remote_introspection", env)
	}
	if mode == "remote_introspection" && strings.TrimSpace(cfg.TokenRuntimeURL) == "" {
		return errors.New("GATEWAY_TOKEN_RUNTIME_URL is required when token runtime mode is remote_introspection")
	}

	return nil
}

func splitCSV(value string) []string {
	rawParts := strings.Split(value, ",")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// encryptPassword 使用AES-GCM加密密码
func encryptPassword(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptPassword 解密密码
func decryptPassword(encrypted string) (string, error) {
	if encrypted == "" {
		return "", nil
	}

	// 检查是否是旧格式（未加密的明文）
	if len(encrypted) < 4 || encrypted[:4] != "enc:" {
		// 尝试作为新格式解密
		ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
		if err != nil {
			// 如果不是有效的base64，可能是旧格式明文，直接返回
			return encrypted, nil
		}

		block, err := aes.NewCipher(encryptionKey)
		if err != nil {
			return "", err
		}

		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return "", err
		}

		nonceSize := gcm.NonceSize()
		if len(ciphertext) < nonceSize {
			return "", errors.New("ciphertext too short")
		}

		nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
		plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return "", err
		}

		return string(plaintext), nil
	}

	// 旧格式：直接返回"enc:"后的部分
	return encrypted[4:], nil
}
