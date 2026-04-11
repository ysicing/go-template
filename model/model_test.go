package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSetPassword(t *testing.T) {
	u := &User{}
	if err := u.SetPassword("secret123"); err != nil {
		t.Fatal(err)
	}
	if u.PasswordHash == "" {
		t.Error("expected PasswordHash to be set, got empty string")
	}
}

func TestCheckPassword_Correct(t *testing.T) {
	u := &User{}
	if err := u.SetPassword("correct-password"); err != nil {
		t.Fatal(err)
	}
	if !u.CheckPassword("correct-password") {
		t.Error("expected CheckPassword to return true for correct password")
	}
}

func TestCheckPassword_Wrong(t *testing.T) {
	u := &User{}
	if err := u.SetPassword("correct-password"); err != nil {
		t.Fatal(err)
	}
	if u.CheckPassword("wrong-password") {
		t.Error("expected CheckPassword to return false for wrong password")
	}
}

func TestCheckPassword_EmptyHash(t *testing.T) {
	u := &User{}
	if u.CheckPassword("anything") {
		t.Error("expected CheckPassword to return false with empty hash")
	}
}

func TestSetPassword_DifferentHashes(t *testing.T) {
	u1 := &User{}
	u2 := &User{}
	if err := u1.SetPassword("same-password"); err != nil {
		t.Fatal(err)
	}
	if err := u2.SetPassword("same-password"); err != nil {
		t.Fatal(err)
	}
	if u1.PasswordHash == u2.PasswordHash {
		t.Error("expected different hashes for same password due to bcrypt salt")
	}
}

func TestPasswordHashNotExposed(t *testing.T) {
	u := &User{
		Username:   "testuser",
		Email:      "test@example.com",
		InviteCode: "TESTINVITE",
	}
	if err := u.SetPassword("secret"); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "password_hash") {
		t.Error("expected password_hash to be excluded from JSON output")
	}
	if strings.Contains(string(data), u.PasswordHash) {
		t.Error("expected password hash value to be excluded from JSON output")
	}
}
