package model

const (
	OrganizationRoleOwner  = "owner"
	OrganizationRoleAdmin  = "admin"
	OrganizationRoleMember = "member"
)

type Organization struct {
	Base
	Name            string `gorm:"type:varchar(255)" json:"name"`
	Slug            string `gorm:"uniqueIndex;type:varchar(64)" json:"slug"`
	CreatedByUserID string `gorm:"index;type:varchar(36)" json:"created_by_user_id"`
}

func (Organization) TableName() string { return "organizations" }

type OrganizationMember struct {
	Base
	OrganizationID string `gorm:"uniqueIndex:idx_org_members_org_user;index;type:varchar(36)" json:"organization_id"`
	UserID         string `gorm:"uniqueIndex:idx_org_members_org_user;index;type:varchar(36)" json:"user_id"`
	Role           string `gorm:"type:varchar(16);index" json:"role"`
}

func (OrganizationMember) TableName() string { return "organization_members" }

func IsValidOrganizationRole(role string) bool {
	switch role {
	case OrganizationRoleOwner, OrganizationRoleAdmin, OrganizationRoleMember:
		return true
	default:
		return false
	}
}
