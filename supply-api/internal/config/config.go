package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	Token      TokenConfig
	Settlement SettlementConfig
	Audit      AuditConfig
}

// ServerConfig HTTP服务配置
type ServerConfig struct {
	Addr              string
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	DefaultSupplierID int64  // 默认供应商ID（仅用于开发/单供应商模式）
	StatementBaseURL  string // 账单PDF下载基础URL
}

// DatabaseConfig PostgreSQL配置
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
}

// TokenConfig Token运行时配置
type TokenConfig struct {
	SecretKey          string
	PublicKey          string // RSA公钥内容（用于RS256验证）
	Algorithm          string // 算法: HS256, HS384, HS512, RS256, RS384, RS512
	Issuer             string
	AccessTokenTTL     time.Duration
	RefreshTokenTTL    time.Duration
	RevocationCacheTTL time.Duration
}

// SettlementConfig 结算与提现能力配置
type SettlementConfig struct {
	WithdrawEnabled bool
}

// AuditConfig 审计配置
type AuditConfig struct {
	BufferSize    int
	FlushInterval time.Duration
	ExportTimeout time.Duration
}

// DSN 返回数据库连接字符串（包含明文密码，仅限内部使用）
func (d *DatabaseConfig) DSN() string {
	// Unix socket 连接（host 以 / 开头）
	if strings.HasPrefix(d.Host, "/") {
		return fmt.Sprintf("host=%s user=%s dbname=%s sslmode=disable",
			d.Host, d.User, d.Database)
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		d.User, d.Password, d.Host, d.Port, d.Database)
}

// SafeDSN 返回脱敏的数据库连接字符串（密码被替换为***），用于日志记录
// P2-05: 避免在日志中泄露数据库密码
func (d *DatabaseConfig) SafeDSN() string {
	// Unix socket 连接（host 以 / 开头）
	if strings.HasPrefix(d.Host, "/") {
		return fmt.Sprintf("host=%s user=%s dbname=%s sslmode=disable",
			d.Host, d.User, d.Database)
	}
	return fmt.Sprintf("postgres://%s:***@%s:%d/%s?sslmode=disable",
		d.User, d.Host, d.Port, d.Database)
}

// Addr 返回Redis地址
func (r *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// Load 加载配置
func Load(env string) (*Config, error) {
	return load(env, "")
}

// LoadFromPath 从指定路径加载配置
func LoadFromPath(env, configPath string) (*Config, error) {
	return load(env, configPath)
}

func load(env, configPath string) (*Config, error) {
	v := viper.New()

	// 设置环境变量前缀
	v.SetEnvPrefix("SUPPLY_API")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 默认配置
	setDefaults(v)

	// 加载配置文件
	if strings.TrimSpace(configPath) != "" {
		v.SetConfigFile(configPath)
	} else {
		configFile := fmt.Sprintf("config.%s.yaml", env)
		v.SetConfigName(configFile)
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
	}

	// 允许环境变量覆盖
	v.AutomaticEnv()

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		if strings.TrimSpace(configPath) != "" {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
		// 配置文件不存在时，使用环境变量
	}

	// 绑定环境变量
	bindEnvVars(v)

	var cfg Config

	// Server配置
	cfg.Server.Addr = v.GetString("server.addr")
	cfg.Server.ReadTimeout = v.GetDuration("server.read_timeout")
	cfg.Server.WriteTimeout = v.GetDuration("server.write_timeout")
	cfg.Server.IdleTimeout = v.GetDuration("server.idle_timeout")
	cfg.Server.ShutdownTimeout = v.GetDuration("server.shutdown_timeout")
	cfg.Server.DefaultSupplierID = v.GetInt64("server.default_supplier_id")
	cfg.Server.StatementBaseURL = v.GetString("server.statement_base_url")

	// Database配置
	cfg.Database.Host = v.GetString("database.host")
	cfg.Database.Port = v.GetInt("database.port")
	cfg.Database.User = v.GetString("database.user")
	cfg.Database.Password = v.GetString("database.password")
	cfg.Database.Database = v.GetString("database.database")
	cfg.Database.MaxOpenConns = v.GetInt("database.max_open_conns")
	cfg.Database.MaxIdleConns = v.GetInt("database.max_idle_conns")
	cfg.Database.ConnMaxLifetime = v.GetDuration("database.conn_max_lifetime")
	cfg.Database.ConnMaxIdleTime = v.GetDuration("database.conn_max_idle_time")

	// Redis配置
	cfg.Redis.Host = v.GetString("redis.host")
	cfg.Redis.Port = v.GetInt("redis.port")
	cfg.Redis.Password = v.GetString("redis.password")
	cfg.Redis.DB = v.GetInt("redis.db")
	cfg.Redis.PoolSize = v.GetInt("redis.pool_size")

	// Token配置
	cfg.Token.SecretKey = v.GetString("token.secret_key")
	cfg.Token.PublicKey = v.GetString("token.public_key")
	cfg.Token.Algorithm = v.GetString("token.algorithm")
	cfg.Token.Issuer = v.GetString("token.issuer")
	cfg.Token.AccessTokenTTL = v.GetDuration("token.access_token_ttl")
	cfg.Token.RefreshTokenTTL = v.GetDuration("token.refresh_token_ttl")
	cfg.Token.RevocationCacheTTL = v.GetDuration("token.revocation_cache_ttl")

	// Audit配置
	cfg.Audit.BufferSize = v.GetInt("audit.buffer_size")
	cfg.Audit.FlushInterval = v.GetDuration("audit.flush_interval")
	cfg.Audit.ExportTimeout = v.GetDuration("audit.export_timeout")

	// Settlement配置
	cfg.Settlement.WithdrawEnabled = v.GetBool("settlement.withdraw_enabled")

	if err := validateForEnv(env, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// setDefaults 设置默认值
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.addr", ":18082")
	v.SetDefault("server.read_timeout", 10*time.Second)
	v.SetDefault("server.write_timeout", 15*time.Second)
	v.SetDefault("server.idle_timeout", 30*time.Second)
	v.SetDefault("server.shutdown_timeout", 5*time.Second)
	v.SetDefault("server.default_supplier_id", 1)
	v.SetDefault("server.statement_base_url", "https://example.com/statements")

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "")
	v.SetDefault("database.database", "supply_db")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", 1*time.Hour)
	v.SetDefault("database.conn_max_idle_time", 10*time.Minute)

	// Redis defaults
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 10)

	// Token defaults
	v.SetDefault("token.issuer", "lijiaoqiao/supply-api")
	v.SetDefault("token.access_token_ttl", 1*time.Hour)
	v.SetDefault("token.refresh_token_ttl", 7*24*time.Hour)
	v.SetDefault("token.revocation_cache_ttl", 30*time.Second)
	v.SetDefault("token.algorithm", "HS256") // 默认HS256，可配置RS256

	// Settlement defaults
	v.SetDefault("settlement.withdraw_enabled", false)

	// Audit defaults
	v.SetDefault("audit.buffer_size", 1000)
	v.SetDefault("audit.flush_interval", 5*time.Second)
	v.SetDefault("audit.export_timeout", 30*time.Second)
}

