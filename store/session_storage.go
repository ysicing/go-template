package store

import (
	"github.com/gofiber/fiber/v3"
	fibermemory "github.com/gofiber/storage/memory/v2"
	fiberredis "github.com/gofiber/storage/redis/v3"
)

type SessionStorageResource struct {
	Storage   fiber.Storage
	CloseFunc func() error
}

func (r SessionStorageResource) Close() error {
	if r.CloseFunc == nil {
		return nil
	}
	return r.CloseFunc()
}

func NewSessionStorageResource(cache Cache) SessionStorageResource {
	if redisCache, ok := cache.(*redisCache); ok && redisCache.client != nil {
		return SessionStorageResource{
			Storage:   fiberredis.NewFromConnection(redisCache.client),
			CloseFunc: func() error { return nil },
		}
	}

	storage := fibermemory.New()
	return SessionStorageResource{
		Storage:   storage,
		CloseFunc: storage.Close,
	}
}
