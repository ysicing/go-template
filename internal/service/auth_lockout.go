package service

import (
	"context"
	"time"

	"github.com/ysicing/go-template/store"
)

const (
	lockoutThreshold = 5
	lockoutTTL       = 15 * time.Minute
)

func LoginFailKey(userID string) string { return "login_fail:" + userID }
func LoginLockKey(userID string) string { return "login_lock:" + userID }

func IsAccountLocked(ctx context.Context, cache store.Cache, userID string) bool {
	val, err := cache.Get(ctx, LoginLockKey(userID))
	return err == nil && val != ""
}

func RecordFailedAuthAttempt(ctx context.Context, cache store.Cache, userID string) {
	key := LoginFailKey(userID)
	count, _ := cache.Incr(ctx, key, lockoutTTL)
	if count >= int64(lockoutThreshold) {
		_ = cache.Set(ctx, LoginLockKey(userID), "1", lockoutTTL)
	}
}

func ClearFailedAuthAttempts(ctx context.Context, cache store.Cache, userID string) {
	_ = cache.Del(ctx, LoginFailKey(userID))
	_ = cache.Del(ctx, LoginLockKey(userID))
}
