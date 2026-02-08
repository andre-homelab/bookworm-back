package repository

import (
	"context"
	"errors"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/models"
	"gorm.io/gorm"
)

type DBStore struct {
	db *gorm.DB
}

func NewDBStore(db *gorm.DB) *DBStore {
	return &DBStore{db: db}
}

func (s *DBStore) Create(ctx context.Context, book *models.Book) error {
	return s.db.WithContext(ctx).Create(book).Error
}

func (s *DBStore) GetByID(ctx context.Context, id string) (*models.Book, error) {
	var book models.Book

	err := s.db.WithContext(ctx).First(&book, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &book, nil
}
