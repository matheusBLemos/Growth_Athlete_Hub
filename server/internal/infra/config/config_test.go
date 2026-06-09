package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/config"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout.Duration != 15*time.Second {
		t.Errorf("read_timeout = %v, want 15s", cfg.Server.ReadTimeout)
	}
	if cfg.Database.URL != "postgres://localhost:5432/gah?sslmode=disable" {
		t.Errorf("database url = %q, want default", cfg.Database.URL)
	}
	if cfg.Database.MaxOpenConns != 25 {
		t.Errorf("max_open_conns = %d, want 25", cfg.Database.MaxOpenConns)
	}
	if cfg.Auth.JWTSecret != "change-me-in-production" {
		t.Errorf("jwt_secret = %q, want default", cfg.Auth.JWTSecret)
	}
	if cfg.Auth.TokenTTL.Duration != 24*time.Hour {
		t.Errorf("token_ttl = %v, want 24h", cfg.Auth.TokenTTL)
	}
	if cfg.Messaging.URL != "amqp://gah:gah@localhost:5672/" {
		t.Errorf("messaging url = %q, want default", cfg.Messaging.URL)
	}
	if cfg.Messaging.Exchange != "gah.events" {
		t.Errorf("messaging exchange = %q, want gah.events", cfg.Messaging.Exchange)
	}
	if cfg.Messaging.QueuePrefix != "gah" {
		t.Errorf("messaging queue_prefix = %q, want gah", cfg.Messaging.QueuePrefix)
	}
	if cfg.Messaging.Prefetch != 10 {
		t.Errorf("messaging prefetch = %d, want 10", cfg.Messaging.Prefetch)
	}
	if cfg.Redis.Addr != "localhost:6379" {
		t.Errorf("redis addr = %q, want localhost:6379", cfg.Redis.Addr)
	}
	if cfg.Redis.DB != 0 {
		t.Errorf("redis db = %d, want 0", cfg.Redis.DB)
	}
	if cfg.Redis.MetricsTTL.Duration != 5*time.Minute {
		t.Errorf("redis metrics_ttl = %v, want 5m", cfg.Redis.MetricsTTL)
	}
}

func TestLoad_RedisFromTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[redis]
addr = "redis-host:6380"
password = "toml-pass"
db = 3
metrics_ttl = "90s"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Redis.Addr != "redis-host:6380" {
		t.Errorf("redis addr = %q, want redis-host:6380", cfg.Redis.Addr)
	}
	if cfg.Redis.Password != "toml-pass" {
		t.Errorf("redis password = %q, want toml-pass", cfg.Redis.Password)
	}
	if cfg.Redis.DB != 3 {
		t.Errorf("redis db = %d, want 3", cfg.Redis.DB)
	}
	if cfg.Redis.MetricsTTL.Duration != 90*time.Second {
		t.Errorf("redis metrics_ttl = %v, want 90s", cfg.Redis.MetricsTTL)
	}
}

func TestLoad_RedisEnvOverrides(t *testing.T) {
	t.Setenv("REDIS_ADDR", "env-redis:6390")
	t.Setenv("REDIS_PASSWORD", "env-pass")
	t.Setenv("REDIS_DB", "7")
	t.Setenv("REDIS_METRICS_TTL", "2m")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Redis.Addr != "env-redis:6390" {
		t.Errorf("redis addr = %q, want env-redis:6390", cfg.Redis.Addr)
	}
	if cfg.Redis.Password != "env-pass" {
		t.Errorf("redis password = %q, want env-pass", cfg.Redis.Password)
	}
	if cfg.Redis.DB != 7 {
		t.Errorf("redis db = %d, want 7", cfg.Redis.DB)
	}
	if cfg.Redis.MetricsTTL.Duration != 2*time.Minute {
		t.Errorf("redis metrics_ttl = %v, want 2m", cfg.Redis.MetricsTTL)
	}
}

func TestLoad_MessagingFromTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[messaging]
url = "amqp://user:pass@broker:5672/"
exchange = "custom.events"
queue_prefix = "custom"
prefetch = 42
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Messaging.URL != "amqp://user:pass@broker:5672/" {
		t.Errorf("messaging url = %q, want toml value", cfg.Messaging.URL)
	}
	if cfg.Messaging.Exchange != "custom.events" {
		t.Errorf("messaging exchange = %q, want custom.events", cfg.Messaging.Exchange)
	}
	if cfg.Messaging.QueuePrefix != "custom" {
		t.Errorf("messaging queue_prefix = %q, want custom", cfg.Messaging.QueuePrefix)
	}
	if cfg.Messaging.Prefetch != 42 {
		t.Errorf("messaging prefetch = %d, want 42", cfg.Messaging.Prefetch)
	}
}

func TestLoad_MessagingEnvOverrides(t *testing.T) {
	t.Setenv("RABBITMQ_URL", "amqp://env:env@envhost:5672/")
	t.Setenv("RABBITMQ_EXCHANGE", "env.events")
	t.Setenv("RABBITMQ_QUEUE_PREFIX", "envprefix")
	t.Setenv("RABBITMQ_PREFETCH", "5")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Messaging.URL != "amqp://env:env@envhost:5672/" {
		t.Errorf("messaging url = %q, want env override", cfg.Messaging.URL)
	}
	if cfg.Messaging.Exchange != "env.events" {
		t.Errorf("messaging exchange = %q, want env.events", cfg.Messaging.Exchange)
	}
	if cfg.Messaging.QueuePrefix != "envprefix" {
		t.Errorf("messaging queue_prefix = %q, want envprefix", cfg.Messaging.QueuePrefix)
	}
	if cfg.Messaging.Prefetch != 5 {
		t.Errorf("messaging prefetch = %d, want 5", cfg.Messaging.Prefetch)
	}
}

