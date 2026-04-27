package webauthnhandler

import (
	"testing"
	"time"

	handlercommon "github.com/ysicing/go-template/handler"
)

func TestNormalizeWebAuthnCredentialName(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "trimmed value", raw: "  work laptop  ", want: "work laptop"},
		{name: "empty falls back", raw: "   ", want: defaultWebAuthnCredentialName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeWebAuthnCredentialName(tt.raw); got != tt.want {
				t.Fatalf("normalizeWebAuthnCredentialName(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestSplitCachedLoginSession(t *testing.T) {
	userID, session, ok := splitCachedLoginSession("user-1|{\"challenge\":\"abc\"}")
	if !ok {
		t.Fatal("expected cached login session to split successfully")
	}
	if userID != "user-1" || session != "{\"challenge\":\"abc\"}" {
		t.Fatalf("unexpected split result: userID=%q session=%q", userID, session)
	}

	if _, _, ok := splitCachedLoginSession("broken"); ok {
		t.Fatal("expected malformed cached login session to fail split")
	}
}

func TestRememberMeRefreshTTL(t *testing.T) {
	cfg := handlercommon.TokenConfig{
		RefreshTTL:    24 * time.Hour,
		RememberMeTTL: 30 * 24 * time.Hour,
	}

	if got := rememberMeRefreshTTL("1", cfg); got != cfg.RememberMeTTL {
		t.Fatalf("rememberMeRefreshTTL(\"1\") = %s, want %s", got, cfg.RememberMeTTL)
	}
	if got := rememberMeRefreshTTL("", cfg); got != cfg.RefreshTTL {
		t.Fatalf("rememberMeRefreshTTL(\"\") = %s, want %s", got, cfg.RefreshTTL)
	}
}
