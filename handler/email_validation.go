package handler

import (
	"net/mail"
	"strings"
)

func isValidEmail(email string) bool {
	parsed, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}
	parts := strings.SplitN(parsed.Address, "@", 2)
	if len(parts) != 2 {
		return false
	}
	domain := parts[1]
	return domain != "" && strings.Contains(domain, ".") && !strings.HasSuffix(domain, ".")
}

func IsValidEmail(email string) bool {
	return isValidEmail(email)
}