func TestLoad_AuthFromTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[auth]
jwt_secret = "toml-secret"
token_ttl = "2h"
password_pepper = "toml-pepper"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Auth.JWTSecret != "toml-secret" {
		t.Errorf("jwt_secret = %q, want toml-secret", cfg.Auth.JWTSecret)
	}
	if cfg.Auth.TokenTTL.Duration != 2*time.Hour {
		t.Errorf("token_ttl = %v, want 2h", cfg.Auth.TokenTTL)
	}
	if cfg.Auth.PasswordPepper != "toml-pepper" {
		t.Errorf("password_pepper = %q, want toml-pepper", cfg.Auth.PasswordPepper)
	}
}

func TestLoad_AuthEnvOverrides(t *testing.T) {
	t.Setenv("AUTH_JWT_SECRET", "env-secret")
	t.Setenv("AUTH_TOKEN_TTL", "30m")
	t.Setenv("AUTH_PASSWORD_PEPPER", "env-pepper")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Auth.JWTSecret != "env-secret" {
		t.Errorf("jwt_secret = %q, want env-secret", cfg.Auth.JWTSecret)
	}
	if cfg.Auth.TokenTTL.Duration != 30*time.Minute {
		t.Errorf("token_ttl = %v, want 30m", cfg.Auth.TokenTTL)
	}
	if cfg.Auth.PasswordPepper != "env-pepper" {
		t.Errorf("password_pepper = %q, want env-pepper", cfg.Auth.PasswordPepper)
	}
}

func TestLoad_FromTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[server]
port = 9090
read_timeout = "30s"
write_timeout = "30s"
idle_timeout = "120s"

[database]
url = "postgres://db:5432/test?sslmode=require"
max_open_conns = 50
max_idle_conns = 20
conn_max_lifetime = "10m"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout.Duration != 30*time.Second {
		t.Errorf("read_timeout = %v, want 30s", cfg.Server.ReadTimeout)
	}
	if cfg.Server.IdleTimeout.Duration != 120*time.Second {
		t.Errorf("idle_timeout = %v, want 120s", cfg.Server.IdleTimeout)
	}
	if cfg.Database.URL != "postgres://db:5432/test?sslmode=require" {
		t.Errorf("database url = %q", cfg.Database.URL)
	}
	if cfg.Database.MaxOpenConns != 50 {
		t.Errorf("max_open_conns = %d, want 50", cfg.Database.MaxOpenConns)
	}
	if cfg.Database.ConnMaxLifetime.Duration != 10*time.Minute {
		t.Errorf("conn_max_lifetime = %v, want 10m", cfg.Database.ConnMaxLifetime)
	}
}

func TestLoad_EnvOverridesToml(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[server]
port = 9090

[database]
url = "postgres://toml-host:5432/db"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	t.Setenv("PORT", "3000")
	t.Setenv("DATABASE_URL", "postgres://env-host:5432/db")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 3000 {
		t.Errorf("port = %d, want 3000 (env override)", cfg.Server.Port)
	}
	if cfg.Database.URL != "postgres://env-host:5432/db" {
		t.Errorf("database url = %q, want env override", cfg.Database.URL)
	}
}

func TestLoad_EnvOverridesDefaults(t *testing.T) {
	t.Setenv("PORT", "4000")
	t.Setenv("DATABASE_URL", "postgres://ci:5432/test")
	t.Setenv("SERVER_READ_TIMEOUT", "5s")
	t.Setenv("DB_MAX_OPEN_CONNS", "10")
	t.Setenv("DB_CONN_MAX_LIFETIME", "1m")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 4000 {
		t.Errorf("port = %d, want 4000", cfg.Server.Port)
	}
	if cfg.Database.URL != "postgres://ci:5432/test" {
		t.Errorf("database url = %q", cfg.Database.URL)
	}
	if cfg.Server.ReadTimeout.Duration != 5*time.Second {
		t.Errorf("read_timeout = %v, want 5s", cfg.Server.ReadTimeout)
	}
	if cfg.Database.MaxOpenConns != 10 {
		t.Errorf("max_open_conns = %d, want 10", cfg.Database.MaxOpenConns)
	}
	if cfg.Database.ConnMaxLifetime.Duration != time.Minute {
		t.Errorf("conn_max_lifetime = %v, want 1m", cfg.Database.ConnMaxLifetime)
	}
}

func TestLoad_MissingFileUsesDefaults(t *testing.T) {
	cfg, err := config.Load("/nonexistent/config.toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("port = %d, want 8080", cfg.Server.Port)
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if err := os.WriteFile(path, []byte("invalid [[[toml"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestLoad_InvalidEnvIgnored(t *testing.T) {
	t.Setenv("PORT", "not-a-number")
	t.Setenv("DB_MAX_OPEN_CONNS", "abc")
	t.Setenv("SERVER_READ_TIMEOUT", "invalid")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("port = %d, want 8080 (default when env invalid)", cfg.Server.Port)
	}
	if cfg.Database.MaxOpenConns != 25 {
		t.Errorf("max_open_conns = %d, want 25", cfg.Database.MaxOpenConns)
	}
	if cfg.Server.ReadTimeout.Duration != 15*time.Second {
		t.Errorf("read_timeout = %v, want 15s", cfg.Server.ReadTimeout)
	}
}
