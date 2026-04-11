package model

import "testing"

func TestValidatePasswordStrength_TooShort(t *testing.T) {
	err := ValidatePasswordStrength("Abc1!xyz")
	if err == nil {
		t.Error("expected error for short password")
	}
}

func TestValidatePasswordStrength_NoUpper(t *testing.T) {
	err := ValidatePasswordStrength("abcdefgh1234!")
	if err == nil {
		t.Error("expected error for no uppercase")
	}
}

func TestValidatePasswordStrength_NoLower(t *testing.T) {
	err := ValidatePasswordStrength("ABCDEFGH1234!")
	if err == nil {
		t.Error("expected error for no lowercase")
	}
}

func TestValidatePasswordStrength_NoDigit(t *testing.T) {
	err := ValidatePasswordStrength("Abcdefghijkl!")
	if err == nil {
		t.Error("expected error for no digit")
	}
}

func TestValidatePasswordStrength_NoSpecial(t *testing.T) {
	err := ValidatePasswordStrength("Abcdefgh1234")
	if err == nil {
		t.Error("expected error for no special char")
	}
}

func TestValidatePasswordStrength_Valid(t *testing.T) {
	err := ValidatePasswordStrength("MyP@ssw0rd123")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestPasswordHistory_MatchesPassword(t *testing.T) {
	u := &User{}
	_ = u.SetPassword("TestPassword1!")
	ph := &PasswordHistory{PasswordHash: u.PasswordHash}

	if !ph.MatchesPassword("TestPassword1!") {
		t.Error("expected MatchesPassword to return true")
	}
	if ph.MatchesPassword("WrongPassword1!") {
		t.Error("expected MatchesPassword to return false")
	}
}
