package handler

import (
	"context"

	authservice "github.com/ysicing/go-template/internal/service/auth"
	"github.com/ysicing/go-template/store"
)

func isAccountLocked(ctx context.Context, cache store.Cache, userID string) bool {
	return authservice.IsAccountLocked(ctx, cache, userID)
}

func recordFailedAuthAttempt(ctx context.Context, cache store.Cache, userID string) {
	authservice.RecordFailedAuthAttempt(ctx, cache, userID)
}

func clearFailedAuthAttempts(ctx context.Context, cache store.Cache, userID string) {
	authservice.ClearFailedAuthAttempts(ctx, cache, userID)
}

func IsAccountLocked(ctx context.Context, cache store.Cache, userID string) bool {
	return isAccountLocked(ctx, cache, userID)
}

func RecordFailedAuthAttempt(ctx context.Context, cache store.Cache, userID string) {
	recordFailedAuthAttempt(ctx, cache, userID)
}

func ClearFailedAuthAttempts(ctx context.Context, cache store.Cache, userID string) {
	clearFailedAuthAttempts(ctx, cache, userID)
}
