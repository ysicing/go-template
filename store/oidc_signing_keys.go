package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/crypto"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/op"
	"gorm.io/gorm"
)

func (s *OIDCStorage) loadOrCreateSigningKey(ctx context.Context) error {
	var sk model.SigningKey
	if err := s.db.WithContext(ctx).Order("created_at desc").First(&sk).Error; err == nil {
		return s.parseSigningKey(&sk)
	}
	return s.rotateSigningKey(ctx, time.Now())
}

func (s *OIDCStorage) rotateSigningKey(ctx context.Context, now time.Time) error {
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return fmt.Errorf("generate rsa key: %w", err)
	}

	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	pemBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes}
	pemData := string(pem.EncodeToMemory(pemBlock))

	storedKey := pemData
	if s.encPassphrase != "" {
		encrypted, err := crypto.Encrypt(s.encPassphrase, pemData)
		if err != nil {
			return fmt.Errorf("encrypt signing key: %w", err)
		}
		storedKey = encrypted
	}

	newSigningKey := model.SigningKey{
		ID:         uuid.New().String(),
		Algorithm:  string(jose.RS256),
		PrivateKey: storedKey,
	}
	if err := s.db.WithContext(ctx).Create(&newSigningKey).Error; err != nil {
		return fmt.Errorf("store signing key: %w", err)
	}

	s.signingMu.Lock()
	s.signingKey = signingKeyData{id: newSigningKey.ID, algorithm: jose.RS256, privateKey: key}
	s.signingMu.Unlock()

	if err := s.db.WithContext(ctx).Where("created_at < ?", now.Add(-oidcSigningKeyRetainDuration)).Delete(&model.SigningKey{}).Error; err != nil {
		return fmt.Errorf("cleanup old signing keys: %w", err)
	}
	return nil
}

func (s *OIDCStorage) ensureSigningKeyFresh(ctx context.Context, now time.Time) error {
	s.signingMu.RLock()
	currentID := s.signingKey.id
	s.signingMu.RUnlock()
	if currentID == "" {
		return s.loadOrCreateSigningKey(ctx)
	}

	var current model.SigningKey
	if err := s.db.WithContext(ctx).Where("id = ?", currentID).First(&current).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.loadOrCreateSigningKey(ctx)
		}
		return fmt.Errorf("load signing key metadata: %w", err)
	}

	if current.CreatedAt.IsZero() {
		seeded := now.Add(-oidcSigningKeyRotationInterval - time.Minute)
		if err := s.db.WithContext(ctx).Model(&model.SigningKey{}).Where("id = ?", current.ID).Update("created_at", seeded).Error; err != nil {
			return fmt.Errorf("seed legacy signing key created_at: %w", err)
		}
		current.CreatedAt = seeded
	}
	if now.Sub(current.CreatedAt) < oidcSigningKeyRotationInterval {
		return nil
	}
	return s.rotateSigningKey(ctx, now)
}

func (s *OIDCStorage) parseSigningKey(sk *model.SigningKey) error {
	pemStr := sk.PrivateKey
	if s.encPassphrase != "" {
		decrypted, err := crypto.DecryptOrPlaintext(s.encPassphrase, pemStr)
		if err != nil {
			return fmt.Errorf("decrypt signing key: %w", err)
		}
		pemStr = decrypted
	}
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return fmt.Errorf("failed to decode PEM block")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}
	s.signingMu.Lock()
	s.signingKey = signingKeyData{id: sk.ID, algorithm: jose.SignatureAlgorithm(sk.Algorithm), privateKey: key}
	s.signingMu.Unlock()
	return nil
}

func (s *OIDCStorage) parsePublicKey(sk *model.SigningKey) (op.Key, error) {
	pemStr := sk.PrivateKey
	if s.encPassphrase != "" {
		decrypted, err := crypto.DecryptOrPlaintext(s.encPassphrase, pemStr)
		if err != nil {
			return nil, fmt.Errorf("decrypt signing key: %w", err)
		}
		pemStr = decrypted
	}
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	return &publicKeyData{id: sk.ID, algorithm: jose.SignatureAlgorithm(sk.Algorithm), key: &privateKey.PublicKey}, nil
}

func (s *OIDCStorage) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		now := time.Now()
		_ = s.ensureSigningKeyFresh(ctx, now)
		s.db.WithContext(ctx).Where("expires_at < ?", now).Delete(&model.Token{})
	}
}

func (s *OIDCStorage) SigningKey(ctx context.Context) (op.SigningKey, error) {
	if err := s.ensureSigningKeyFresh(ctx, time.Now()); err != nil {
		return nil, err
	}
	s.signingMu.RLock()
	key := s.signingKey
	s.signingMu.RUnlock()
	return &key, nil
}

func (s *OIDCStorage) SignatureAlgorithms(ctx context.Context) ([]jose.SignatureAlgorithm, error) {
	if err := s.ensureSigningKeyFresh(ctx, time.Now()); err != nil {
		return nil, err
	}
	s.signingMu.RLock()
	algorithm := s.signingKey.algorithm
	s.signingMu.RUnlock()
	return []jose.SignatureAlgorithm{algorithm}, nil
}

func (s *OIDCStorage) KeySet(ctx context.Context) ([]op.Key, error) {
	if err := s.ensureSigningKeyFresh(ctx, time.Now()); err != nil {
		return nil, err
	}

	var signingKeys []model.SigningKey
	if err := s.db.WithContext(ctx).
		Where("created_at >= ?", time.Now().Add(-oidcSigningKeyRetainDuration)).
		Order("created_at desc").
		Find(&signingKeys).Error; err != nil {
		return nil, fmt.Errorf("load signing key set: %w", err)
	}

	keys := make([]op.Key, 0, len(signingKeys))
	for i := range signingKeys {
		pub, err := s.parsePublicKey(&signingKeys[i])
		if err != nil {
			return nil, err
		}
		keys = append(keys, pub)
	}
	if len(keys) == 0 {
		s.signingMu.RLock()
		current := s.signingKey
		s.signingMu.RUnlock()
		if current.privateKey != nil {
			keys = append(keys, &publicKeyData{id: current.id, algorithm: current.algorithm, key: &current.privateKey.PublicKey})
		}
	}
	return keys, nil
}
