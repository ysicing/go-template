package store

import (
	"context"
	"slices"

	"github.com/ysicing/go-template/model"

	"gorm.io/gorm"
)

type SocialAccountStore struct {
	db *gorm.DB
}

func NewSocialAccountStore(db *gorm.DB) *SocialAccountStore {
	return &SocialAccountStore{db: db}
}

// CreateUserWithSocialAccount 在同一事务中创建用户和社交账号绑定。
func (s *SocialAccountStore) CreateUserWithSocialAccount(ctx context.Context, user *model.User, account *model.SocialAccount) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		account.UserID = user.ID
		return tx.Create(account).Error
	})
}

// GetByProviderAndID finds a social account by provider and provider ID.
func (s *SocialAccountStore) GetByProviderAndID(ctx context.Context, provider, providerID string) (*model.SocialAccount, error) {
	var account model.SocialAccount
	err := s.db.WithContext(ctx).Where("provider = ? AND provider_id = ?", provider, providerID).First(&account).Error
	return &account, normalizeNotFound(err)
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
	return &account, normalizeNotFound(err)
}
