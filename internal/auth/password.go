package auth

import "golang.org/x/crypto/bcrypt"

func HashPassword(raw string) (string, error) {
	data, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	return string(data), err
}

func CheckPassword(hash string, raw string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw))
}

