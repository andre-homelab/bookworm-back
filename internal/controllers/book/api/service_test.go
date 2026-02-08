package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/models"
)

type fakeStore struct {
	createFn func(ctx context.Context, book *models.Book) error
	getFn    func(ctx context.Context, id string) (*models.Book, error)
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
		svc := NewService(store)

		finishedAt := time.Date(2025, 8, 11, 14, 0, 0, 0, time.UTC)
		book, err := svc.Create(context.Background(), "Clean Code", "Robert C. Martin", "9780132350884", 464, true, &finishedAt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if book == nil || book.ID != "book-1" {
			t.Fatalf("unexpected book: %+v", book)
		}
		if book.Title != "Clean Code" || book.Author != "Robert C. Martin" {
			t.Fatalf("unexpected basic fields: %+v", book)
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		t.Parallel()

		svc := NewService(&fakeStore{})
		book, err := svc.Create(context.Background(), "", "Autor", "", 100, false, nil)
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
		if book != nil {
			t.Fatalf("expected nil book, got %+v", book)
		}
	})
}
