package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig   `mapstructure:"server" yaml:"server"`
	Database DatabaseConfig `mapstructure:"database" yaml:"database"`
	Redis    RedisConfig    `mapstructure:"redis" yaml:"redis"`
	Logging  LoggingConfig  `mapstructure:"logging" yaml:"logging"`
	Auth     AuthConfig     `mapstructure:"auth" yaml:"auth"`
	Metrics  MetricsConfig  `mapstructure:"metrics" yaml:"metrics"`
	Limits   LimitsConfig   `mapstructure:"limits" yaml:"limits"`
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	Host            string          `mapstructure:"host" yaml:"host"`
	Port            int             `mapstructure:"port" yaml:"port"`
	Mode            string          `mapstructure:"mode" yaml:"mode"`
	ReadTimeout     time.Duration   `mapstructure:"read_timeout" yaml:"read_timeout"`
	WriteTimeout    time.Duration   `mapstructure:"write_timeout" yaml:"write_timeout"`
	ShutdownTimeout time.Duration   `mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"`
	CORS            CORSConfig      `mapstructure:"cors" yaml:"cors"`
	RateLimit       RateLimitConfig `mapstructure:"rate_limit" yaml:"rate_limit"`
}

// CORSConfig contains CORS configuration
type CORSConfig struct {
	AllowOrigins     []string `mapstructure:"allow_origins" yaml:"allow_origins"`
	AllowMethods     []string `mapstructure:"allow_methods" yaml:"allow_methods"`
	AllowHeaders     []string `mapstructure:"allow_headers" yaml:"allow_headers"`
	ExposeHeaders    []string `mapstructure:"expose_headers" yaml:"expose_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials" yaml:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age" yaml:"max_age"`
}

// RateLimitConfig contains rate limiting configuration
type RateLimitConfig struct {
	Enabled bool          `mapstructure:"enabled" yaml:"enabled"`
	RPS     float64       `mapstructure:"rps" yaml:"rps"`
	Burst   int           `mapstructure:"burst" yaml:"burst"`
	Cleanup time.Duration `mapstructure:"cleanup" yaml:"cleanup"`
}

// DatabaseConfig contains database connection configuration
type DatabaseConfig struct {
	Host            string        `mapstructure:"host" yaml:"host"`
	Port            int           `mapstructure:"port" yaml:"port"`
	User            string        `mapstructure:"user" yaml:"user"`
	Password        string        `mapstructure:"password" yaml:"password"`
	Database        string        `mapstructure:"database" yaml:"database"`
	SSLMode         string        `mapstructure:"ssl_mode" yaml:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns" yaml:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns" yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime" yaml:"conn_max_lifetime"`
	MigrationsPath  string        `mapstructure:"migrations_path" yaml:"migrations_path"`
}

// DSN returns the database connection string
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Database, d.SSLMode)
}

// RedisConfig contains Redis connection configuration
type RedisConfig struct {
	Host     string        `mapstructure:"host" yaml:"host"`
	Port     int           `mapstructure:"port" yaml:"port"`
	Password string        `mapstructure:"password" yaml:"password"`
	DB       int           `mapstructure:"db" yaml:"db"`
	Timeout  time.Duration `mapstructure:"timeout" yaml:"timeout"`
	Enabled  bool          `mapstructure:"enabled" yaml:"enabled"`
}

// Address returns the Redis address
func (r RedisConfig) Address() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level      string `mapstructure:"level" yaml:"level"`
	Format     string `mapstructure:"format" yaml:"format"`
	Output     string `mapstructure:"output" yaml:"output"`
	Filename   string `mapstructure:"filename" yaml:"filename"`
	MaxSize    int    `mapstructure:"max_size" yaml:"max_size"`
	MaxBackups int    `mapstructure:"max_backups" yaml:"max_backups"`
	MaxAge     int    `mapstructure:"max_age" yaml:"max_age"`
	Compress   bool   `mapstructure:"compress" yaml:"compress"`
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	JWTSecret     string        `mapstructure:"jwt_secret" yaml:"jwt_secret"`
	JWTExpiration time.Duration `mapstructure:"jwt_expiration" yaml:"jwt_expiration"`
	APIKeyHeader  string        `mapstructure:"api_key_header" yaml:"api_key_header"`
	APIKeys       []string      `mapstructure:"api_keys" yaml:"api_keys"`
	Enabled       bool          `mapstructure:"enabled" yaml:"enabled"`
}

