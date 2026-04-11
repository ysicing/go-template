package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var ErrEphemeralTokenNotFound = errors.New("ephemeral token not found")

type EphemeralTokenStore struct {
	cache ephemeralCache
}

type ephemeralCache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	DelIfValue(ctx context.Context, key, value string) (bool, error)
}

func NewEphemeralTokenStore(cache ephemeralCache) *EphemeralTokenStore {
	return &EphemeralTokenStore{cache: cache}
}

func ephemeralTokenKey(category, name, token string) string {
	return "ephemeral:" + category + ":" + name + ":" + token
}

func (s *EphemeralTokenStore) IssueString(ctx context.Context, category, name, token, subject string, ttl time.Duration) error {
	if s == nil || s.cache == nil {
		return ErrEphemeralTokenNotFound
	}
	return s.cache.Set(ctx, ephemeralTokenKey(category, name, token), subject, ttl)
}

func (s *EphemeralTokenStore) LoadString(ctx context.Context, category, name, token string) (string, error) {
	if s == nil || s.cache == nil {
		return "", ErrEphemeralTokenNotFound
	}
	val, err := s.cache.Get(ctx, ephemeralTokenKey(category, name, token))
	if err != nil {
		return "", ErrEphemeralTokenNotFound
	}
	return val, nil
}

func (s *EphemeralTokenStore) ConsumeString(ctx context.Context, category, name, token string) (string, error) {
	val, err := s.LoadString(ctx, category, name, token)
	if err != nil {
		return "", err
	}
	ok, err := s.cache.DelIfValue(ctx, ephemeralTokenKey(category, name, token), val)
	if err != nil || !ok {
		return "", ErrEphemeralTokenNotFound
	}
	return val, nil
}

func (s *EphemeralTokenStore) IssueJSON(ctx context.Context, category, name, token string, payload any, ttl time.Duration) error {
	if s == nil || s.cache == nil {
		return ErrEphemeralTokenNotFound
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.cache.Set(ctx, ephemeralTokenKey(category, name, token), string(raw), ttl)
}

func (s *EphemeralTokenStore) LoadJSON(ctx context.Context, category, name, token string, out any) error {
	if s == nil || s.cache == nil {
		return ErrEphemeralTokenNotFound
	}
	raw, err := s.cache.Get(ctx, ephemeralTokenKey(category, name, token))
	if err != nil {
		return ErrEphemeralTokenNotFound
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return err
	}
	return nil
}

func (s *EphemeralTokenStore) ConsumeJSON(ctx context.Context, category, name, token string, out any) error {
	if s == nil || s.cache == nil {
		return ErrEphemeralTokenNotFound
	}
	key := ephemeralTokenKey(category, name, token)
	raw, err := s.cache.Get(ctx, key)
	if err != nil {
		return ErrEphemeralTokenNotFound
	}
	ok, err := s.cache.DelIfValue(ctx, key, raw)
	if err != nil || !ok {
		return ErrEphemeralTokenNotFound
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return err
	}
	return nil
}
