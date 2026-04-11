package validator

import "testing"

func TestValidateEmailDomain_Disabled(t *testing.T) {
	err := ValidateEmailDomain("user@any.com", "disabled", nil, nil)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	err = ValidateEmailDomain("user@any.com", "", nil, nil)
	if err != nil {
		t.Errorf("expected nil for empty mode, got %v", err)
	}
}

func TestValidateEmailDomain_Whitelist(t *testing.T) {
	whitelist := []string{"company.com", "Partner.com"}
	if err := ValidateEmailDomain("user@company.com", "whitelist", whitelist, nil); err != nil {
		t.Errorf("expected allowed, got %v", err)
	}
	if err := ValidateEmailDomain("user@partner.com", "whitelist", whitelist, nil); err != nil {
		t.Errorf("expected case-insensitive match, got %v", err)
	}
	if err := ValidateEmailDomain("user@other.com", "whitelist", whitelist, nil); err == nil {
		t.Error("expected error for non-whitelisted domain")
	}
}

func TestValidateEmailDomain_Blacklist(t *testing.T) {
	blacklist := []string{"mailinator.com", "tempmail.com"}
	if err := ValidateEmailDomain("user@good.com", "blacklist", nil, blacklist); err != nil {
		t.Errorf("expected allowed, got %v", err)
	}
	if err := ValidateEmailDomain("user@mailinator.com", "blacklist", nil, blacklist); err == nil {
		t.Error("expected error for blacklisted domain")
	}
}

func TestValidateEmailDomain_InvalidEmail(t *testing.T) {
	if err := ValidateEmailDomain("invalid-email", "blacklist", nil, nil); err == nil {
		t.Error("expected error for invalid email")
	}
}
