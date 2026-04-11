package store

import (
	"context"
	"slices"

	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
)

type SocialAccountStore struct {
	db *gorm.DB
}

func NewSocialAccountStore(db *gorm.DB) *SocialAccountStore {
	return &SocialAccountStore{db: db}
}

// GetByProviderAndID finds a social account by provider and provider ID.
func (s *SocialAccountStore) GetByProviderAndID(ctx context.Context, provider, providerID string) (*model.SocialAccount, error) {
	var account model.SocialAccount
	err := s.db.WithContext(ctx).Where("provider = ? AND provider_id = ?", provider, providerID).First(&account).Error
	return &account, err
}

// ListByUserID returns all social accounts for a user.
func (s *SocialAccountStore) ListByUserID(ctx context.Context, userID string) ([]*model.SocialAccount, error) {
	var accounts []*model.SocialAccount
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&accounts).Error
	return accounts, err
}

func (s *SocialAccountStore) ListProvidersByUserIDs(ctx context.Context, userIDs []string) (map[string][]string, error) {
	result := make(map[string][]string, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}

	var accounts []model.SocialAccount
	if err := s.db.WithContext(ctx).
		Select("user_id", "provider").
		Where("user_id IN ?", userIDs).
		Order("provider ASC").
		Find(&accounts).Error; err != nil {
		return nil, err
	}

	for _, account := range accounts {
		providers := result[account.UserID]
		if !slices.Contains(providers, account.Provider) {
			result[account.UserID] = append(providers, account.Provider)
		}
	}

	return result, nil
}

// Create creates a new social account binding.
func (s *SocialAccountStore) Create(ctx context.Context, account *model.SocialAccount) error {
	return s.db.WithContext(ctx).Create(account).Error
}

// Update updates a social account.
func (s *SocialAccountStore) Update(ctx context.Context, account *model.SocialAccount) error {
	return s.db.WithContext(ctx).Save(account).Error
}

// Delete deletes a social account binding.
func (s *SocialAccountStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&model.SocialAccount{}, "id = ?", id).Error
}

// DeleteByUserIDAndProvider deletes a specific social account binding.
func (s *SocialAccountStore) DeleteByUserIDAndProvider(ctx context.Context, userID, provider string) error {
	return s.db.WithContext(ctx).Where("user_id = ? AND provider = ?", userID, provider).Delete(&model.SocialAccount{}).Error
}

func (s *SocialAccountStore) GetByProviderForUser(ctx context.Context, userID, provider string) (*model.SocialAccount, error) {
	var account model.SocialAccount
	err := s.db.WithContext(ctx).Where("user_id = ? AND provider = ?", userID, provider).First(&account).Error
	return &account, err
}
