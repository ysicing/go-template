package store

import (
	"context"

	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
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
		return nil, err
	}
	return &client, nil
}

// GetByClientID retrieves an OAuth client by its client_id field.
func (s *OAuthClientStore) GetByClientID(ctx context.Context, clientID string) (*model.OAuthClient, error) {
	var client model.OAuthClient
	if err := s.db.WithContext(ctx).Where("client_id = ?", clientID).First(&client).Error; err != nil {
		return nil, err
	}
	return &client, nil
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

// ListByUserID returns a paginated list of personal OAuth clients owned by the given user.
func (s *OAuthClientStore) ListByUserID(ctx context.Context, userID string, page, pageSize int) ([]model.OAuthClient, int64, error) {
	var clients []model.OAuthClient
	var total int64

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	q := s.db.WithContext(ctx).
		Model(&model.OAuthClient{}).
		Where("user_id = ? AND (organization_id = '' OR organization_id IS NULL)", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := q.Offset(offset).Limit(pageSize).Find(&clients).Error; err != nil {
		return nil, 0, err
	}

	return clients, total, nil
}

// GetByIDAndUserID retrieves a personal OAuth client only if it belongs to the given user.
func (s *OAuthClientStore) GetByIDAndUserID(ctx context.Context, id, userID string) (*model.OAuthClient, error) {
	var client model.OAuthClient
	if err := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND (organization_id = '' OR organization_id IS NULL)", id, userID).
		First(&client).Error; err != nil {
		return nil, err
	}
	return &client, nil
}

// DeleteByIDAndUserID soft-deletes a personal OAuth client only if it belongs to the given user.
func (s *OAuthClientStore) DeleteByIDAndUserID(ctx context.Context, id, userID string) error {
	result := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND (organization_id = '' OR organization_id IS NULL)", id, userID).
		Delete(&model.OAuthClient{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
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
