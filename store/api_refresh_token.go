package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/ysicing/go-template/model"
)

// APIRefreshTokenStore handles persistence for API refresh tokens.
type APIRefreshTokenStore struct {
	db    *gorm.DB
	cache Cache
}

// NewAPIRefreshTokenStore creates an APIRefreshTokenStore.
func NewAPIRefreshTokenStore(db *gorm.DB, cache ...Cache) *APIRefreshTokenStore {
	store := &APIRefreshTokenStore{db: db}
	if len(cache) > 0 {
		store.cache = cache[0]
	}
	return store
}

func refreshTokenUsedKey(hash string) string { return "rt_used:" + hash }

func refreshTokenUsedTTL(expiresAt time.Time) time.Duration {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return 0
	}
	return ttl
}

func (s *APIRefreshTokenStore) rememberUsedToken(ctx context.Context, rt *model.APIRefreshToken) {
	if s.cache == nil || rt == nil || rt.TokenHash == "" || rt.Family == "" {
		return
	}
	ttl := refreshTokenUsedTTL(rt.ExpiresAt)
	if ttl <= 0 {
		return
	}
	_ = s.cache.Set(ctx, refreshTokenUsedKey(rt.TokenHash), rt.Family, ttl)
}

// GetUsedFamily returns the cached token family for a consumed refresh token.
func (s *APIRefreshTokenStore) GetUsedFamily(ctx context.Context, hash string) (string, error) {
	if s.cache == nil {
		return "", ErrCacheMiss
	}
	return s.cache.Get(ctx, refreshTokenUsedKey(hash))
}

// HashToken returns SHA-256 hex hash of a token string.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// Create stores a new refresh token (pass the hash, not plaintext).
func (s *APIRefreshTokenStore) Create(ctx context.Context, rt *model.APIRefreshToken) error {
	return s.db.WithContext(ctx).Create(rt).Error
}

// GetByTokenHash retrieves a refresh token entry by its hash.
func (s *APIRefreshTokenStore) GetByTokenHash(ctx context.Context, hash string) (*model.APIRefreshToken, error) {
	var rt model.APIRefreshToken
	if err := s.db.WithContext(ctx).Where("token_hash = ?", hash).First(&rt).Error; err != nil {
		return nil, err
	}
	return &rt, nil
}

// DeleteByTokenHash removes a specific refresh token by hash.
func (s *APIRefreshTokenStore) DeleteByTokenHash(ctx context.Context, hash string) error {
	return s.db.WithContext(ctx).Where("token_hash = ?", hash).Delete(&model.APIRefreshToken{}).Error
}

// ConsumeToken atomically retrieves and deletes a refresh token by hash.
// Returns the token if found and successfully deleted, or an error if not found or already consumed.
// This prevents concurrent refresh requests from reusing the same token.
func (s *APIRefreshTokenStore) ConsumeToken(ctx context.Context, hash string) (*model.APIRefreshToken, error) {
	var rt model.APIRefreshToken
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the row for update to prevent concurrent consumption
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ?", hash).First(&rt).Error; err != nil {
			return err
		}
		// Delete the token within the same transaction
		if err := tx.Delete(&rt).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.rememberUsedToken(ctx, &rt)
	return &rt, nil
}

// DeleteByUserID removes all refresh tokens for a user (used on logout).
func (s *APIRefreshTokenStore) DeleteByUserID(ctx context.Context, userID string) error {
	return s.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&model.APIRefreshToken{}).Error
}

// DeleteExpired removes all expired refresh tokens.
func (s *APIRefreshTokenStore) DeleteExpired(ctx context.Context) error {
	return s.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&model.APIRefreshToken{}).Error
}

// ListByUserID returns all active refresh tokens for a user (sessions).
func (s *APIRefreshTokenStore) ListByUserID(ctx context.Context, userID string) ([]model.APIRefreshToken, error) {
	var tokens []model.APIRefreshToken
	err := s.db.WithContext(ctx).Where("user_id = ? AND expires_at > ?", userID, time.Now()).
		Order("created_at DESC").Find(&tokens).Error
	return tokens, err
}

// ListByUserIDPaged returns paginated active refresh tokens for a user and the total count.
func (s *APIRefreshTokenStore) ListByUserIDPaged(ctx context.Context, userID string, page, pageSize int) ([]model.APIRefreshToken, int64, error) {
	var tokens []model.APIRefreshToken
	var total int64
	q := s.db.WithContext(ctx).Model(&model.APIRefreshToken{}).
		Where("user_id = ? AND expires_at > ?", userID, time.Now())
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tokens).Error
	return tokens, total, err
}

// DeleteByID removes a specific refresh token by ID.
func (s *APIRefreshTokenStore) DeleteByID(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.APIRefreshToken{}).Error
}

// DeleteByIDAndUserID removes a specific refresh token only if it belongs to the user.
func (s *APIRefreshTokenStore) DeleteByIDAndUserID(ctx context.Context, id, userID string) error {
	result := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.APIRefreshToken{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteByUserIDExcept removes all refresh tokens for a user except the given ID.
func (s *APIRefreshTokenStore) DeleteByUserIDExcept(ctx context.Context, userID, exceptID string) error {
	return s.db.WithContext(ctx).Where("user_id = ? AND id != ?", userID, exceptID).Delete(&model.APIRefreshToken{}).Error
}

// UpdateLastUsed updates the last_used_at timestamp for a refresh token.
func (s *APIRefreshTokenStore) UpdateLastUsed(ctx context.Context, hash string) {
	_ = s.db.WithContext(ctx).Model(&model.APIRefreshToken{}).
		Where("token_hash = ?", hash).
		Update("last_used_at", time.Now()).Error
}

// DeleteByFamily removes all refresh tokens in a token family (replay detection).
func (s *APIRefreshTokenStore) DeleteByFamily(ctx context.Context, family string) error {
	if family == "" {
		return nil
	}
	return s.db.WithContext(ctx).Where("family = ?", family).Delete(&model.APIRefreshToken{}).Error
}

// GenerateRandomToken creates a cryptographically random hex token.
func GenerateRandomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
