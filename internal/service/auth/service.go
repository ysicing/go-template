package authservice

import (
	"context"
	"errors"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountLocked      = errors.New("account temporarily locked")
)

var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), 12)

func CompareWithDummyHash(password string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}

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
	user, err := s.login(ctx, input)
	if errors.Is(err, ErrInvalidCredentials) {
		return nil, err
	}
	return user, err
}

func (s *AuthService) LoginForAudit(ctx context.Context, input LoginInput) (*model.User, error) {
	return s.login(ctx, input)
}

func (s *AuthService) login(ctx context.Context, input LoginInput) (*model.User, error) {
	user, err := s.users.GetByUsernameOrEmail(ctx, input.Identity)
	if err != nil {
		CompareWithDummyHash(input.Password)
		return nil, ErrInvalidCredentials
	}
	if IsAccountLocked(ctx, s.cache, user.ID) {
		return user, ErrAccountLocked
	}
	if !user.CheckPassword(input.Password) {
		RecordFailedAuthAttempt(ctx, s.cache, user.ID)
		return user, ErrInvalidCredentials
	}
	ClearFailedAuthAttempts(ctx, s.cache, user.ID)
	return user, nil
}
