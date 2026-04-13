package store

import (
	"context"
	"strings"
	"testing"
	"time"
)

func newTestMemoryCache(t *testing.T) *MemoryCache {
	t.Helper()
	c := NewMemoryCache()
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestMemoryCache_SetAndGet(t *testing.T) {
	c := newTestMemoryCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "k1", "v1", time.Minute); err != nil {
		t.Fatal(err)
	}
	v, err := c.Get(ctx, "k1")
	if err != nil {
		t.Fatal(err)
	}
	if v != "v1" {
		t.Errorf("expected v1, got %q", v)
	}
}

func TestMemoryCache_GetMiss(t *testing.T) {
	c := newTestMemoryCache(t)
	_, err := c.Get(context.Background(), "nonexistent")
	if err != ErrCacheMiss {
		t.Errorf("expected ErrCacheMiss, got %v", err)
	}
}

func TestMemoryCache_TTLExpiry(t *testing.T) {
	c := newTestMemoryCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "k", "v", time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	_, err := c.Get(ctx, "k")
	if err != ErrCacheMiss {
		t.Errorf("expected ErrCacheMiss after expiry, got %v", err)
	}
}

func TestMemoryCache_CleanupExpired(t *testing.T) {
	c := newTestMemoryCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "stale", "v", time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	c.cleanupExpired()

	if _, err := c.Get(ctx, "stale"); err != ErrCacheMiss {
		t.Fatalf("expected ErrCacheMiss after cleanupExpired, got %v", err)
	}
}

func TestMemoryCache_Del(t *testing.T) {
	c := newTestMemoryCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "k", "v", time.Minute)
	if err := c.Del(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	_, err := c.Get(ctx, "k")
	if err != ErrCacheMiss {
		t.Errorf("expected ErrCacheMiss after Del, got %v", err)
	}
}

func TestMemoryCache_SetNX(t *testing.T) {
	c := newTestMemoryCache(t)
	ctx := context.Background()

	ok, err := c.SetNX(ctx, "k", "v1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected SetNX to succeed on new key")
	}

	ok, err = c.SetNX(ctx, "k", "v2", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected SetNX to fail on existing key")
	}

	// Value should still be v1.
	v, _ := c.Get(ctx, "k")
	if v != "v1" {
		t.Errorf("expected v1, got %q", v)
	}
}

func TestMemoryCache_SetNX_AfterExpiry(t *testing.T) {
	c := newTestMemoryCache(t)
	ctx := context.Background()

	_, _ = c.SetNX(ctx, "k", "v1", time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	ok, err := c.SetNX(ctx, "k", "v2", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected SetNX to succeed after expiry")
	}
	v, _ := c.Get(ctx, "k")
	if v != "v2" {
		t.Errorf("expected v2, got %q", v)
	}
}

func TestMemoryCache_Overwrite(t *testing.T) {
	c := newTestMemoryCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "k", "v1", time.Minute)
	_ = c.Set(ctx, "k", "v2", time.Minute)
	v, _ := c.Get(ctx, "k")
	if v != "v2" {
		t.Errorf("expected v2, got %q", v)
	}
}

func TestMemoryCache_ZeroTTL(t *testing.T) {
	c := newTestMemoryCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "k", "v", 0)
	v, err := c.Get(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if v != "v" {
		t.Errorf("expected v, got %q", v)
	}
}

