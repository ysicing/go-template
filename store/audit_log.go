package store

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
)

// AuditLogStore handles persistence for audit logs.
type AuditLogStore struct {
	db *gorm.DB
}

// NewAuditLogStore creates an AuditLogStore.
func NewAuditLogStore(db *gorm.DB) *AuditLogStore {
	return &AuditLogStore{db: db}
}

// Create stores a new audit log entry.
func (s *AuditLogStore) Create(ctx context.Context, log *model.AuditLog) error {
	return s.db.WithContext(ctx).Create(log).Error
}

// ListByUser returns audit logs for a specific user.
func (s *AuditLogStore) ListByUser(ctx context.Context, userID string, page, pageSize int) ([]model.AuditLog, int64, error) {
	var logs []model.AuditLog
	var total int64
	q := s.db.WithContext(ctx).Model(&model.AuditLog{}).Where("user_id = ?", userID)
	q.Count(&total)
	err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

// --- Login statistics (replaces LoginEventStore) ---

// CountLogin returns the total number of login events.
func (s *AuditLogStore) CountLogin(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&model.AuditLog{}).
		Where("action = ?", model.AuditLogin).Count(&count).Error
	return count, err
}

// CountLoginToday returns today's login count.
func (s *AuditLogStore) CountLoginToday(ctx context.Context) (int64, error) {
	var count int64
	today := time.Now().Truncate(24 * time.Hour)
	err := s.db.WithContext(ctx).Model(&model.AuditLog{}).
		Where("action = ? AND created_at >= ?", model.AuditLogin, today).Count(&count).Error
	return count, err
}

// CountLoginByUserID returns total login count for a specific user.
func (s *AuditLogStore) CountLoginByUserID(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&model.AuditLog{}).
		Where("user_id = ? AND action = ?", userID, model.AuditLogin).Count(&count).Error
	return count, err
}

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

// LoginRow is a flattened row returned by paginated login history queries.
type LoginRow struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username,omitempty"`
	ClientID  string    `json:"client_id"`
	AppName   string    `json:"app_name"`
	Provider  string    `json:"provider"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditLogFilter struct {
	UserID      string
	Action      string
	Resource    string
	Source      string
	Status      string
	IP          string
	Keyword     string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

type AuditLogRow struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Username   string    `json:"username,omitempty"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resource_id"`
	ClientID   string    `json:"client_id"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	Detail     string    `json:"detail"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	Source     string    `json:"source"`
}

func (s *AuditLogStore) ListAuditLogsPaged(ctx context.Context, filter AuditLogFilter, page, pageSize int) ([]AuditLogRow, int64, error) {
	query := s.db.WithContext(ctx).
		Table("audit_logs AS al").
		Joins("LEFT JOIN users u ON u.id = al.user_id AND u.deleted_at IS NULL").
		Where("al.deleted_at IS NULL")

	if filter.UserID != "" {
		query = query.Where("al.user_id = ?", filter.UserID)
	}
	if filter.Action != "" {
		query = query.Where("al.action = ?", filter.Action)
	}
	if filter.Resource != "" {
		query = query.Where("al.resource = ?", filter.Resource)
	}
	if filter.Status != "" {
		query = query.Where("al.status = ?", filter.Status)
	}
	if filter.IP != "" {
		query = query.Where("al.ip = ?", filter.IP)
	}
	if filter.Source != "" {
		query = query.Where("LOWER(al.detail) LIKE ?", "source="+strings.ToLower(filter.Source)+"%")
	}
	if filter.CreatedFrom != nil {
		query = query.Where("al.created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("al.created_at <= ?", *filter.CreatedTo)
	}
	if keyword := strings.ToLower(strings.TrimSpace(filter.Keyword)); keyword != "" {
		pattern := "%" + keyword + "%"
		query = query.Where(`
			LOWER(COALESCE(u.username, '')) LIKE ?
			OR LOWER(COALESCE(al.user_id, '')) LIKE ?
			OR LOWER(COALESCE(al.action, '')) LIKE ?
			OR LOWER(COALESCE(al.resource, '')) LIKE ?
			OR LOWER(COALESCE(al.resource_id, '')) LIKE ?
			OR LOWER(COALESCE(al.ip, '')) LIKE ?
			OR LOWER(COALESCE(al.user_agent, '')) LIKE ?
			OR LOWER(COALESCE(al.detail, '')) LIKE ?
		`, pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []AuditLogRow
	if err := query.
		Select(`
			al.id,
			al.user_id,
			COALESCE(u.username, '') AS username,
			al.action,
			al.resource,
			al.resource_id,
			al.client_id,
			al.ip,
			al.user_agent,
			al.detail,
			al.status,
			al.created_at
		`).
		Order("al.created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	for i := range rows {
		rows[i].Source = extractAuditSource(rows[i].Detail)
	}

	return rows, total, nil
}

func extractAuditSource(detail string) string {
	for _, part := range strings.Fields(detail) {
		if source, ok := strings.CutPrefix(part, "source="); ok && source != "" {
			return source
		}
	}
	return ""
}

// ListLoginByUserIDPaged returns paginated login history for a single user.
func (s *AuditLogStore) ListLoginByUserIDPaged(ctx context.Context, userID string, page, pageSize int) ([]LoginRow, int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).Model(&model.AuditLog{}).
		Where("user_id = ? AND action = ?", userID, model.AuditLogin).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []LoginRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT al.id, al.user_id, al.client_id,
		       COALESCE(oc.name, '') AS app_name,
		       al.detail AS provider, al.ip, al.user_agent, al.created_at
		FROM audit_logs al
		LEFT JOIN oauth_clients oc ON oc.client_id = al.client_id AND oc.deleted_at IS NULL
		WHERE al.user_id = ? AND al.action = ? AND al.deleted_at IS NULL
		ORDER BY al.created_at DESC
		LIMIT ? OFFSET ?
	`, userID, model.AuditLogin, pageSize, (page-1)*pageSize).Scan(&rows).Error
	return rows, total, err
}

// ListLoginAllPaged returns paginated login history for all users (admin view).
func (s *AuditLogStore) ListLoginAllPaged(ctx context.Context, page, pageSize int) ([]LoginRow, int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).Model(&model.AuditLog{}).
		Where("action = ?", model.AuditLogin).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []LoginRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT al.id, al.user_id,
		       COALESCE(u.username, al.user_id) AS username,
		       al.client_id,
		       COALESCE(oc.name, '') AS app_name,
		       al.detail AS provider, al.ip, al.user_agent, al.created_at
		FROM audit_logs al
		LEFT JOIN users u ON u.id = al.user_id AND u.deleted_at IS NULL
		LEFT JOIN oauth_clients oc ON oc.client_id = al.client_id AND oc.deleted_at IS NULL
		WHERE al.action = ? AND al.deleted_at IS NULL
		ORDER BY al.created_at DESC
		LIMIT ? OFFSET ?
	`, model.AuditLogin, pageSize, (page-1)*pageSize).Scan(&rows).Error
	return rows, total, err
}
