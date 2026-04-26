package store

import (
	"errors"

	"gorm.io/gorm"
)

var ErrNotFound = errors.New("store: not found")

func normalizeNotFound(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
