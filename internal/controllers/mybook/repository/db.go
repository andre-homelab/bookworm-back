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

func (s *DBStore) ListByUser(ctx context.Context, userID string) ([]models.UserBook, error) {
	var userBooks []models.UserBook
	err := s.db.WithContext(ctx).
		Preload("Book").
		Where("user_id = ?", userID).
		Order("updated_at desc").
		Find(&userBooks).Error
	return userBooks, err
}

func (s *DBStore) Create(ctx context.Context, userBook *models.UserBook) error {
	return s.db.WithContext(ctx).Create(userBook).Error
}

func (s *DBStore) GetByIDAndUser(ctx context.Context, id, userID string) (*models.UserBook, error) {
	var userBook models.UserBook
	err := s.db.WithContext(ctx).
		Preload("Book").
		First(&userBook, "id = ? AND user_id = ?", id, userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &userBook, nil
}

func (s *DBStore) Update(ctx context.Context, userBook *models.UserBook) error {
	return s.db.WithContext(ctx).Save(userBook).Error
}

func (s *DBStore) DeleteByIDAndUser(ctx context.Context, id, userID string) error {
	return s.db.WithContext(ctx).Delete(&models.UserBook{}, "id = ? AND user_id = ?", id, userID).Error
}

func (s *DBStore) ExistsByUserAndBook(ctx context.Context, userID, bookID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&models.UserBook{}).
		Where("user_id = ? AND book_id = ?", userID, bookID).
		Count(&count).Error
	return count > 0, err
}

func (s *DBStore) GetBookByID(ctx context.Context, id string) (*models.Book, error) {
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