func TestMemoryCache_Ping(t *testing.T) {
	c := newTestMemoryCache(t)
	if err := c.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryCache_Close(t *testing.T) {
	c := newTestMemoryCache(t)
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewCache_Memory(t *testing.T) {
	c := NewCache("", "", 0)
	if _, ok := c.(*MemoryCache); !ok {
		t.Errorf("expected *MemoryCache, got %T", c)
	}
	_ = c.Close()
}

func TestNewCache_Redis(t *testing.T) {
	// Just verify it returns a non-nil Cache (no real Redis needed).
	c := NewCache("localhost:6379", "", 0)
	if c == nil {
		t.Error("expected non-nil Cache")
	}
	_ = c.Close()
}

func TestMemoryCache_Incr_Basic(t *testing.T) {
	c := newTestMemoryCache(t)
	ctx := context.Background()

	// First Incr on non-existent key should return 1.
	val, err := c.Incr(ctx, "counter", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if val != 1 {
		t.Errorf("expected 1, got %d", val)
	}

	// Second Incr should return 2.
	val, err = c.Incr(ctx, "counter", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if val != 2 {
		t.Errorf("expected 2, got %d", val)
	}
}

func TestMemoryCache_Incr_FixedWindow(t *testing.T) {
	c := newTestMemoryCache(t)
	ctx := context.Background()

	_, _ = c.Incr(ctx, "k", 50*time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	// Incr again should NOT reset TTL (fixed window).
	val, _ := c.Incr(ctx, "k", 50*time.Millisecond)
	if val != 2 {
		t.Errorf("expected 2, got %d", val)
	}
	time.Sleep(30 * time.Millisecond)
	// Key should have expired (original TTL was 50ms, 30+30=60ms elapsed).
	_, err := c.Get(ctx, "k")
	if err == nil {
		t.Fatalf("expected key to be expired, but it still exists")
	}
}

func TestMemoryCache_Incr_AfterExpiry(t *testing.T) {
	c := newTestMemoryCache(t)
	ctx := context.Background()

	_, _ = c.Incr(ctx, "k", time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	// After expiry, Incr should start from 1 again.
	val, _ := c.Incr(ctx, "k", time.Minute)
	if val != 1 {
		t.Errorf("expected 1 after expiry, got %d", val)
	}
}

func TestMemoryCache_Incr_Concurrent(t *testing.T) {
	c := newTestMemoryCache(t)
	ctx := context.Background()
	const goroutines = 100

	done := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		go func() {
			_, _ = c.Incr(ctx, "race", time.Minute)
			done <- struct{}{}
		}()
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}

	v, err := c.Get(ctx, "race")
	if err != nil {
		t.Fatal(err)
	}
	if v != "100" {
		t.Errorf("expected '100' after %d concurrent Incrs, got %q", goroutines, v)
	}
}

func TestMemoryCache_RefreshIfValue(t *testing.T) {
	c := newTestMemoryCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "leader", "owner-a", 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}

	ok, err := c.RefreshIfValue(ctx, "leader", "owner-a", 30*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected refresh to succeed for matching owner")
	}

	time.Sleep(15 * time.Millisecond)
	if _, err := c.Get(ctx, "leader"); err != nil {
		t.Fatalf("expected key to remain after refreshed ttl, got %v", err)
	}
}

func TestMemoryCache_RefreshIfValue_Mismatch(t *testing.T) {
	c := newTestMemoryCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "leader", "owner-a", 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}

	ok, err := c.RefreshIfValue(ctx, "leader", "owner-b", 30*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected refresh to fail for non-matching owner")
	}

	time.Sleep(15 * time.Millisecond)
	if _, err := c.Get(ctx, "leader"); err != ErrCacheMiss {
		t.Fatalf("expected key to expire without refresh, got %v", err)
	}
}

func TestMemoryCache_DelIfValue(t *testing.T) {
	c := newTestMemoryCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "leader", "owner-a", time.Minute); err != nil {
		t.Fatal(err)
	}

	ok, err := c.DelIfValue(ctx, "leader", "owner-a")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected delete to succeed for matching owner")
	}
	if _, err := c.Get(ctx, "leader"); err != ErrCacheMiss {
		t.Fatalf("expected key to be deleted, got %v", err)
	}
}

func TestMemoryCache_DelIfValue_Mismatch(t *testing.T) {
	c := newTestMemoryCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "leader", "owner-a", time.Minute); err != nil {
		t.Fatal(err)
	}

	ok, err := c.DelIfValue(ctx, "leader", "owner-b")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected delete to fail for non-matching owner")
	}
	if v, err := c.Get(ctx, "leader"); err != nil || v != "owner-a" {
		t.Fatalf("expected key to remain unchanged, value=%q err=%v", v, err)
	}
}

func TestRedisScripts_RequireMatchingValue(t *testing.T) {
	if strings.Contains(refreshIfValueScriptSource, "~= ARGV[1]") {
		t.Fatalf("refreshIfValueScriptSource must not use mismatch operator: %q", refreshIfValueScriptSource)
	}
	if !strings.Contains(refreshIfValueScriptSource, "== ARGV[1]") {
		t.Fatalf("refreshIfValueScriptSource must require exact value match: %q", refreshIfValueScriptSource)
	}
	if strings.Contains(delIfValueScriptSource, "~= ARGV[1]") {
		t.Fatalf("delIfValueScriptSource must not use mismatch operator: %q", delIfValueScriptSource)
	}
	if !strings.Contains(delIfValueScriptSource, "== ARGV[1]") {
		t.Fatalf("delIfValueScriptSource must require exact value match: %q", delIfValueScriptSource)
	}
}
