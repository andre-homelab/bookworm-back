package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/models"
	"gorm.io/gorm"
)

type DBStore struct {
	db *gorm.DB
}

func NewDBStore(db *gorm.DB) *DBStore {
	return &DBStore{db: db}
}

func (s *DBStore) Create(ctx context.Context, user *models.User) error {
	return s.db.WithContext(ctx).Create(user).Error
}

func (s *DBStore) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User

	err := s.db.WithContext(ctx).First(&user, "LOWER(email) = LOWER(?)", strings.TrimSpace(email)).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *DBStore) GetByID(ctx context.Context, id string) (*models.User, error) {
	var user models.User

	err := s.db.WithContext(ctx).First(&user, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *DBStore) UpdateProfile(ctx context.Context, id, name, email string) error {
	return s.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"name":       name,
			"email":      strings.ToLower(strings.TrimSpace(email)),
			"updated_at": time.Now().UTC(),
		}).Error
}

func (s *DBStore) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	return s.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"password_hash": passwordHash,
			"updated_at":    time.Now().UTC(),
		}).Error
}

func (s *DBStore) UpdateLastLogin(ctx context.Context, id string, loginAt time.Time) error {
	return s.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"last_login_at": loginAt,
			"updated_at":    time.Now().UTC(),
		}).Error
}
