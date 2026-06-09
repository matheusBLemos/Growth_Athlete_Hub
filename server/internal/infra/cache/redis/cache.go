// Package redis implementa a porta port.Cache sobre o Redis usando o cliente
// github.com/redis/go-redis/v9. É a camada de cache que fica à frente do
// caminho de leitura de métricas (cache-aside na use case QueryMetrics).
package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

// Config reúne os parâmetros de conexão do Redis.
type Config struct {
	Addr     string
	Password string
	DB       int
}

// Cache é o adapter Redis que implementa port.Cache.
type Cache struct {
	client redis.UniversalClient
}

var _ port.Cache = (*Cache)(nil)

// New cria um Cache a partir da Config. Não pinga o servidor — use Ping para
// validar a conectividade no startup.
func New(cfg Config) (*Cache, error) {
	if cfg.Addr == "" {
		return nil, errors.New("redis: empty addr")
	}
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Instrumenta o cliente com tracing e métricas OpenTelemetry. Usa os
	// providers globais (no-op quando a telemetria está desabilitada).
	if err := redisotel.InstrumentTracing(client); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: instrument tracing: %w", err)
	}
	if err := redisotel.InstrumentMetrics(client); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: instrument metrics: %w", err)
	}

	return &Cache{client: client}, nil
}

// Get busca a chave. Retorna hit=false (sem erro) quando a chave não existe.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	val, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("redis get %q: %w", key, err)
	}
	return val, true, nil
}

// Set grava value sob key. TTL <= 0 grava sem expiração.
func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := c.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("redis set %q: %w", key, err)
	}
	return nil
}

// Delete remove as chaves informadas. Sem chaves é no-op.
func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("redis del: %w", err)
	}
	return nil
}

// Ping verifica a conectividade com o servidor Redis.
func (c *Cache) Ping(ctx context.Context) error {
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	return nil
}

// Close encerra a conexão com o Redis.
func (c *Cache) Close() error {
	return c.client.Close()
}
