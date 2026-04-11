package store

import (
	"context"
	"testing"
	"time"
)

func TestEphemeralTokenStore_IssueLoadAndConsumeString(t *testing.T) {
	cache := NewMemoryCache()
	t.Cleanup(func() { _ = cache.Close() })

	s := NewEphemeralTokenStore(cache)
	ctx := context.Background()

	if err := s.IssueString(ctx, "verify", "email", "token-1", "user-1", time.Minute); err != nil {
		t.Fatalf("issue string token: %v", err)
	}

	got, err := s.LoadString(ctx, "verify", "email", "token-1")
	if err != nil {
		t.Fatalf("load string token: %v", err)
	}
	if got != "user-1" {
		t.Fatalf("expected subject user-1, got %q", got)
	}

	consumed, err := s.ConsumeString(ctx, "verify", "email", "token-1")
	if err != nil {
		t.Fatalf("consume string token: %v", err)
	}
	if consumed != "user-1" {
		t.Fatalf("expected consumed subject user-1, got %q", consumed)
	}

	if _, err := s.LoadString(ctx, "verify", "email", "token-1"); err == nil {
		t.Fatal("expected consumed token to be unavailable")
	}
}

func TestEphemeralTokenStore_IssueLoadAndConsumeJSON(t *testing.T) {
	cache := NewMemoryCache()
	t.Cleanup(func() { _ = cache.Close() })

	s := NewEphemeralTokenStore(cache)
	ctx := context.Background()

	type payload struct {
		UserID   string `json:"user_id"`
		Provider string `json:"provider"`
	}

	expected := payload{UserID: "user-2", Provider: "github"}
	if err := s.IssueJSON(ctx, "bind", "provider_token", "token-2", expected, time.Minute); err != nil {
		t.Fatalf("issue json token: %v", err)
	}

	var loaded payload
	if err := s.LoadJSON(ctx, "bind", "provider_token", "token-2", &loaded); err != nil {
		t.Fatalf("load json token: %v", err)
	}
	if loaded != expected {
		t.Fatalf("expected payload %#v, got %#v", expected, loaded)
	}

	var consumed payload
	if err := s.ConsumeJSON(ctx, "bind", "provider_token", "token-2", &consumed); err != nil {
		t.Fatalf("consume json token: %v", err)
	}
	if consumed != expected {
		t.Fatalf("expected consumed payload %#v, got %#v", expected, consumed)
	}

	if err := s.LoadJSON(ctx, "bind", "provider_token", "token-2", &loaded); err == nil {
		t.Fatal("expected consumed json token to be unavailable")
	}
}

func TestEphemeralTokenKey(t *testing.T) {
	got := ephemeralTokenKey("bind", "provider_token", "abc123")
	want := "ephemeral:bind:provider_token:abc123"
	if got != want {
		t.Fatalf("expected key %q, got %q", want, got)
	}
}
