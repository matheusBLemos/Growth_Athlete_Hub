//go:build integration

// Teste de integração do cache Redis contra um servidor Redis REAL. Gated em
// TEST_REDIS_ADDR — pula quando a env não está configurada. Complementa o teste
// unitário com miniredis (cache_test.go), validando o comportamento real de
// Set/Get/Delete e expiração por TTL. Usa um prefixo de chave único por execução
// para não colidir com outros dados no servidor compartilhado.
package redis_test

import (
	"context"
	"os"
	"testing"
	"time"

	rediscache "github.com/Growth-Athlete-Hub/gah-server/internal/infra/cache/redis"
)

func liveCache(t *testing.T) (*rediscache.Cache, string) {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set; skipping live Redis integration test")
	}
	c, err := rediscache.New(rediscache.Config{Addr: addr})
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("ping live redis: %v", err)
	}

	// Prefixo único para isolar este teste de outros dados no Redis.
	prefix := "gahtest:" + time.Now().Format("150405.000000") + ":"
	return c, prefix
}

func TestIntegration_RedisCache_SetGetDelete(t *testing.T) {
	c, prefix := liveCache(t)
	ctx := context.Background()
	key := prefix + "k1"
	t.Cleanup(func() { _ = c.Delete(ctx, key) })

	// Miss inicial.
	if _, hit, err := c.Get(ctx, key); err != nil || hit {
		t.Fatalf("expected miss; hit=%v err=%v", hit, err)
	}

	// Set + Get hit.
	if err := c.Set(ctx, key, []byte("hello"), time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	val, hit, err := c.Get(ctx, key)
	if err != nil || !hit {
		t.Fatalf("expected hit; hit=%v err=%v", hit, err)
	}
	if string(val) != "hello" {
		t.Fatalf("value mismatch: %q", val)
	}

	// Delete.
	if err := c.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, hit, _ := c.Get(ctx, key); hit {
		t.Fatal("expected miss after delete")
	}
}

func TestIntegration_RedisCache_TTLExpiry(t *testing.T) {
	c, prefix := liveCache(t)
	ctx := context.Background()
	key := prefix + "ttl"
	t.Cleanup(func() { _ = c.Delete(ctx, key) })

	// TTL curto, expira de verdade no servidor real.
	if err := c.Set(ctx, key, []byte("v"), 1*time.Second); err != nil {
		t.Fatalf("set: %v", err)
	}
	if _, hit, _ := c.Get(ctx, key); !hit {
		t.Fatal("expected hit before TTL elapsed")
	}

	time.Sleep(1500 * time.Millisecond)

	if _, hit, err := c.Get(ctx, key); err != nil || hit {
		t.Fatalf("expected miss after TTL; hit=%v err=%v", hit, err)
	}
}
