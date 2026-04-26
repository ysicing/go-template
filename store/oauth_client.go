package store

import (
	"context"

	"github.com/ysicing/go-template/model"

	"gorm.io/gorm"
)

// OAuthClientStore handles persistence for OAuth2 client applications.
type OAuthClientStore struct {
	db *gorm.DB
}

// NewOAuthClientStore creates an OAuthClientStore.
func NewOAuthClientStore(db *gorm.DB) *OAuthClientStore {
	return &OAuthClientStore{db: db}
}

// Create persists a new OAuth client.
func (s *OAuthClientStore) Create(ctx context.Context, client *model.OAuthClient) error {
	return s.db.WithContext(ctx).Create(client).Error
}

// GetByID retrieves an OAuth client by primary key.
func (s *OAuthClientStore) GetByID(ctx context.Context, id string) (*model.OAuthClient, error) {
	var client model.OAuthClient
	if err := s.db.WithContext(ctx).First(&client, "id = ?", id).Error; err != nil {
		return nil, normalizeNotFound(err)
	}
	return &client, nil
}

// GetByClientID retrieves an OAuth client by its client_id field.
func (s *OAuthClientStore) GetByClientID(ctx context.Context, clientID string) (*model.OAuthClient, error) {
	var client model.OAuthClient
	if err := s.db.WithContext(ctx).Where("client_id = ?", clientID).First(&client).Error; err != nil {
		return nil, normalizeNotFound(err)
	}
	return &client, nil
}

// IssueClientAccessToken 在同一事务中写入客户端访问令牌和审计日志。
func (s *OAuthClientStore) IssueClientAccessToken(ctx context.Context, token *model.Token, auditLog *model.AuditLog) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(token).Error; err != nil {
			return err
		}
		return tx.Create(auditLog).Error
	})
}

// RevokeClientAccessToken 在同一事务中删除客户端访问令牌并写入审计日志。
func (s *OAuthClientStore) RevokeClientAccessToken(ctx context.Context, tokenID string, auditLog *model.AuditLog) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Delete(&model.Token{}, "id = ?", tokenID).Error; err != nil {
			return err
		}
		return tx.Create(auditLog).Error
	})
}

// FindClientPrincipalToken 返回客户端主体访问令牌，不匹配客户端主体时按未找到处理。
func (s *OAuthClientStore) FindClientPrincipalToken(ctx context.Context, tokenValue string) (*model.Token, error) {
	var token model.Token
	if err := s.db.WithContext(ctx).Where("token_id = ?", tokenValue).First(&token).Error; err != nil {
		return nil, normalizeNotFound(err)
	}
	if token.SubjectType != "oauth_client" {
		return nil, ErrNotFound
	}
	return &token, nil
}

// Update saves changes to an existing OAuth client.
func (s *OAuthClientStore) Update(ctx context.Context, client *model.OAuthClient) error {
	return s.db.WithContext(ctx).Save(client).Error
}

// Delete soft-deletes an OAuth client by primary key.
func (s *OAuthClientStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.OAuthClient{}).Error
}

// Count returns total number of OAuth clients.
func (s *OAuthClientStore) Count(ctx context.Context) (int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).Model(&model.OAuthClient{}).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// List returns a paginated list of OAuth clients and the total count.
func (s *OAuthClientStore) List(ctx context.Context, page, pageSize int) ([]model.OAuthClient, int64, error) {
	var clients []model.OAuthClient
	var total int64

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	d := s.db.WithContext(ctx)
	if err := d.Model(&model.OAuthClient{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := d.Offset(offset).Limit(pageSize).Find(&clients).Error; err != nil {
		return nil, 0, err
	}

	return clients, total, nil
}
