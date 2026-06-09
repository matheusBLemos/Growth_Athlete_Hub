package redis_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	rediscache "github.com/Growth-Athlete-Hub/gah-server/internal/infra/cache/redis"
)

func newTestCache(t *testing.T) (*rediscache.Cache, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	c, err := rediscache.New(rediscache.Config{Addr: mr.Addr()})
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c, mr
}

func TestCache_GetMiss(t *testing.T) {
	c, _ := newTestCache(t)

	val, hit, err := c.Get(context.Background(), "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hit {
		t.Fatal("expected miss, got hit")
	}
	if val != nil {
		t.Fatalf("expected nil value, got %q", val)
	}
}

func TestCache_SetThenGetHit(t *testing.T) {
	c, _ := newTestCache(t)

	if err := c.Set(context.Background(), "k1", []byte("v1"), time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}

	val, hit, err := c.Get(context.Background(), "k1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !hit {
		t.Fatal("expected hit, got miss")
	}
	if string(val) != "v1" {
		t.Fatalf("value = %q, want v1", val)
	}
}

func TestCache_SetTTLHonored(t *testing.T) {
	c, mr := newTestCache(t)

	if err := c.Set(context.Background(), "ttlkey", []byte("v"), 30*time.Second); err != nil {
		t.Fatalf("set: %v", err)
	}

	// A chave existe antes de expirar.
	if _, hit, _ := c.Get(context.Background(), "ttlkey"); !hit {
		t.Fatal("expected hit before TTL elapsed")
	}

	// Avança o relógio do miniredis além do TTL.
	mr.FastForward(31 * time.Second)

	_, hit, err := c.Get(context.Background(), "ttlkey")
	if err != nil {
		t.Fatalf("get after ttl: %v", err)
	}
	if hit {
		t.Fatal("expected miss after TTL elapsed")
	}
}

func TestCache_SetNoExpiry(t *testing.T) {
	c, mr := newTestCache(t)

	if err := c.Set(context.Background(), "persist", []byte("v"), 0); err != nil {
		t.Fatalf("set: %v", err)
	}
	if mr.TTL("persist") != 0 {
		t.Fatalf("expected no TTL, got %v", mr.TTL("persist"))
	}
}

func TestCache_Delete(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "a", []byte("1"), time.Minute)
	_ = c.Set(ctx, "b", []byte("2"), time.Minute)

	if err := c.Delete(ctx, "a", "b", "nonexistent"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, hit, _ := c.Get(ctx, "a"); hit {
		t.Fatal("expected a deleted")
	}
	if _, hit, _ := c.Get(ctx, "b"); hit {
		t.Fatal("expected b deleted")
	}
}

func TestCache_DeleteNoKeys(t *testing.T) {
	c, _ := newTestCache(t)
	if err := c.Delete(context.Background()); err != nil {
		t.Fatalf("delete with no keys should be a no-op, got %v", err)
	}
}

func TestCache_Ping(t *testing.T) {
	c, _ := newTestCache(t)
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
}
