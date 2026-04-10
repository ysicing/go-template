package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/ysicing/go-template/internal/config"
)

type Store interface {
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, bool, error)
	Delete(ctx context.Context, key string) error
	Ping(ctx context.Context) error
}

func NewStore(ctx context.Context, cfg config.CacheConfig) (Store, error) {
	switch cfg.Driver {
	case "", "memory":
		store := NewMemoryStore()
		return store, store.Ping(ctx)
	case "redis":
		if cfg.Addr == "" {
			return nil, fmt.Errorf("redis addr is required")
		}

		client := redis.NewClient(&redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		})
		store := NewRedisStore(client)
		return store, store.Ping(ctx)
	default:
		return nil, fmt.Errorf("unsupported cache driver: %s", cfg.Driver)
	}
}

