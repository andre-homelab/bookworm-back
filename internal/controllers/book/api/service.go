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
	ListAll(ctx context.Context) ([]models.Book, error)
	GetByID(ctx context.Context, id string) (*models.Book, error)
	Update(ctx context.Context, book *models.Book) error
	DeleteByID(ctx context.Context, id string) error
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

func (s *Service) ListBooks(ctx context.Context) ([]models.Book, error) {
	return s.repo.ListAll(ctx)
}

func (s *Service) Update(
	ctx context.Context,
	id string,
	title string,
	author string,
	isbn string,
	pages int,
	read bool,
	finishedAt *time.Time,
) (*models.Book, error) {
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" || strings.TrimSpace(title) == "" || strings.TrimSpace(author) == "" || pages < 0 {
		return nil, ErrInvalidInput
	}

	current, err := s.repo.GetByID(ctx, normalizedID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrBookNotFound
	}

	current.Title = strings.TrimSpace(title)
	current.Author = strings.TrimSpace(author)
	current.ISBN = strings.TrimSpace(isbn)
	current.Pages = pages
	current.Read = read
	current.FinishedAt = finishedAt
	current.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, current); err != nil {
		return nil, err
	}

	updated, err := s.repo.GetByID(ctx, current.ID)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, ErrBookNotFound
	}

	s.cache.Reset()
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" {
		return ErrInvalidInput
	}

	current, err := s.repo.GetByID(ctx, normalizedID)
	if err != nil {
		return err
	}
	if current == nil {
		return ErrBookNotFound
	}

	if err := s.repo.DeleteByID(ctx, normalizedID); err != nil {
		return err
	}

	s.cache.Reset()
	return nil
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
