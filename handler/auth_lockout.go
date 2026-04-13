package handler

import (
	"context"

	"github.com/ysicing/go-template/internal/service"
	"github.com/ysicing/go-template/store"
)

func isAccountLocked(ctx context.Context, cache store.Cache, userID string) bool {
	return service.IsAccountLocked(ctx, cache, userID)
}

func recordFailedAuthAttempt(ctx context.Context, cache store.Cache, userID string) {
	service.RecordFailedAuthAttempt(ctx, cache, userID)
}

func clearFailedAuthAttempts(ctx context.Context, cache store.Cache, userID string) {
	service.ClearFailedAuthAttempts(ctx, cache, userID)
}
