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
	listFn   func(ctx context.Context) ([]models.Book, error)
	getFn    func(ctx context.Context, id string) (*models.Book, error)
	updateFn func(ctx context.Context, book *models.Book) error
	deleteFn func(ctx context.Context, id string) error
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

func (f *fakeStore) ListAll(ctx context.Context) ([]models.Book, error) {
	if f.listFn == nil {
		return nil, nil
	}
	return f.listFn(ctx)
}

func (f *fakeStore) GetByID(ctx context.Context, id string) (*models.Book, error) {
	if f.getFn == nil {
		return nil, nil
	}
	return f.getFn(ctx, id)
}

func (f *fakeStore) Update(ctx context.Context, book *models.Book) error {
	if f.updateFn == nil {
		return nil
	}
	return f.updateFn(ctx, book)
}

func (f *fakeStore) DeleteByID(ctx context.Context, id string) error {
	if f.deleteFn == nil {
		return nil
	}
	return f.deleteFn(ctx, id)
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

func TestServiceListBooks(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		listFn: func(ctx context.Context) ([]models.Book, error) {
			return []models.Book{
				{ID: "book-1", Title: "Clean Code", Author: "Robert C. Martin"},
				{ID: "book-2", Title: "DDD", Author: "Eric Evans"},
			}, nil
		},
	}
	svc := newTestService(store, &fakeProvider{})

	books, err := svc.ListBooks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("books len = %d, want 2", len(books))
	}
	if books[0].ID != "book-1" {
		t.Fatalf("first id = %q, want %q", books[0].ID, "book-1")
	}
}

func TestServiceUpdate(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		stored := models.Book{
			ID:     "book-1",
			Title:  "Old",
			Author: "Old Author",
			Pages:  120,
		}
		store := &fakeStore{
			getFn: func(ctx context.Context, id string) (*models.Book, error) {
				bookCopy := stored
				return &bookCopy, nil
			},
			updateFn: func(ctx context.Context, book *models.Book) error {
				if book.Title != "New Title" {
					t.Fatalf("title = %q, want %q", book.Title, "New Title")
				}
				stored = *book
				return nil
			},
		}
		svc := newTestService(store, &fakeProvider{})

		book, err := svc.Update(context.Background(), "book-1", "New Title", "New Author", "123", 300, true, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if book == nil || book.Title != "New Title" {
			t.Fatalf("unexpected book: %+v", book)
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		t.Parallel()

		svc := newTestService(&fakeStore{}, &fakeProvider{})
		book, err := svc.Update(context.Background(), "", "Title", "Author", "", 10, false, nil)
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
		if book != nil {
			t.Fatalf("expected nil book, got %+v", book)
		}
	})

	t.Run("book not found", func(t *testing.T) {
		t.Parallel()

		store := &fakeStore{
			getFn: func(ctx context.Context, id string) (*models.Book, error) {
				return nil, nil
			},
		}
		svc := newTestService(store, &fakeProvider{})

		book, err := svc.Update(context.Background(), "missing", "Title", "Author", "", 10, false, nil)
		if !errors.Is(err, ErrBookNotFound) {
			t.Fatalf("expected ErrBookNotFound, got %v", err)
		}
		if book != nil {
			t.Fatalf("expected nil book, got %+v", book)
		}
	})
}

func TestServiceDelete(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		deletedID := ""
		store := &fakeStore{
			getFn: func(ctx context.Context, id string) (*models.Book, error) {
				return &models.Book{ID: id}, nil
			},
			deleteFn: func(ctx context.Context, id string) error {
				deletedID = id
				return nil
			},
		}
		svc := newTestService(store, &fakeProvider{})

		if err := svc.Delete(context.Background(), "book-1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if deletedID != "book-1" {
			t.Fatalf("deletedID = %q, want %q", deletedID, "book-1")
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		t.Parallel()

		svc := newTestService(&fakeStore{}, &fakeProvider{})
		if err := svc.Delete(context.Background(), ""); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("book not found", func(t *testing.T) {
		t.Parallel()

		store := &fakeStore{
			getFn: func(ctx context.Context, id string) (*models.Book, error) {
				return nil, nil
			},
		}
		svc := newTestService(store, &fakeProvider{})

		if err := svc.Delete(context.Background(), "missing"); !errors.Is(err, ErrBookNotFound) {
			t.Fatalf("expected ErrBookNotFound, got %v", err)
		}
	})
}
