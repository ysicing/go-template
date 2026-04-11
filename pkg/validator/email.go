package validator

import (
	"fmt"
	"strings"
)

// ValidateEmailDomain checks if the email domain is allowed.
func ValidateEmailDomain(email, mode string, whitelist, blacklist []string) error {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return fmt.Errorf("invalid email format")
	}
	domain := strings.ToLower(parts[1])

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
