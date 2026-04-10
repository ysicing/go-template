package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/ysicing/go-template/internal/cache"
)

func TestMemoryStoreSetGet(t *testing.T) {
	store := cache.NewMemoryStore()
	ctx := context.Background()

	if err := store.Set(ctx, "k", "v", time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}

	value, ok, err := store.Get(ctx, "k")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok || value != "v" {
		t.Fatalf("unexpected value: %v %v", ok, value)
	}
}

func TestMemoryStoreExpired(t *testing.T) {
	store := cache.NewMemoryStore()
	ctx := context.Background()

	if err := store.Set(ctx, "k", "v", time.Nanosecond); err != nil {
		t.Fatalf("set: %v", err)
	}
	time.Sleep(time.Millisecond)

	_, ok, err := store.Get(ctx, "k")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if ok {
		t.Fatal("expected cache miss for expired item")
	}
}

