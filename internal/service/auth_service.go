package service

import (
	"context"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
)

const (
	lockoutThreshold = 5
	lockoutTTL       = 15 * time.Minute
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountLocked      = errors.New("account temporarily locked")
)

var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), 12)

type loginUserStore interface {
	GetByUsernameOrEmail(ctx context.Context, identity string) (*model.User, error)
}

type AuthServiceDeps struct {
	Users loginUserStore
	Cache store.Cache
}

type LoginInput struct {
	Identity string
	Password string
}

type AuthService struct {
	users loginUserStore
	cache store.Cache
}

func NewAuthService(deps AuthServiceDeps) *AuthService {
	return &AuthService{users: deps.Users, cache: deps.Cache}
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*model.User, error) {
	user, err := s.users.GetByUsernameOrEmail(ctx, input.Identity)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(input.Password))
		return nil, ErrInvalidCredentials
	}
	if isAccountLocked(ctx, s.cache, user.ID) {
		return user, ErrAccountLocked
	}
	if !user.CheckPassword(input.Password) {
		recordFailedAuthAttempt(ctx, s.cache, user.ID)
		return nil, ErrInvalidCredentials
	}
	clearFailedAuthAttempts(ctx, s.cache, user.ID)
	return user, nil
}

func loginFailKey(userID string) string { return "login_fail:" + userID }
func loginLockKey(userID string) string { return "login_lock:" + userID }

func isAccountLocked(ctx context.Context, cache store.Cache, userID string) bool {
	val, err := cache.Get(ctx, loginLockKey(userID))
	return err == nil && val != ""
}

func recordFailedAuthAttempt(ctx context.Context, cache store.Cache, userID string) {
	key := loginFailKey(userID)
	count, _ := cache.Incr(ctx, key, lockoutTTL)
	if count >= int64(lockoutThreshold) {
		_ = cache.Set(ctx, loginLockKey(userID), "1", lockoutTTL)
	}
}

func clearFailedAuthAttempts(ctx context.Context, cache store.Cache, userID string) {
	_ = cache.Del(ctx, loginFailKey(userID))
	_ = cache.Del(ctx, loginLockKey(userID))
}
