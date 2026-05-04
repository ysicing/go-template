package store

import (
	"context"
	"strings"
	"time"

	"github.com/ysicing/go-template/model"
)

// AppStat holds login count per OAuth client app.
type AppStat struct {
	ClientID                  string  `json:"client_id"`
	AppName                   string  `json:"app_name"`
	LoginCount                int64   `json:"login_count"`
	UserCount                 int64   `json:"user_count"`
	MachineTokenIssueCount    int64   `json:"machine_token_issue_count"`
	MachineTokenRevokeCount   int64   `json:"machine_token_revoke_count"`
	LastMachineTokenIssuedAt  *string `json:"last_machine_token_issued_at,omitempty"`
	LastMachineTokenRevokedAt *string `json:"last_machine_token_revoked_at,omitempty"`
}

type appStatRow struct {
	ClientID                string  `gorm:"column:client_id"`
	AppName                 string  `gorm:"column:app_name"`
	LoginCount              int64   `gorm:"column:login_count"`
	UserCount               int64   `gorm:"column:user_count"`
	MachineTokenIssueCount  int64   `gorm:"column:machine_token_issue_count"`
	MachineTokenRevokeCount int64   `gorm:"column:machine_token_revoke_count"`
	LastMachineIssuedAt     *string `gorm:"column:last_machine_token_issued_at"`
	LastMachineRevokedAt    *string `gorm:"column:last_machine_token_revoked_at"`
}

// AppLoginStats returns per-app login stats for the apps owned by a user.
func (s *AuditLogStore) AppLoginStats(ctx context.Context, userID string) ([]AppStat, error) {
	var rows []appStatRow
	err := s.db.WithContext(ctx).Raw(`
			SELECT oc.client_id, oc.name AS app_name,
			       COUNT(CASE WHEN al.action = ? THEN 1 END) AS login_count,
			       COUNT(DISTINCT CASE WHEN al.action = ? THEN al.user_id END) AS user_count,
			       COUNT(CASE WHEN al.action = ? THEN 1 END) AS machine_token_issue_count,
			       COUNT(CASE WHEN al.action = ? THEN 1 END) AS machine_token_revoke_count,
			       MAX(CASE WHEN al.action = ? THEN al.created_at END) AS last_machine_token_issued_at,
			       MAX(CASE WHEN al.action = ? THEN al.created_at END) AS last_machine_token_revoked_at
			FROM oauth_clients oc
			LEFT JOIN audit_logs al
			       ON al.client_id = oc.client_id AND al.deleted_at IS NULL
			WHERE oc.user_id = ? AND oc.deleted_at IS NULL
			GROUP BY oc.client_id, oc.name
			ORDER BY login_count DESC
		`,
		model.AuditLogin,
		model.AuditLogin,
		model.AuditOAuthClientTokenIssue,
		model.AuditOAuthTokenRevoke,
		model.AuditOAuthClientTokenIssue,
		model.AuditOAuthTokenRevoke,
		userID,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	stats := make([]AppStat, 0, len(rows))
	for _, row := range rows {
		stats = append(stats, AppStat{
			ClientID:                  row.ClientID,
			AppName:                   row.AppName,
			LoginCount:                row.LoginCount,
			UserCount:                 row.UserCount,
			MachineTokenIssueCount:    row.MachineTokenIssueCount,
			MachineTokenRevokeCount:   row.MachineTokenRevokeCount,
			LastMachineTokenIssuedAt:  normalizeAppStatTimestamp(row.LastMachineIssuedAt),
			LastMachineTokenRevokedAt: normalizeAppStatTimestamp(row.LastMachineRevokedAt),
		})
	}
	return stats, nil
}

func formatAppStatTimestamp(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func normalizeAppStatTimestamp(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return formatAppStatTimestamp(&parsed)
		}
	}

	return &trimmed
}
