package store

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// Cache is a generic key-value cache interface.
// Implementations include MemoryCache (default) and RedisCache.
type Cache interface {
	Ping(ctx context.Context) error
	Close() error
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Del(ctx context.Context, key string) error
	SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
	RefreshIfValue(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
	DelIfValue(ctx context.Context, key, value string) (bool, error)
	// Incr atomically increments a key's integer value by 1 and returns the new value.
	// If the key does not exist, it is initialized to 1. TTL is set/reset on each call.
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
	// GetInt retrieves an integer value without incrementing it. Returns 0 if key doesn't exist.
	GetInt(ctx context.Context, key string) (int64, error)
}

// ErrCacheMiss is returned when a key is not found in the cache.
var ErrCacheMiss = errCacheMiss{}

type errCacheMiss struct{}

func (errCacheMiss) Error() string { return "cache: key not found" }

// --- MemoryCache ---

type memEntry struct {
	value     string
	expiresAt time.Time
}

func (e memEntry) expired() bool {
	return !e.expiresAt.IsZero() && time.Now().After(e.expiresAt)
}

// MemoryCache is an in-process cache backed by a map with TTL support.
type MemoryCache struct {
	mu         sync.RWMutex
	entries    map[string]memEntry
	maxEntries int

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

const defaultMemoryCacheMaxEntries = 10000

// NewMemoryCache creates a MemoryCache.
func NewMemoryCache() *MemoryCache {
	return NewMemoryCacheWithCapacity(defaultMemoryCacheMaxEntries)
}

// NewMemoryCacheWithCapacity creates a MemoryCache with a hard entry cap.
func NewMemoryCacheWithCapacity(maxEntries int) *MemoryCache {
	m := &MemoryCache{
		entries:    make(map[string]memEntry),
		maxEntries: maxEntries,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
	go m.cleanupExpiredLoop(1 * time.Minute)
	return m
}

func (m *MemoryCache) Ping(_ context.Context) error { return nil }

func (m *MemoryCache) Close() error {
	m.stopOnce.Do(func() {
		close(m.stopCh)
		<-m.doneCh
	})
	return nil
}

func (m *MemoryCache) cleanupExpiredLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer close(m.doneCh)
	for {
		select {
		case <-ticker.C:
			m.cleanupExpired()
		case <-m.stopCh:
			return
		}
	}
}

func (m *MemoryCache) cleanupExpired() {
	now := time.Now()
	m.mu.Lock()
	m.cleanupExpiredLocked(now)
	m.mu.Unlock()
}

func (m *MemoryCache) cleanupExpiredLocked(now time.Time) {
	for key, e := range m.entries {
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			delete(m.entries, key)
		}
	}
}

func (m *MemoryCache) ensureCapacityLocked() {
	if m.maxEntries <= 0 || len(m.entries) < m.maxEntries {
		return
	}
	m.cleanupExpiredLocked(time.Now())
	if len(m.entries) < m.maxEntries {
		return
	}
	for key := range m.entries {
		delete(m.entries, key)
		return
	}
}

func (m *MemoryCache) Get(_ context.Context, key string) (string, error) {
	m.mu.RLock()
	e, ok := m.entries[key]
	valid := ok && !e.expired()
	m.mu.RUnlock()
	if !valid {
		return "", ErrCacheMiss
	}
	return e.value, nil
}

func (m *MemoryCache) Set(_ context.Context, key, value string, ttl time.Duration) error {
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	m.mu.Lock()
	if _, exists := m.entries[key]; !exists {
		m.ensureCapacityLocked()
	}
	m.entries[key] = memEntry{value: value, expiresAt: exp}
	m.mu.Unlock()
	return nil
}

func (m *MemoryCache) Del(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.entries, key)
	m.mu.Unlock()
	return nil
}

func (m *MemoryCache) SetNX(_ context.Context, key, value string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.entries[key]; ok && !e.expired() {
		return false, nil
	}
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	m.ensureCapacityLocked()
	m.entries[key] = memEntry{value: value, expiresAt: exp}
	return true, nil
}

func (m *MemoryCache) RefreshIfValue(_ context.Context, key, value string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[key]
	if !ok || e.expired() || e.value != value {
		if ok && e.expired() {
			delete(m.entries, key)
		}
		return false, nil
	}
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	m.entries[key] = memEntry{value: value, expiresAt: exp}
	return true, nil
}

func (m *MemoryCache) DelIfValue(_ context.Context, key, value string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[key]
	if !ok || e.expired() || e.value != value {
		if ok && e.expired() {
			delete(m.entries, key)
		}
		return false, nil
	}
	delete(m.entries, key)
	return true, nil
}

func (m *MemoryCache) Incr(_ context.Context, key string, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var count int64
	var exp time.Time
	_, exists := m.entries[key]
	if e, ok := m.entries[key]; ok && !e.expired() {
		n, err := strconv.ParseInt(e.value, 10, 64)
		if err == nil {
			count = n
		}
		exp = e.expiresAt // preserve existing TTL
	}
	count++
	if count == 1 && ttl > 0 {
		exp = time.Now().Add(ttl) // only set TTL on first increment
	}
	if !exists {
		m.ensureCapacityLocked()
	}
	m.entries[key] = memEntry{value: strconv.FormatInt(count, 10), expiresAt: exp}
	return count, nil
}

func (m *MemoryCache) GetInt(_ context.Context, key string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if e, ok := m.entries[key]; ok && !e.expired() {
		n, err := strconv.ParseInt(e.value, 10, 64)
		if err != nil {
			return 0, nil
		}
		return n, nil
	}
	return 0, nil
}

// NewCache creates a Cache. If Redis addr is provided, returns a Redis-backed cache;
// otherwise returns an in-memory cache.
func NewCache(redisAddr, redisPassword string, redisDB int) Cache {
	if redisAddr == "" {
		return NewMemoryCache()
	}
	return newRedisCache(redisAddr, redisPassword, redisDB)
}
