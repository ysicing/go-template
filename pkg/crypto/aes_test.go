package crypto

import (
	"testing"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	passphrase := "my-secret-key-for-testing"
	plaintext := "github-client-secret-abc123"

	ciphertext, err := Encrypt(passphrase, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if ciphertext == plaintext {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted, err := Decrypt(passphrase, ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestEncrypt_DifferentCiphertexts(t *testing.T) {
	passphrase := "test-key"
	plaintext := "same-secret"

	c1, _ := Encrypt(passphrase, plaintext)
	c2, _ := Encrypt(passphrase, plaintext)
	if c1 == c2 {
		t.Fatal("two encryptions of same plaintext should produce different ciphertexts (random salt + nonce)")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	passphrase1 := "key-one"
	passphrase2 := "key-two"

	ciphertext, _ := Encrypt(passphrase1, "secret")
	_, err := Decrypt(passphrase2, ciphertext)
	if err == nil {
		t.Fatal("expected error decrypting with wrong passphrase")
	}
}

func TestDecryptPlaintext_Fallback(t *testing.T) {
	passphrase := "test-key"
	plaintext := "not-encrypted-value"

	result, err := DecryptOrPlaintext(passphrase, plaintext)
	if err != nil {
		t.Fatalf("expected no error for plaintext: %v", err)
	}
	if result != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, result)
	}
}

func TestDecryptOrPlaintext_Encrypted(t *testing.T) {
	passphrase := "test-key"
	plaintext := "secret-value"

	encrypted, _ := Encrypt(passphrase, plaintext)
	decrypted, err := DecryptOrPlaintext(passphrase, encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestEncrypt_EmptyPlaintext(t *testing.T) {
	passphrase := "test-key"
	ciphertext, err := Encrypt(passphrase, "")
	if err != nil {
		t.Fatalf("encrypt empty: %v", err)
	}
	decrypted, err := Decrypt(passphrase, ciphertext)
	if err != nil {
		t.Fatalf("decrypt empty: %v", err)
	}
	if decrypted != "" {
		t.Fatalf("expected empty string, got %q", decrypted)
	}
}

func TestIsEncrypted(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"enc:abc123", true},
		{"plaintext", false},
		{"", false},
		{"enc", false},
	}

	for _, tt := range tests {
		result := IsEncrypted(tt.value)
		if result != tt.expected {
			t.Errorf("IsEncrypted(%q) = %v, want %v", tt.value, result, tt.expected)
		}
	}
}
