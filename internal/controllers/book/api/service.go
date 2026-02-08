package api

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/book/cache"
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/book/provider"
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/models"
	"golang.org/x/sync/singleflight"
)

var (
	ErrInvalidInput = errors.New("dados de entrada inválidos")
	ErrBookNotFound = errors.New("livro não encontrado")
)

type Store interface {
	Create(ctx context.Context, book *models.Book) error
	GetByID(ctx context.Context, id string) (*models.Book, error)
	FindByISBN(ctx context.Context, isbn string) (*models.Book, error)
	SearchByTitleOrAuthor(ctx context.Context, query string, limit int) ([]models.Book, error)
	UpsertFromExternal(ctx context.Context, book *models.Book) error
}

type SearchSource string

const (
	SearchSourceMemory   SearchSource = "RAM"
	SearchSourceDatabase SearchSource = "POSTGRES"
	SearchSourceExternal SearchSource = "EXTERNAL"
)

type Service struct {
	repo       Store
	cache      cache.MemoryCache
	provider   provider.ExternalBookProvider
	sfByISBN   singleflight.Group
	sfBySearch singleflight.Group
}

func NewService(repo Store, memoryCache cache.MemoryCache, extProvider provider.ExternalBookProvider) *Service {
	return &Service{repo: repo, cache: memoryCache, provider: extProvider}
}

func (s *Service) Create(
	ctx context.Context,
	title string,
	author string,
	isbn string,
	pages int,
	read bool,
	finishedAt *time.Time,
) (*models.Book, error) {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(author) == "" || pages < 0 {
		return nil, ErrInvalidInput
	}

	now := time.Now().UTC()
	book := models.Book{
		Title:      strings.TrimSpace(title),
		Author:     strings.TrimSpace(author),
		ISBN:       strings.TrimSpace(isbn),
		Pages:      pages,
		Read:       read,
		FinishedAt: finishedAt,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.repo.Create(ctx, &book); err != nil {
		return nil, err
	}

	created, err := s.repo.GetByID(ctx, book.ID)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, ErrBookNotFound
	}

	return created, nil
}

func (s *Service) SearchByISBN(ctx context.Context, isbn string) (*models.Book, SearchSource, error) {
	normalizedISBN := strings.TrimSpace(isbn)
	if normalizedISBN == "" {
		return nil, "", ErrInvalidInput
	}

	cacheKey := "isbn:" + strings.ToLower(normalizedISBN)
	if cached, ok := s.cache.GetBook(cacheKey); ok {
		return cached, SearchSourceMemory, nil
	}

	book, err := s.repo.FindByISBN(ctx, normalizedISBN)
	if err != nil {
		return nil, "", err
	}
	if book != nil {
		s.cache.SetBook(cacheKey, book)
		return book, SearchSourceDatabase, nil
	}

	resolved, err, _ := s.sfByISBN.Do(cacheKey, func() (interface{}, error) {
		extBook, extErr := s.provider.SearchByISBN(ctx, normalizedISBN)
		if extErr != nil {
			if errors.Is(extErr, provider.ErrNoResults) {
				return nil, ErrBookNotFound
			}
			return nil, extErr
		}

		now := time.Now().UTC()
		extBook.CachedAt = &now
		extBook.UpdatedAt = now
		if extBook.CreatedAt.IsZero() {
			extBook.CreatedAt = now
		}
		if err := s.repo.UpsertFromExternal(ctx, extBook); err != nil {
			return nil, err
		}

		persisted, err := s.repo.FindByISBN(ctx, normalizedISBN)
		if err != nil {
			return nil, err
		}
		if persisted == nil {
			return nil, ErrBookNotFound
		}
		s.cache.SetBook(cacheKey, persisted)
		return persisted, nil
	})
	if err != nil {
		return nil, "", err
	}

	return resolved.(*models.Book), SearchSourceExternal, nil
}

func (s *Service) SearchByTitleOrAuthor(ctx context.Context, query string) ([]models.Book, SearchSource, error) {
	normalizedQuery := strings.TrimSpace(query)
	if normalizedQuery == "" {
		return nil, "", ErrInvalidInput
	}

	cacheKey := "query:" + strings.ToLower(normalizedQuery)
	if cached, ok := s.cache.GetBookList(cacheKey); ok {
		return cached, SearchSourceMemory, nil
	}

	books, err := s.repo.SearchByTitleOrAuthor(ctx, normalizedQuery, 20)
	if err != nil {
		return nil, "", err
	}
	if len(books) > 0 {
		s.cache.SetBookList(cacheKey, books)
		return books, SearchSourceDatabase, nil
	}

	resolved, err, _ := s.sfBySearch.Do(cacheKey, func() (interface{}, error) {
		externalBooks, extErr := s.provider.SearchByQuery(ctx, normalizedQuery)
		if extErr != nil {
			if errors.Is(extErr, provider.ErrNoResults) {
				return nil, ErrBookNotFound
			}
			return nil, extErr
		}

		now := time.Now().UTC()
		for i := range externalBooks {
			externalBooks[i].CachedAt = &now
			externalBooks[i].UpdatedAt = now
			if externalBooks[i].CreatedAt.IsZero() {
				externalBooks[i].CreatedAt = now
			}
			if strings.TrimSpace(externalBooks[i].Title) == "" || strings.TrimSpace(externalBooks[i].Author) == "" {
				continue
			}
			if err := s.repo.UpsertFromExternal(ctx, &externalBooks[i]); err != nil {
				return nil, err
			}
		}

		persisted, err := s.repo.SearchByTitleOrAuthor(ctx, normalizedQuery, 20)
		if err != nil {
			return nil, err
		}
		if len(persisted) == 0 {
			return nil, ErrBookNotFound
		}
		s.cache.SetBookList(cacheKey, persisted)
		return persisted, nil
	})
	if err != nil {
		return nil, "", err
	}

	return resolved.([]models.Book), SearchSourceExternal, nil
}
