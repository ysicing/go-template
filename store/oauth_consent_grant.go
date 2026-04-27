package store

import (
	"context"
	"errors"
	"sort"

	"github.com/ysicing/go-template/model"

	"gorm.io/gorm"
)

type OAuthConsentGrantStore struct {
	db *gorm.DB
}

func NewOAuthConsentGrantStore(db *gorm.DB) *OAuthConsentGrantStore {
	return &OAuthConsentGrantStore{db: db}
}

func (s *OAuthConsentGrantStore) GetByUserAndClient(ctx context.Context, userID, clientID string) (*model.OAuthConsentGrant, error) {
	var grant model.OAuthConsentGrant
	if err := s.db.WithContext(ctx).Where("user_id = ? AND client_id = ?", userID, clientID).First(&grant).Error; err != nil {
		return nil, normalizeNotFound(err)
	}
	return &grant, nil
}

func (s *OAuthConsentGrantStore) Upsert(ctx context.Context, grant *model.OAuthConsentGrant) error {
	existing, err := s.GetByUserAndClient(ctx, grant.UserID, grant.ClientID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if existing == nil || errors.Is(err, ErrNotFound) {
		grant.Scopes = normalizeScopes(grant.Scopes)
		return s.db.WithContext(ctx).Create(grant).Error
	}

	existing.Scopes = mergeScopeSets(existing.Scopes, grant.Scopes)
	return s.db.WithContext(ctx).Save(existing).Error
}

func (s *OAuthConsentGrantStore) HasGrantedScopes(ctx context.Context, userID, clientID string, requested []string) (bool, error) {
	grant, err := s.GetByUserAndClient(ctx, userID, clientID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	granted := SplitTrimmed(grant.Scopes)
	for _, scope := range requested {
		found := false
		for _, grantedScope := range granted {
			if grantedScope == scope {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	return true, nil
}

func (s *OAuthConsentGrantStore) ListByUserIDPaged(ctx context.Context, userID string, page, pageSize int) ([]model.OAuthConsentGrant, int64, error) {
	var grants []model.OAuthConsentGrant
	var total int64

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	query := s.db.WithContext(ctx).Model(&model.OAuthConsentGrant{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&grants).Error; err != nil {
		return nil, 0, err
	}

	return grants, total, nil
}

func (s *OAuthConsentGrantStore) DeleteByIDAndUserID(ctx context.Context, id, userID string) error {
	result := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.OAuthConsentGrant{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func normalizeScopes(raw string) string {
	scopes := SplitTrimmed(raw)
	sort.Strings(scopes)
	return joinUniqueScopes(scopes)
}

func mergeScopeSets(left, right string) string {
	scopes := append(SplitTrimmed(left), SplitTrimmed(right)...)
	sort.Strings(scopes)
	return joinUniqueScopes(scopes)
}

func joinUniqueScopes(scopes []string) string {
	if len(scopes) == 0 {
		return ""
	}
	unique := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if scope == "" {
			continue
		}
		if len(unique) == 0 || unique[len(unique)-1] != scope {
			unique = append(unique, scope)
		}
	}
	if len(unique) == 0 {
		return ""
	}
	result := unique[0]
	for _, scope := range unique[1:] {
		result += " " + scope
	}
	return result
}
