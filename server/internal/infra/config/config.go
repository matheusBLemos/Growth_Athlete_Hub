package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server    ServerConfig    `toml:"server"`
	Database  DatabaseConfig  `toml:"database"`
	Auth      AuthConfig      `toml:"auth"`
	Messaging  MessagingConfig  `toml:"messaging"`
	Redis      RedisConfig      `toml:"redis"`
	Connectors ConnectorsConfig `toml:"connectors"`
}

// ConnectorsConfig agrupa a configuração dos conectores de provedores externos.
type ConnectorsConfig struct {
	Strava StravaConfig `toml:"strava"`
}

// StravaConfig carrega as credenciais e endpoints do conector Strava.
type StravaConfig struct {
	// ClientID é o client_id da aplicação Strava. Sobrescreva via STRAVA_CLIENT_ID.
	ClientID string `toml:"client_id"`
	// ClientSecret é o client_secret da aplicação Strava. Sobrescreva via STRAVA_CLIENT_SECRET.
	ClientSecret string `toml:"client_secret"`
	// RedirectURL é a URL de callback OAuth registrada na Strava. Sobrescreva via STRAVA_REDIRECT_URL.
	RedirectURL string `toml:"redirect_url"`
	// WebhookVerifyToken valida a subscription do webhook. Sobrescreva via STRAVA_WEBHOOK_VERIFY_TOKEN.
	WebhookVerifyToken string `toml:"webhook_verify_token"`
	// AuthURL sobrescreve o endpoint de autorização OAuth (vazio = padrão Strava).
	AuthURL string `toml:"auth_url"`
	// TokenURL sobrescreve o endpoint de token OAuth (vazio = padrão Strava).
	TokenURL string `toml:"token_url"`
	// APIBaseURL sobrescreve a base da API (vazio = padrão Strava).
	APIBaseURL string `toml:"api_base_url"`
}

type ServerConfig struct {
	Port         int      `toml:"port"`
	ReadTimeout  Duration `toml:"read_timeout"`
	WriteTimeout Duration `toml:"write_timeout"`
	IdleTimeout  Duration `toml:"idle_timeout"`
}

type AuthConfig struct {
	// JWTSecret assina os tokens JWT (HS256).
	JWTSecret string `toml:"jwt_secret"`
	// TokenTTL define o tempo de validade do token de acesso.
	TokenTTL Duration `toml:"token_ttl"`
	// PasswordPepper é um segredo da aplicação aplicado ao hash Argon2id da senha.
	PasswordPepper string `toml:"password_pepper"`
}

type MessagingConfig struct {
	// URL é a connection string AMQP do RabbitMQ. Sobrescreva via RABBITMQ_URL.
	URL string `toml:"url"`
	// Exchange é o nome do topic exchange onde os eventos são publicados.
	Exchange string `toml:"exchange"`
	// QueuePrefix prefixa o nome das filas declaradas pelos consumidores
	// (ex.: "gah" -> "gah.activity.registered").
	QueuePrefix string `toml:"queue_prefix"`
	// Prefetch é o limite de mensagens não confirmadas por consumidor (QoS).
	Prefetch int `toml:"prefetch"`
}

type RedisConfig struct {
	// Addr é o endereço host:porta do Redis. Sobrescreva via REDIS_ADDR.
	Addr string `toml:"addr"`
	// Password é a senha do Redis (vazio = sem auth). Sobrescreva via REDIS_PASSWORD.
	Password string `toml:"password"`
	// DB é o índice do banco lógico do Redis. Sobrescreva via REDIS_DB.
	DB int `toml:"db"`
	// MetricsTTL é o TTL das entradas de cache do caminho de leitura de métricas.
	MetricsTTL Duration `toml:"metrics_ttl"`
}

type DatabaseConfig struct {
	URL             string   `toml:"url"`
	MaxOpenConns    int      `toml:"max_open_conns"`
	MaxIdleConns    int      `toml:"max_idle_conns"`
	ConnMaxLifetime Duration `toml:"conn_max_lifetime"`
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}

func Load(path string) (*Config, error) {
	cfg := defaults()

	if path != "" {
		if _, err := os.Stat(path); err == nil {
			if _, err := toml.DecodeFile(path, cfg); err != nil {
				return nil, fmt.Errorf("parsing config file: %w", err)
			}
		}
	}

	applyEnvOverrides(cfg)

	return cfg, nil
}

func defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         8080,
			ReadTimeout:  Duration{15 * time.Second},
			WriteTimeout: Duration{15 * time.Second},
			IdleTimeout:  Duration{60 * time.Second},
		},
		Database: DatabaseConfig{
			URL:             "postgres://localhost:5432/gah?sslmode=disable",
			MaxOpenConns:    25,
			MaxIdleConns:    10,
			ConnMaxLifetime: Duration{5 * time.Minute},
		},
		Auth: AuthConfig{
			JWTSecret:      "change-me-in-production",
			TokenTTL:       Duration{24 * time.Hour},
			PasswordPepper: "",
		},
		Messaging: MessagingConfig{
			URL:         "amqp://gah:gah@localhost:5672/",
			Exchange:    "gah.events",
			QueuePrefix: "gah",
			Prefetch:    10,
		},
		Redis: RedisConfig{
			Addr:       "localhost:6379",
			Password:   "",
			DB:         0,
			MetricsTTL: Duration{5 * time.Minute},
		},
		Connectors: ConnectorsConfig{
			Strava: StravaConfig{
				ClientID:           "",
				ClientSecret:       "",
				RedirectURL:        "http://localhost:8080/api/v1/connectors/strava/callback",
				WebhookVerifyToken: "change-me-webhook-token",
			},
		},
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}

	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.Database.URL = v
	}

	if v := os.Getenv("SERVER_READ_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Server.ReadTimeout = Duration{d}
		}
	}

	if v := os.Getenv("SERVER_WRITE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Server.WriteTimeout = Duration{d}
		}
	}

	if v := os.Getenv("SERVER_IDLE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Server.IdleTimeout = Duration{d}
		}
	}

	if v := os.Getenv("DB_MAX_OPEN_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Database.MaxOpenConns = n
		}
	}

	if v := os.Getenv("DB_MAX_IDLE_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Database.MaxIdleConns = n
		}
	}

	if v := os.Getenv("DB_CONN_MAX_LIFETIME"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Database.ConnMaxLifetime = Duration{d}
		}
	}

	if v := os.Getenv("AUTH_JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}

	if v := os.Getenv("AUTH_TOKEN_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Auth.TokenTTL = Duration{d}
		}
	}

	if v := os.Getenv("AUTH_PASSWORD_PEPPER"); v != "" {
		cfg.Auth.PasswordPepper = v
	}

	if v := os.Getenv("RABBITMQ_URL"); v != "" {
		cfg.Messaging.URL = v
	}

	if v := os.Getenv("RABBITMQ_EXCHANGE"); v != "" {
		cfg.Messaging.Exchange = v
	}

	if v := os.Getenv("RABBITMQ_QUEUE_PREFIX"); v != "" {
		cfg.Messaging.QueuePrefix = v
	}

	if v := os.Getenv("RABBITMQ_PREFETCH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Messaging.Prefetch = n
		}
	}

	if v := os.Getenv("REDIS_ADDR"); v != "" {
		cfg.Redis.Addr = v
	}

	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}

	if v := os.Getenv("REDIS_DB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Redis.DB = n
		}
	}

	if v := os.Getenv("REDIS_METRICS_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Redis.MetricsTTL = Duration{d}
		}
	}

	if v := os.Getenv("STRAVA_CLIENT_ID"); v != "" {
		cfg.Connectors.Strava.ClientID = v
	}
	if v := os.Getenv("STRAVA_CLIENT_SECRET"); v != "" {
		cfg.Connectors.Strava.ClientSecret = v
	}
	if v := os.Getenv("STRAVA_REDIRECT_URL"); v != "" {
		cfg.Connectors.Strava.RedirectURL = v
	}
	if v := os.Getenv("STRAVA_WEBHOOK_VERIFY_TOKEN"); v != "" {
		cfg.Connectors.Strava.WebhookVerifyToken = v
	}
	if v := os.Getenv("STRAVA_AUTH_URL"); v != "" {
		cfg.Connectors.Strava.AuthURL = v
	}
	if v := os.Getenv("STRAVA_TOKEN_URL"); v != "" {
		cfg.Connectors.Strava.TokenURL = v
	}
	if v := os.Getenv("STRAVA_API_BASE_URL"); v != "" {
		cfg.Connectors.Strava.APIBaseURL = v
	}
}
