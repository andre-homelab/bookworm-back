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

func (s *DBStore) Create(ctx context.Context, book *models.Book) error {
	return s.db.WithContext(ctx).Create(book).Error
}

func (s *DBStore) ListAll(ctx context.Context) ([]models.Book, error) {
	var books []models.Book
	err := s.db.WithContext(ctx).
		Order("created_at desc").
		Find(&books).Error

	return books, err
}

func (s *DBStore) FindByISBN(ctx context.Context, isbn string) (*models.Book, error) {
	var book models.Book

	err := s.db.WithContext(ctx).First(&book, "isbn = ?", isbn).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &book, nil
}

func (s *DBStore) SearchByTitleOrAuthor(ctx context.Context, query string, limit int) ([]models.Book, error) {
	if limit <= 0 {
		limit = 20
	}

	var books []models.Book
	search := "%" + strings.TrimSpace(query) + "%"

	err := s.db.WithContext(ctx).
		Where("title ILIKE ? OR author ILIKE ?", search, search).
		Order("updated_at desc").
		Limit(limit).
		Find(&books).Error

	return books, err
}

func (s *DBStore) UpsertFromExternal(ctx context.Context, book *models.Book) error {
	now := time.Now().UTC()
	book.CachedAt = &now

	existing, err := s.FindByISBN(ctx, book.ISBN)
	if err != nil {
		return err
	}

	if existing == nil {
		return s.Create(ctx, book)
	}

	updates := map[string]any{
		"title":          book.Title,
		"author":         book.Author,
		"description":    book.Description,
		"cover_url":      book.CoverURL,
		"publisher":      book.Publisher,
		"published_year": book.PublishedYear,
		"pages":          book.Pages,
		"cache_source":   book.CacheSource,
		"cached_at":      book.CachedAt,
		"updated_at":     now,
	}

	return s.db.WithContext(ctx).
		Model(&models.Book{}).
		Where("id = ?", existing.ID).
		Updates(updates).Error
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

func (s *DBStore) Update(ctx context.Context, book *models.Book) error {
	return s.db.WithContext(ctx).Save(book).Error
}

func (s *DBStore) DeleteByID(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.Book{}, "id = ?", id).Error
}
