package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/book/cache"
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/book/provider"
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/models"
)

type fakeStore struct {
	createFn func(ctx context.Context, book *models.Book) error
	getFn    func(ctx context.Context, id string) (*models.Book, error)
	findFn   func(ctx context.Context, isbn string) (*models.Book, error)
	searchFn func(ctx context.Context, query string, limit int) ([]models.Book, error)
	upsertFn func(ctx context.Context, book *models.Book) error
}

func (f *fakeStore) Create(ctx context.Context, book *models.Book) error {
	if f.createFn == nil {
		return nil
	}
	return f.createFn(ctx, book)
}

func (f *fakeStore) GetByID(ctx context.Context, id string) (*models.Book, error) {
	if f.getFn == nil {
		return nil, nil
	}
	return f.getFn(ctx, id)
}

func (f *fakeStore) FindByISBN(ctx context.Context, isbn string) (*models.Book, error) {
	if f.findFn == nil {
		return nil, nil
	}
	return f.findFn(ctx, isbn)
}

func (f *fakeStore) SearchByTitleOrAuthor(ctx context.Context, query string, limit int) ([]models.Book, error) {
	if f.searchFn == nil {
		return nil, nil
	}
	return f.searchFn(ctx, query, limit)
}

func (f *fakeStore) UpsertFromExternal(ctx context.Context, book *models.Book) error {
	if f.upsertFn == nil {
		return nil
	}
	return f.upsertFn(ctx, book)
}

type fakeProvider struct {
	isbnFn   func(ctx context.Context, isbn string) (*models.Book, error)
	queryFn  func(ctx context.Context, query string) ([]models.Book, error)
	isbnHits int
}

func (f *fakeProvider) SearchByISBN(ctx context.Context, isbn string) (*models.Book, error) {
	f.isbnHits++
	if f.isbnFn == nil {
		return nil, provider.ErrNoResults
	}
	return f.isbnFn(ctx, isbn)
}

func (f *fakeProvider) SearchByQuery(ctx context.Context, query string) ([]models.Book, error) {
	if f.queryFn == nil {
		return nil, provider.ErrNoResults
	}
	return f.queryFn(ctx, query)
}

func newTestService(store Store, ext provider.ExternalBookProvider) *Service {
	return NewService(store, cache.NewLRUCache(20), ext)
}

func TestServiceCreate(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var captured *models.Book
		store := &fakeStore{
			createFn: func(ctx context.Context, book *models.Book) error {
				captured = book
				book.ID = "book-1"
				return nil
			},
			getFn: func(ctx context.Context, id string) (*models.Book, error) {
				if captured == nil {
					t.Fatal("expected Create to run before GetByID")
				}
				if id != captured.ID {
					t.Fatalf("expected id %q, got %q", captured.ID, id)
				}
				return captured, nil
			},
		}
		svc := newTestService(store, &fakeProvider{})

		finishedAt := time.Date(2025, 8, 11, 14, 0, 0, 0, time.UTC)
		book, err := svc.Create(context.Background(), "Clean Code", "Robert C. Martin", "9780132350884", 464, true, &finishedAt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if book == nil || book.ID != "book-1" {
			t.Fatalf("unexpected book: %+v", book)
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		t.Parallel()

		svc := newTestService(&fakeStore{}, &fakeProvider{})
		book, err := svc.Create(context.Background(), "", "Autor", "", 100, false, nil)
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
		if book != nil {
			t.Fatalf("expected nil book, got %+v", book)
		}
	})
}

func TestServiceSearchByISBN(t *testing.T) {
	t.Parallel()

	t.Run("memory hit", func(t *testing.T) {
		t.Parallel()

		provider := &fakeProvider{}
		store := &fakeStore{}
		svc := newTestService(store, provider)
		svc.cache.SetBook("isbn:9780132350884", &models.Book{ISBN: "9780132350884", Title: "Clean Code", Author: "Uncle Bob"})

		book, source, err := svc.SearchByISBN(context.Background(), "9780132350884")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if source != SearchSourceMemory {
			t.Fatalf("source = %s, want %s", source, SearchSourceMemory)
		}
		if book == nil || book.Title != "Clean Code" {
			t.Fatalf("unexpected book: %+v", book)
		}
		if provider.isbnHits != 0 {
			t.Fatalf("provider should not be called")
		}
	})

	t.Run("database hit", func(t *testing.T) {
		t.Parallel()

		provider := &fakeProvider{}
		store := &fakeStore{
			findFn: func(ctx context.Context, isbn string) (*models.Book, error) {
				return &models.Book{ISBN: isbn, Title: "DB Book", Author: "DB Author"}, nil
			},
		}
		svc := newTestService(store, provider)

		book, source, err := svc.SearchByISBN(context.Background(), "9780132350884")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if source != SearchSourceDatabase {
			t.Fatalf("source = %s, want %s", source, SearchSourceDatabase)
		}
		if book == nil || book.Title != "DB Book" {
			t.Fatalf("unexpected book: %+v", book)
		}
		if provider.isbnHits != 0 {
			t.Fatalf("provider should not be called")
		}
	})

	t.Run("external hit", func(t *testing.T) {
		t.Parallel()

		storeData := map[string]models.Book{}
		store := &fakeStore{
			findFn: func(ctx context.Context, isbn string) (*models.Book, error) {
				if book, ok := storeData[isbn]; ok {
					bookCopy := book
					return &bookCopy, nil
				}
				return nil, nil
			},
			upsertFn: func(ctx context.Context, book *models.Book) error {
				book.ID = "persisted-id"
				storeData[book.ISBN] = *book
				return nil
			},
		}
		provider := &fakeProvider{
			isbnFn: func(ctx context.Context, isbn string) (*models.Book, error) {
				return &models.Book{ISBN: isbn, Title: "API Book", Author: "API Author", CacheSource: "google_books"}, nil
			},
		}
		svc := newTestService(store, provider)

		book, source, err := svc.SearchByISBN(context.Background(), "9780132350884")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if source != SearchSourceExternal {
			t.Fatalf("source = %s, want %s", source, SearchSourceExternal)
		}
		if book == nil || book.Title != "API Book" {
			t.Fatalf("unexpected book: %+v", book)
		}
		if provider.isbnHits != 1 {
			t.Fatalf("provider calls = %d, want 1", provider.isbnHits)
		}
	})
}
