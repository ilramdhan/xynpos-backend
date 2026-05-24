package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the root configuration for any XynPOS service.
type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Redis    RedisConfig
	NATS     NATSConfig
	JWT      JWTConfig
	Storage  StorageConfig
	Email    EmailConfig
	FCM      FCMConfig
	Tracer   TracerConfig
	Worker   WorkerConfig
}

type AppConfig struct {
	Env      string `mapstructure:"APP_ENV"`      // development | staging | production
	Name     string `mapstructure:"APP_NAME"`     // service name e.g. "auth-service"
	Port     string `mapstructure:"APP_PORT"`     // e.g. "8001"
	LogLevel string `mapstructure:"APP_LOG_LEVEL"` // debug | info | warn | error
}

type DatabaseConfig struct {
	URL          string `mapstructure:"DATABASE_URL"`
	MaxOpenConns int    `mapstructure:"DATABASE_MAX_CONNS"`
	MinOpenConns int    `mapstructure:"DATABASE_MIN_CONNS"`
	MaxIdleTime  time.Duration
}

type RedisConfig struct {
	URL      string `mapstructure:"REDIS_URL"`
	Password string `mapstructure:"REDIS_PASSWORD"`
	DB       int    `mapstructure:"REDIS_DB"`
}

type NATSConfig struct {
	URL string `mapstructure:"NATS_URL"`
}

type JWTConfig struct {
	AccessSecret      string        `mapstructure:"JWT_ACCESS_SECRET"`
	RefreshSecret     string        `mapstructure:"JWT_REFRESH_SECRET"`
	AccessExpiry      time.Duration `mapstructure:"JWT_ACCESS_EXPIRY"`
	RefreshExpiry     time.Duration `mapstructure:"JWT_REFRESH_EXPIRY"`
	Issuer            string        `mapstructure:"JWT_ISSUER"`
}

type StorageConfig struct {
	Provider        string `mapstructure:"STORAGE_PROVIDER"` // minio | r2 | s3
	Endpoint        string `mapstructure:"STORAGE_ENDPOINT"`
	AccessKeyID     string `mapstructure:"STORAGE_ACCESS_KEY_ID"`
	SecretAccessKey string `mapstructure:"STORAGE_SECRET_ACCESS_KEY"`
	Bucket          string `mapstructure:"STORAGE_BUCKET"`
	PublicURL       string `mapstructure:"STORAGE_PUBLIC_URL"`
	Region          string `mapstructure:"STORAGE_REGION"`
	UseSSL          bool   `mapstructure:"STORAGE_USE_SSL"`
}

type EmailConfig struct {
	Provider string `mapstructure:"EMAIL_PROVIDER"` // resend | mock
	APIKey   string `mapstructure:"RESEND_API_KEY"`
	From     string `mapstructure:"EMAIL_FROM"`
}

type FCMConfig struct {
	CredentialsJSON string `mapstructure:"FIREBASE_CREDENTIALS_JSON"`
	ProjectID       string `mapstructure:"FIREBASE_PROJECT_ID"`
}

type TracerConfig struct {
	Enabled     bool   `mapstructure:"TRACER_ENABLED"`
	JaegerURL   string `mapstructure:"JAEGER_OTLP_URL"` // e.g. "localhost:4317"
	ServiceName string // set programmatically, not from env
}

type WorkerConfig struct {
	RedisURL    string `mapstructure:"REDIS_URL"`
	Concurrency int    `mapstructure:"WORKER_CONCURRENCY"`
}

