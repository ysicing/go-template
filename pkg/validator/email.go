package validator

import (
	"fmt"
	"net/mail"
	"strings"
)

// IsValidEmail checks whether the input looks like a normal email address.
func IsValidEmail(email string) bool {
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

// ValidateEmailDomain checks if the email domain is allowed.
func ValidateEmailDomain(email, mode string, whitelist, blacklist []string) error {
	if !IsValidEmail(email) {
		return fmt.Errorf("invalid email format")
	}
	domain := strings.ToLower(strings.SplitN(email, "@", 2)[1])

	switch mode {
	case "whitelist":
		for _, allowed := range whitelist {
			if domain == strings.ToLower(strings.TrimSpace(allowed)) {
				return nil
			}
		}
		return fmt.Errorf("email domain not allowed")
	case "blacklist":
		for _, blocked := range blacklist {
			if domain == strings.ToLower(strings.TrimSpace(blocked)) {
				return fmt.Errorf("email domain not allowed")
			}
		}
		return nil
	default: // "disabled" or empty
		return nil
	}
}

// DefaultTempEmailDomains is a preset blacklist of temporary email domains.
var DefaultTempEmailDomains = []string{
	"10minutemail.com",
	"guerrillamail.com",
	"mailinator.com",
	"tempmail.com",
	"throwaway.email",
	"yopmail.com",
	"trashmail.com",
	"sharklasers.com",
	"guerrillamailblock.com",
	"grr.la",
}