// bindEnvVars 绑定环境变量
func bindEnvVars(v *viper.Viper) {
	_ = v.BindEnv("server.addr", "SUPPLY_API_ADDR")
	_ = v.BindEnv("server.read_timeout", "SUPPLY_API_READ_TIMEOUT")
	_ = v.BindEnv("server.write_timeout", "SUPPLY_API_WRITE_TIMEOUT")

	_ = v.BindEnv("database.host", "SUPPLY_DB_HOST")
	_ = v.BindEnv("database.port", "SUPPLY_DB_PORT")
	_ = v.BindEnv("database.user", "SUPPLY_DB_USER")
	_ = v.BindEnv("database.password", "SUPPLY_DB_PASSWORD")
	_ = v.BindEnv("database.database", "SUPPLY_DB_NAME")
	_ = v.BindEnv("database.max_open_conns", "SUPPLY_DB_MAX_OPEN_CONNS")
	_ = v.BindEnv("database.max_idle_conns", "SUPPLY_DB_MAX_IDLE_CONNS")

	_ = v.BindEnv("redis.host", "SUPPLY_REDIS_HOST")
	_ = v.BindEnv("redis.port", "SUPPLY_REDIS_PORT")
	_ = v.BindEnv("redis.password", "SUPPLY_REDIS_PASSWORD")
	_ = v.BindEnv("redis.db", "SUPPLY_REDIS_DB")

	_ = v.BindEnv("token.secret_key", "SUPPLY_TOKEN_SECRET_KEY")
	_ = v.BindEnv("token.public_key", "SUPPLY_TOKEN_PUBLIC_KEY")
	_ = v.BindEnv("token.algorithm", "SUPPLY_TOKEN_ALGORITHM")
	_ = v.BindEnv("token.issuer", "SUPPLY_TOKEN_ISSUER")
	_ = v.BindEnv("settlement.withdraw_enabled", "SUPPLY_SETTLEMENT_WITHDRAW_ENABLED")
}

// GetEnvInt 获取环境变量int值
func GetEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func validateForEnv(env string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	cfg.Token.Algorithm = strings.ToUpper(strings.TrimSpace(cfg.Token.Algorithm))
	if cfg.Token.Algorithm == "" {
		cfg.Token.Algorithm = "HS256"
	}

	if env != "prod" {
		return nil
	}

	if cfg.Server.DefaultSupplierID != 0 {
		return fmt.Errorf("invalid prod config: server.default_supplier_id must be 0 to disable static supplier fallback")
	}
	if strings.TrimSpace(cfg.Token.Issuer) == "" {
		return fmt.Errorf("invalid prod config: token.issuer is required")
	}
	if cfg.Settlement.WithdrawEnabled {
		return fmt.Errorf("invalid prod config: settlement.withdraw_enabled cannot be true until SMS integration is production-ready")
	}

	switch cfg.Token.Algorithm {
	case "HS256", "HS384", "HS512":
		if strings.TrimSpace(cfg.Token.SecretKey) == "" {
			return fmt.Errorf("invalid prod config: token.secret_key is required for %s", cfg.Token.Algorithm)
		}
	case "RS256", "RS384", "RS512":
		if strings.TrimSpace(cfg.Token.PublicKey) == "" {
			return fmt.Errorf("invalid prod config: token.public_key is required for %s", cfg.Token.Algorithm)
		}
	default:
		return fmt.Errorf("invalid prod config: unsupported token.algorithm %q", cfg.Token.Algorithm)
	}

	return nil
}

// GetEnvDuration 获取环境变量duration值
func GetEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}
