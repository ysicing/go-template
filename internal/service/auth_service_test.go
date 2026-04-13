package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
)

type fakeLoginUserStore struct {
	user *model.User
	err  error
}

func (f fakeLoginUserStore) GetByUsernameOrEmail(context.Context, string) (*model.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.user, nil
}

func TestAuthServiceLoginRejectsLockedAccount(t *testing.T) {
	cache := store.NewMemoryCache()
	_ = cache.Set(context.Background(), "login_lock:user-1", "1", 15*time.Minute)

	passwordUser := &model.User{Base: model.Base{ID: "user-1"}, Username: "demo", Provider: "local"}
	require.NoError(t, passwordUser.SetPassword("Password123!abcd"))
	svc := NewAuthService(AuthServiceDeps{
		Users: fakeLoginUserStore{user: passwordUser},
		Cache: cache,
	})

	_, err := svc.Login(context.Background(), LoginInput{
		Identity: "demo",
		Password: "Password123!abcd",
	})

	require.ErrorIs(t, err, ErrAccountLocked)
}

func TestAuthServiceLoginReturnsUserOnValidPassword(t *testing.T) {
	cache := store.NewMemoryCache()
	passwordUser := &model.User{Base: model.Base{ID: "user-1"}, Username: "demo", Provider: "local"}
	require.NoError(t, passwordUser.SetPassword("Password123!abcd"))
	svc := NewAuthService(AuthServiceDeps{
		Users: fakeLoginUserStore{user: passwordUser},
		Cache: cache,
	})

	user, err := svc.Login(context.Background(), LoginInput{
		Identity: "demo",
		Password: "Password123!abcd",
	})

	require.NoError(t, err)
	require.Equal(t, "user-1", user.ID)
}

func TestAuthServiceLoginReturnsUserOnInvalidPassword(t *testing.T) {
	cache := store.NewMemoryCache()
	passwordUser := &model.User{Base: model.Base{ID: "user-1"}, Username: "demo", Provider: "local"}
	require.NoError(t, passwordUser.SetPassword("Password123!abcd"))
	svc := NewAuthService(AuthServiceDeps{
		Users: fakeLoginUserStore{user: passwordUser},
		Cache: cache,
	})

	user, err := svc.Login(context.Background(), LoginInput{
		Identity: "demo",
		Password: "wrong-password",
	})

	require.ErrorIs(t, err, ErrInvalidCredentials)
	require.NotNil(t, user)
	require.Equal(t, "user-1", user.ID)
}
