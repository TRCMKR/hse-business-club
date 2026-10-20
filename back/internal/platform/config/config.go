// Package config loads application configuration from environment variables
// (ARCHITECTURE §5). In APP_ENV=local a .env file is read first, if present.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Supported values of APP_ENV.
const (
	EnvLocal      = "local"
	EnvTest       = "test"
	EnvProduction = "production"
)

// MinProductionSecretLen is the minimum JWT_SECRET length in production.
const MinProductionSecretLen = 32

const redacted = "***"

// Config is the full application configuration.
type Config struct {
	Env       string `env:"APP_ENV" envDefault:"local"`
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	Server    Server
	DB        DB
	Auth      Auth
	Blob      Blob
	Telemetry Telemetry
	Seed      Seed
}

// Server holds listener addresses and HTTP settings.
type Server struct {
	HTTPAddr        string        `env:"HTTP_ADDR" envDefault:":8080"`
	DebugAddr       string        `env:"DEBUG_ADDR" envDefault:":8081"`
	GRPCAddr        string        `env:"GRPC_ADDR" envDefault:"127.0.0.1:8082"`
	CORSOrigins     []string      `env:"CORS_ORIGINS" envDefault:"http://localhost:3000" envSeparator:","`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"10s"`
}

// DB holds PostgreSQL settings.
type DB struct {
	DSN      string `env:"POSTGRES_DSN,required,notEmpty"`
	MaxConns int32  `env:"POSTGRES_MAX_CONNS" envDefault:"10"`
}

// Auth holds token settings: the access JWT and the opaque refresh token.
type Auth struct {
	JWTSecret  string        `env:"JWT_SECRET,required,notEmpty"`
	AccessTTL  time.Duration `env:"ACCESS_TTL" envDefault:"15m"`
	RefreshTTL time.Duration `env:"REFRESH_TTL" envDefault:"720h"`
}

// Blob holds S3-compatible object storage settings.
type Blob struct {
	Endpoint  string `env:"S3_ENDPOINT,required,notEmpty"` // host:port without scheme
	AccessKey string `env:"S3_ACCESS_KEY,required,notEmpty"`
	SecretKey string `env:"S3_SECRET_KEY,required,notEmpty"`
	Bucket    string `env:"S3_BUCKET" envDefault:"hbc-avatars"`
	UseSSL    bool   `env:"S3_USE_SSL" envDefault:"false"`
	Region    string `env:"S3_REGION" envDefault:"us-east-1"`
}

// Telemetry holds OpenTelemetry settings.
type Telemetry struct {
	OTLPEndpoint string  `env:"OTEL_EXPORTER_OTLP_ENDPOINT"` // empty = do not export
	ServiceName  string  `env:"OTEL_SERVICE_NAME" envDefault:"hbc-back"`
	SamplerRatio float64 `env:"OTEL_TRACES_SAMPLER_ARG" envDefault:"1.0"`
}

// Seed holds demo data settings (SPEC §6).
type Seed struct {
	DemoData bool `env:"SEED_DEMO_DATA" envDefault:"false"`
}

// Load reads configuration from the environment and validates it.
// When APP_ENV is local (the default), variables from ./.env are loaded first;
// variables already set in the environment take precedence.
func Load() (*Config, error) {
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" || appEnv == EnvLocal {
		if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("config: load .env: %w", err)
		}
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate checks values that env tags cannot express.
func (c *Config) Validate() error {
	var errs []error
	switch c.Env {
	case EnvLocal, EnvTest, EnvProduction:
	default:
		errs = append(errs, fmt.Errorf("APP_ENV must be one of %s, %s, %s, got %q", EnvLocal, EnvTest, EnvProduction, c.Env))
	}
	switch strings.ToLower(c.LogLevel) {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error, got %q", c.LogLevel))
	}
	if c.DB.MaxConns < 1 {
		errs = append(errs, fmt.Errorf("POSTGRES_MAX_CONNS must be >= 1, got %d", c.DB.MaxConns))
	}
	if c.Auth.AccessTTL <= 0 {
		errs = append(errs, fmt.Errorf("ACCESS_TTL must be positive, got %s", c.Auth.AccessTTL))
	}
	if c.Auth.RefreshTTL <= 0 {
		errs = append(errs, fmt.Errorf("REFRESH_TTL must be positive, got %s", c.Auth.RefreshTTL))
	}
	if c.Server.ShutdownTimeout <= 0 {
		errs = append(errs, fmt.Errorf("SHUTDOWN_TIMEOUT must be positive, got %s", c.Server.ShutdownTimeout))
	}
	if c.Telemetry.SamplerRatio < 0 || c.Telemetry.SamplerRatio > 1 {
		errs = append(errs, fmt.Errorf("OTEL_TRACES_SAMPLER_ARG must be in [0, 1], got %v", c.Telemetry.SamplerRatio))
	}
	if strings.Contains(c.Blob.Endpoint, "://") {
		errs = append(errs, fmt.Errorf("S3_ENDPOINT must be host:port without scheme, got %q", c.Blob.Endpoint))
	}
	if c.Blob.Bucket == "" {
		errs = append(errs, errors.New("S3_BUCKET must not be empty"))
	}
	if c.Env == EnvProduction && len(c.Auth.JWTSecret) < MinProductionSecretLen {
		errs = append(errs, fmt.Errorf("JWT_SECRET must be at least %d bytes in production", MinProductionSecretLen))
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return nil
}

// IsProduction reports whether APP_ENV is production.
func (c *Config) IsProduction() bool {
	return c.Env == EnvProduction
}

// Redacted returns a copy of the config safe for logging: secrets are masked
// and the password in POSTGRES_DSN is removed.
func (c Config) Redacted() Config {
	out := c
	out.Server.CORSOrigins = append([]string(nil), c.Server.CORSOrigins...)
	out.Auth.JWTSecret = mask(c.Auth.JWTSecret)
	out.Blob.AccessKey = mask(c.Blob.AccessKey)
	out.Blob.SecretKey = mask(c.Blob.SecretKey)
	out.DB.DSN = redactDSN(c.DB.DSN)
	return out
}

func mask(s string) string {
	if s == "" {
		return ""
	}
	return redacted
}

func redactDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme == "" {
		// key=value DSN: hide the whole thing rather than parse it.
		return mask(dsn)
	}
	if _, ok := u.User.Password(); ok {
		u.User = url.UserPassword(u.User.Username(), redacted)
	}
	return u.String()
}
