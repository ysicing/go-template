package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

// EncPrefix marks a value as AES-GCM encrypted with random salt (v2 format).
const (
	EncPrefix    = "enc:"
	pbkdf2Iter   = 100000
	pbkdf2KeyLen = 32
	saltLen      = 16 // 16 bytes = 128 bits
)

// DeriveKey derives a 32-byte AES-256 key using PBKDF2-SHA256 with a custom salt.
func DeriveKey(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iter, pbkdf2KeyLen, sha256.New)
}

// Encrypt encrypts plaintext with AES-256-GCM using a random per-value salt.
// Format: enc:base64(salt || nonce || ciphertext)
func Encrypt(passphrase, plaintext string) (string, error) {
	// Generate random salt
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}

	// Derive key from passphrase + salt
	key := DeriveKey(passphrase, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Encrypt: ciphertext = nonce || encrypted_data
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Combine: salt || ciphertext (which already contains nonce)
	combined := append(salt, ciphertext...)

	return EncPrefix + base64.StdEncoding.EncodeToString(combined), nil
}

// Decrypt decrypts an EncPrefix-prefixed base64 AES-GCM ciphertext with embedded salt.
func Decrypt(passphrase, encoded string) (string, error) {
	if !strings.HasPrefix(encoded, EncPrefix) {
		return "", errors.New("not an encrypted value")
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded, EncPrefix))
	if err != nil {
		return "", err
	}

	// Extract salt (first saltLen bytes)
	if len(data) < saltLen {
		return "", errors.New("ciphertext too short for salt")
	}
	salt := data[:saltLen]
	ciphertext := data[saltLen:]

	// Derive key from passphrase + salt
	key := DeriveKey(passphrase, salt)

	return decryptRaw(key, ciphertext)
}

func decryptRaw(key, data []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// DecryptOrPlaintext tries to decrypt; if the value lacks EncPrefix, returns it as-is.
func DecryptOrPlaintext(passphrase, value string) (string, error) {
	if !strings.HasPrefix(value, EncPrefix) {
		return value, nil
	}
	return Decrypt(passphrase, value)
}

// IsEncrypted returns true if the value is encrypted.
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, EncPrefix)
}
