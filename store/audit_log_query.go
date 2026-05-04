package store

import (
	"context"
	"strings"
	"time"

	"github.com/ysicing/go-template/model"
)

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
			       '' AS app_name,
			       al.detail AS provider, al.ip, al.user_agent, al.created_at
			FROM audit_logs al
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
			       '' AS app_name,
			       al.detail AS provider, al.ip, al.user_agent, al.created_at
			FROM audit_logs al
			LEFT JOIN users u ON u.id = al.user_id AND u.deleted_at IS NULL
			WHERE al.action = ? AND al.deleted_at IS NULL
			ORDER BY al.created_at DESC
			LIMIT ? OFFSET ?
		`, model.AuditLogin, pageSize, (page-1)*pageSize).Scan(&rows).Error
	return rows, total, err
}
