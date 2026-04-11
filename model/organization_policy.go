package model

type OrganizationPolicy struct {
	Base
	OrganizationID        string `gorm:"uniqueIndex;type:varchar(36)" json:"organization_id"`
	EnforceRequireConsent bool   `gorm:"default:false" json:"enforce_require_consent"`
}

func (OrganizationPolicy) TableName() string { return "organization_policies" }
