package store

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisCache implements Cache using Redis.
type redisCache struct {
	client *redis.Client
}

// newRedisCache creates a Redis-backed Cache.
func newRedisCache(addr, password string, db int) *redisCache {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &redisCache{client: client}
}

func (r *redisCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *redisCache) Close() error {
	return r.client.Close()
}

func (r *redisCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *redisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	return val, err
}

func (r *redisCache) Del(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *redisCache) SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, value, ttl).Result()
}

const refreshIfValueScriptSource = `
local current = redis.call('GET', KEYS[1])
if current == ARGV[1] then
    if tonumber(ARGV[2]) > 0 then
        redis.call('PEXPIRE', KEYS[1], ARGV[2])
    else
        redis.call('PERSIST', KEYS[1])
    end
    return 1
end
return 0
`

var refreshIfValueScript = redis.NewScript(refreshIfValueScriptSource)

func (r *redisCache) RefreshIfValue(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	ttlMS := int64(ttl / time.Millisecond)
	res, err := refreshIfValueScript.Run(ctx, r.client, []string{key}, value, ttlMS).Int()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

const delIfValueScriptSource = `
local current = redis.call('GET', KEYS[1])
if current == ARGV[1] then
    redis.call('DEL', KEYS[1])
    return 1
end
return 0
`

var delIfValueScript = redis.NewScript(delIfValueScriptSource)

func (r *redisCache) DelIfValue(ctx context.Context, key, value string) (bool, error) {
	res, err := delIfValueScript.Run(ctx, r.client, []string{key}, value).Int()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// incrWithTTL is a Lua script that atomically increments a key and sets its TTL.
var incrWithTTL = redis.NewScript(`
local val = redis.call('INCR', KEYS[1])
if val == 1 and tonumber(ARGV[1]) > 0 then
    redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return val
`)

func (r *redisCache) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	ttlSec := int64(0)
	if ttl > 0 {
		ttlSec = int64(ttl.Seconds())
	}
	val, err := incrWithTTL.Run(ctx, r.client, []string{key}, ttlSec).Int64()
	if err != nil {
		return 0, err
	}
	return val, nil
}

func (r *redisCache) GetInt(ctx context.Context, key string) (int64, error) {
	val, err := r.client.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return val, nil
}