// Load reads configuration from environment variables and optional .env file.
// The serviceName is injected programmatically (not from env).
func Load(serviceName string) (*Config, error) {
	v := viper.New()

	// Allow reading from .env file if present (dev only)
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AddConfigPath("..")
	_ = v.ReadInConfig() // Ignore error — env file is optional

	// Environment variables take precedence
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Defaults
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("APP_PORT", "8080")
	v.SetDefault("APP_LOG_LEVEL", "info")
	v.SetDefault("DATABASE_MAX_CONNS", 25)
	v.SetDefault("DATABASE_MIN_CONNS", 5)
	v.SetDefault("REDIS_DB", 0)
	v.SetDefault("JWT_ACCESS_EXPIRY", "15m")
	v.SetDefault("JWT_REFRESH_EXPIRY", "720h")
	v.SetDefault("JWT_ISSUER", "xynpos.com")
	v.SetDefault("TRACER_ENABLED", true)
	v.SetDefault("WORKER_CONCURRENCY", 10)
	v.SetDefault("STORAGE_USE_SSL", true)
	v.SetDefault("STORAGE_REGION", "auto")

	cfg := &Config{}

	// Manually bind to handle duration parsing
	cfg.App = AppConfig{
		Env:      v.GetString("APP_ENV"),
		Name:     serviceName,
		Port:     v.GetString("APP_PORT"),
		LogLevel: v.GetString("APP_LOG_LEVEL"),
	}

	cfg.Database = DatabaseConfig{
		URL:          requireString(v, "DATABASE_URL"),
		MaxOpenConns: v.GetInt("DATABASE_MAX_CONNS"),
		MinOpenConns: v.GetInt("DATABASE_MIN_CONNS"),
		MaxIdleTime:  10 * time.Minute,
	}

	cfg.Redis = RedisConfig{
		URL:      v.GetString("REDIS_URL"),
		Password: v.GetString("REDIS_PASSWORD"),
		DB:       v.GetInt("REDIS_DB"),
	}

	cfg.NATS = NATSConfig{
		URL: v.GetString("NATS_URL"),
	}

	accessExpiry, err := time.ParseDuration(v.GetString("JWT_ACCESS_EXPIRY"))
	if err != nil {
		accessExpiry = 15 * time.Minute
	}
	refreshExpiry, err := time.ParseDuration(v.GetString("JWT_REFRESH_EXPIRY"))
	if err != nil {
		refreshExpiry = 720 * time.Hour
	}

	cfg.JWT = JWTConfig{
		AccessSecret:  requireString(v, "JWT_ACCESS_SECRET"),
		RefreshSecret: requireString(v, "JWT_REFRESH_SECRET"),
		AccessExpiry:  accessExpiry,
		RefreshExpiry: refreshExpiry,
		Issuer:        v.GetString("JWT_ISSUER"),
	}

	cfg.Storage = StorageConfig{
		Provider:        v.GetString("STORAGE_PROVIDER"),
		Endpoint:        v.GetString("STORAGE_ENDPOINT"),
		AccessKeyID:     v.GetString("STORAGE_ACCESS_KEY_ID"),
		SecretAccessKey: v.GetString("STORAGE_SECRET_ACCESS_KEY"),
		Bucket:          v.GetString("STORAGE_BUCKET"),
		PublicURL:       v.GetString("STORAGE_PUBLIC_URL"),
		Region:          v.GetString("STORAGE_REGION"),
		UseSSL:          v.GetBool("STORAGE_USE_SSL"),
	}

	cfg.Email = EmailConfig{
		Provider: v.GetString("EMAIL_PROVIDER"),
		APIKey:   v.GetString("RESEND_API_KEY"),
		From:     v.GetString("EMAIL_FROM"),
	}

	cfg.FCM = FCMConfig{
		CredentialsJSON: v.GetString("FIREBASE_CREDENTIALS_JSON"),
		ProjectID:       v.GetString("FIREBASE_PROJECT_ID"),
	}

	cfg.Tracer = TracerConfig{
		Enabled:     v.GetBool("TRACER_ENABLED"),
		JaegerURL:   v.GetString("JAEGER_OTLP_URL"),
		ServiceName: serviceName,
	}

	cfg.Worker = WorkerConfig{
		RedisURL:    v.GetString("REDIS_URL"),
		Concurrency: v.GetInt("WORKER_CONCURRENCY"),
	}

	return cfg, nil
}

// MustLoad calls Load and fatally panics if there is an error.
// Use in main() only.
func MustLoad(serviceName string) *Config {
	cfg, err := Load(serviceName)
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}
	return cfg
}

func requireString(v *viper.Viper, key string) string {
	val := v.GetString(key)
	if val == "" {
		panic(fmt.Sprintf("required config key %q is not set", key))
	}
	return val
}
