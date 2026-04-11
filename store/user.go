package store

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/ysicing/go-template/model"
)

type UserStore struct {
	db *gorm.DB
}

func NewUserStore(db *gorm.DB) *UserStore {
	return &UserStore{db: db}
}

func (s *UserStore) Create(ctx context.Context, user *model.User) error {
	return s.db.WithContext(ctx).Create(user).Error
}

func (s *UserStore) GetByID(ctx context.Context, id string) (*model.User, error) {
	var user model.User
	if err := s.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UserStore) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UserStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	if err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByUsernameOrEmail returns a user matching the given identity, preferring username match.
// This avoids ambiguity when a user's email matches another user's username.
func (s *UserStore) GetByUsernameOrEmail(ctx context.Context, identity string) (*model.User, error) {
	var user model.User
	// Keep single query while guaranteeing username match wins over email match.
	result := s.db.WithContext(ctx).
		Where("username = ? OR email = ?", identity, identity).
		Order(clause.Expr{SQL: "CASE WHEN username = ? THEN 0 ELSE 1 END", Vars: []any{identity}}).
		Limit(1).
		Find(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &user, nil
}

func (s *UserStore) GetByInviteCode(ctx context.Context, inviteCode string) (*model.User, error) {
	var user model.User
	if err := s.db.WithContext(ctx).Where("invite_code = ?", inviteCode).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UserStore) GetByProviderID(ctx context.Context, provider, providerID string) (*model.User, error) {
	var user model.User
	if err := s.db.WithContext(ctx).Where("provider = ? AND provider_id = ?", provider, providerID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UserStore) Update(ctx context.Context, user *model.User) error {
	return s.db.WithContext(ctx).Save(user).Error
}

func (s *UserStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.User{}).Error
}

// Count returns total number of users.
func (s *UserStore) Count(ctx context.Context) (int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).Model(&model.User{}).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (s *UserStore) List(ctx context.Context, page, pageSize int) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	d := s.db.WithContext(ctx)
	if err := d.Model(&model.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := d.Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// ChangePasswordWithHistory changes user password and records the previous hash in a transaction.
func (s *UserStore) ChangePasswordWithHistory(ctx context.Context, user *model.User, previousPasswordHash string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Save previous password to history (must be done before updating user.PasswordHash)
		if previousPasswordHash != "" {
			if err := tx.Create(&model.PasswordHistory{
				UserID:       user.ID,
				PasswordHash: previousPasswordHash,
			}).Error; err != nil {
				return err
			}
		}
		// user.PasswordHash is already set to the new hash by caller
		return tx.Save(user).Error
	})
}
