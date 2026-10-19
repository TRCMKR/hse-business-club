package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/trcmkr/hse-business-club/back/internal/platform/config"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", config.EnvTest) // do not read .env
	t.Setenv("POSTGRES_DSN", "postgres://hbc:secret@localhost:5432/hbc?sslmode=disable")
	t.Setenv("JWT_SECRET", "dev-secret")
	t.Setenv("S3_ENDPOINT", "localhost:9000")
	t.Setenv("S3_ACCESS_KEY", "minioadmin")
	t.Setenv("S3_SECRET_KEY", "minioadmin")
}

func TestLoadDefaults(t *testing.T) {
	setRequired(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	checks := []struct {
		name      string
		got, want any
	}{
		{"HTTPAddr", cfg.Server.HTTPAddr, ":8080"},
		{"DebugAddr", cfg.Server.DebugAddr, ":8081"},
		{"GRPCAddr", cfg.Server.GRPCAddr, "127.0.0.1:8082"},
		{"CORSOrigins", strings.Join(cfg.Server.CORSOrigins, ","), "http://localhost:3000"},
		{"ShutdownTimeout", cfg.Server.ShutdownTimeout, 10 * time.Second},
		{"MaxConns", cfg.DB.MaxConns, int32(10)},
		{"JWTTTL", cfg.Auth.JWTTTL, 24 * time.Hour},
		{"Bucket", cfg.Blob.Bucket, "hbc-avatars"},
		{"UseSSL", cfg.Blob.UseSSL, false},
		{"Region", cfg.Blob.Region, "us-east-1"},
		{"LogLevel", cfg.LogLevel, "info"},
		{"OTLPEndpoint", cfg.Telemetry.OTLPEndpoint, ""},
		{"ServiceName", cfg.Telemetry.ServiceName, "hbc-back"},
		{"SamplerRatio", cfg.Telemetry.SamplerRatio, 1.0},
		{"SeedDemoData", cfg.Seed.DemoData, false},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestLoadOverrides(t *testing.T) {
	setRequired(t)
	t.Setenv("CORS_ORIGINS", "http://a.test,http://b.test")
	t.Setenv("JWT_TTL", "1h")
	t.Setenv("S3_USE_SSL", "true")
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0.25")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.Server.CORSOrigins; len(got) != 2 || got[1] != "http://b.test" {
		t.Errorf("CORSOrigins = %v", got)
	}
	if cfg.Auth.JWTTTL != time.Hour || !cfg.Blob.UseSSL || cfg.Telemetry.SamplerRatio != 0.25 {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}

func TestLoadRequired(t *testing.T) {
	for _, name := range []string{"POSTGRES_DSN", "JWT_SECRET", "S3_ENDPOINT", "S3_ACCESS_KEY", "S3_SECRET_KEY"} {
		t.Run(name, func(t *testing.T) {
			setRequired(t)
			t.Setenv(name, "")

			_, err := config.Load()
			if err == nil {
				t.Fatalf("Load() without %s must fail", name)
			}
			if !strings.Contains(err.Error(), name) {
				t.Fatalf("error %q must mention %s", err, name)
			}
		})
	}
}

func TestLoadInvalid(t *testing.T) {
	tests := map[string]map[string]string{
		"bad env":            {"APP_ENV": "staging"},
		"bad log level":      {"LOG_LEVEL": "verbose"},
		"endpoint scheme":    {"S3_ENDPOINT": "http://minio:9000"},
		"sampler range":      {"OTEL_TRACES_SAMPLER_ARG": "2"},
		"short prod secret":  {"APP_ENV": config.EnvProduction, "JWT_SECRET": "short"},
		"zero max conns":     {"POSTGRES_MAX_CONNS": "0"},
		"malformed duration": {"JWT_TTL": "forever"},
	}
	for name, vars := range tests {
		t.Run(name, func(t *testing.T) {
			setRequired(t)
			for k, v := range vars {
				t.Setenv(k, v)
			}
			if _, err := config.Load(); err == nil {
				t.Fatal("Load() must fail")
			}
		})
	}
}

func TestLoadProductionLongSecret(t *testing.T) {
	setRequired(t)
	t.Setenv("APP_ENV", config.EnvProduction)
	t.Setenv("JWT_SECRET", strings.Repeat("x", config.MinProductionSecretLen))

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.IsProduction() {
		t.Fatal("IsProduction() = false")
	}
}

func TestRedacted(t *testing.T) {
	setRequired(t)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	r := cfg.Redacted()
	const masked = "***"
	if r.Auth.JWTSecret != masked || r.Blob.SecretKey != masked || r.Blob.AccessKey != masked {
		t.Errorf("secrets not masked: %+v", r)
	}
	if strings.Contains(r.DB.DSN, "secret") || !strings.Contains(r.DB.DSN, "hbc") {
		t.Errorf("DSN = %q", r.DB.DSN)
	}
	if cfg.Auth.JWTSecret != "dev-secret" {
		t.Error("Redacted must not modify the original")
	}
}