// MetricsConfig contains metrics configuration
type MetricsConfig struct {
	Enabled   bool   `mapstructure:"enabled" yaml:"enabled"`
	Path      string `mapstructure:"path" yaml:"path"`
	Namespace string `mapstructure:"namespace" yaml:"namespace"`
	Subsystem string `mapstructure:"subsystem" yaml:"subsystem"`
}

// LimitsConfig contains resource limits configuration
type LimitsConfig struct {
	MaxCPUCores int `mapstructure:"max_cpu_cores" yaml:"max_cpu_cores"`
	MaxRAMMB    int `mapstructure:"max_ram_mb" yaml:"max_ram_mb"`
	MaxDiskGB   int `mapstructure:"max_disk_gb" yaml:"max_disk_gb"`
	MaxVMs      int `mapstructure:"max_vms" yaml:"max_vms"`
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	// Set defaults
	setDefaults()

	// Configure Viper
	viper.SetConfigType("yaml")
	viper.SetConfigName("config")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("./")
	viper.AddConfigPath("/etc/vm-manager/")

	if configPath != "" {
		viper.SetConfigFile(configPath)
	}

	// Enable environment variable support
	viper.AutomaticEnv()
	viper.SetEnvPrefix("VM_MANAGER")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read configuration file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal configuration
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.shutdown_timeout", "5s")

	// CORS defaults
	viper.SetDefault("server.cors.allow_origins", []string{"*"})
	viper.SetDefault("server.cors.allow_methods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	viper.SetDefault("server.cors.allow_headers", []string{"Content-Type", "Authorization", "X-Request-ID"})
	viper.SetDefault("server.cors.allow_credentials", false)
	viper.SetDefault("server.cors.max_age", 3600)

	// Rate limit defaults
	viper.SetDefault("server.rate_limit.enabled", true)
	viper.SetDefault("server.rate_limit.rps", 100.0)
	viper.SetDefault("server.rate_limit.burst", 200)
	viper.SetDefault("server.rate_limit.cleanup", "1m")

	// Database defaults
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "vmmanager")
	viper.SetDefault("database.password", "password123")
	viper.SetDefault("database.database", "vmmanager")
	viper.SetDefault("database.ssl_mode", "disable")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_lifetime", "1h")
	viper.SetDefault("database.migrations_path", "./internal/database/migrations")

	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.timeout", "5s")
	viper.SetDefault("redis.enabled", false)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")
	viper.SetDefault("logging.max_size", 100)
	viper.SetDefault("logging.max_backups", 3)
	viper.SetDefault("logging.max_age", 7)
	viper.SetDefault("logging.compress", true)

	// Auth defaults
	viper.SetDefault("auth.jwt_secret", "default-secret-change-in-production")
	viper.SetDefault("auth.jwt_expiration", "24h")
	viper.SetDefault("auth.api_key_header", "X-API-Key")
	viper.SetDefault("auth.enabled", false)

	// Metrics defaults
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.path", "/metrics")
	viper.SetDefault("metrics.namespace", "vm_manager")
	viper.SetDefault("metrics.subsystem", "api")

	// Limits defaults
	viper.SetDefault("limits.max_cpu_cores", 64)
	viper.SetDefault("limits.max_ram_mb", 262144)
	viper.SetDefault("limits.max_disk_gb", 10240)
	viper.SetDefault("limits.max_vms", 1000)
}

// validateConfig validates the configuration
func validateConfig(cfg *Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	if cfg.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}

	if cfg.Database.Database == "" {
		return fmt.Errorf("database name is required")
	}

	if cfg.Auth.Enabled && cfg.Auth.JWTSecret == "default-secret-change-in-production" {
		return fmt.Errorf("jwt secret must be changed in production")
	}

	return nil
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Server.Mode == "debug" || c.Server.Mode == "development"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Server.Mode == "release" || c.Server.Mode == "production"
}

// Address returns the server listen address
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
