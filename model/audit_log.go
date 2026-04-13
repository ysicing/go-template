package model

// AuditLog records security-relevant actions.
type AuditLog struct {
	Base
	UserID     string `gorm:"index;type:varchar(36)" json:"user_id"`
	Action     string `gorm:"type:varchar(50);index" json:"action"`
	Resource   string `gorm:"type:varchar(50)" json:"resource"`
	ResourceID string `gorm:"type:varchar(36)" json:"resource_id"`
	ClientID   string `gorm:"index;type:varchar(36)" json:"client_id"`
	IP         string `gorm:"type:varchar(45)" json:"ip"`
	UserAgent  string `gorm:"type:varchar(512)" json:"user_agent"`
	Detail     string `gorm:"type:text" json:"detail"`
	Status     string `gorm:"type:varchar(20)" json:"status"`
}

func (AuditLog) TableName() string { return "audit_logs" }

// Audit action constants.
const (
	AuditLogin                  = "login"
	AuditLoginFailed            = "login_failed"
	AuditLogout                 = "logout"
	AuditRegister               = "register"
	AuditPasswordChange         = "password_change"
	AuditMFAEnable              = "mfa_enable"
	AuditMFADisable             = "mfa_disable"
	AuditMFAVerify              = "mfa_verify"
	AuditMFABackupRegenerate    = "mfa_backup_regenerate"
	AuditWebAuthnAdd            = "webauthn_add"
	AuditWebAuthnRemove         = "webauthn_remove"
	AuditUserUpdate             = "user_update"
	AuditUserDelete             = "user_delete"
	AuditUserCreate             = "user_create"
	AuditSettingUpdate          = "setting_update"
	AuditSessionRevoke          = "session_revoke"
	AuditPointsCheckIn          = "points_checkin"
	AuditPointsSpend            = "points_spend"
	AuditPointsAdjust           = "points_adjust"
	AuditProviderCreate         = "provider_create"
	AuditProviderUpdate         = "provider_update"
	AuditProviderDelete         = "provider_delete"
	AuditAppCreate              = "app_create"
	AuditAppUpdate              = "app_update"
	AuditAppDelete              = "app_delete"
	AuditAppRotateSecret        = "app_rotate_secret"
	AuditEmailVerify            = "email_verify"
	AuditEmailResend            = "email_resend"
	AuditEmailChange            = "email_change"
	AuditPasswordSet            = "password_set"
	AuditInviteRewardGranted    = "invite_reward_granted"
	AuditInviteRewardSkipped    = "invite_reward_skipped"
	AuditSocialAccountLink      = "social_account_link"
	AuditSocialAccountUnlink    = "social_account_unlink"
	AuditOIDCConsentApprove     = "oidc_consent_approve"
	AuditOIDCConsentDeny        = "oidc_consent_deny"
	AuditOIDCConsentGrantUpsert = "oidc_consent_grant_upsert"
	AuditOIDCConsentGrantRevoke = "oidc_consent_grant_revoke"
	AuditOAuthClientTokenIssue  = "oauth_client_token_issue"
	AuditOAuthTokenRevoke       = "oauth_token_revoke"
)

// Audit source constants.
const (
	AuditSourceWeb    = "web"
	AuditSourceAPI    = "api"
	AuditSourceAdmin  = "admin"
	AuditSourceCLI    = "cli"
	AuditSourceSystem = "system"
)
