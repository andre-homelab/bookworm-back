package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/models"
)

type fakeStore struct {
	listByUserFn          func(ctx context.Context, userID string) ([]models.UserBook, error)
	createFn              func(ctx context.Context, userBook *models.UserBook) error
	getByIDAndUserFn      func(ctx context.Context, id, userID string) (*models.UserBook, error)
	updateFn              func(ctx context.Context, userBook *models.UserBook) error
	deleteByIDAndUserFn   func(ctx context.Context, id, userID string) error
	existsByUserAndBookFn func(ctx context.Context, userID, bookID string) (bool, error)
	getBookByIDFn         func(ctx context.Context, id string) (*models.Book, error)
}

func (f *fakeStore) ListByUser(ctx context.Context, userID string) ([]models.UserBook, error) {
	if f.listByUserFn != nil {
		return f.listByUserFn(ctx, userID)
	}
	return nil, nil
}

func (f *fakeStore) Create(ctx context.Context, userBook *models.UserBook) error {
	if f.createFn != nil {
		return f.createFn(ctx, userBook)
	}
	return nil
}

func (f *fakeStore) GetByIDAndUser(ctx context.Context, id, userID string) (*models.UserBook, error) {
	if f.getByIDAndUserFn != nil {
		return f.getByIDAndUserFn(ctx, id, userID)
	}
	return nil, nil
}

func (f *fakeStore) Update(ctx context.Context, userBook *models.UserBook) error {
	if f.updateFn != nil {
		return f.updateFn(ctx, userBook)
	}
	return nil
}

func (f *fakeStore) DeleteByIDAndUser(ctx context.Context, id, userID string) error {
	if f.deleteByIDAndUserFn != nil {
		return f.deleteByIDAndUserFn(ctx, id, userID)
	}
	return nil
}

func (f *fakeStore) ExistsByUserAndBook(ctx context.Context, userID, bookID string) (bool, error) {
	if f.existsByUserAndBookFn != nil {
		return f.existsByUserAndBookFn(ctx, userID, bookID)
	}
	return false, nil
}

func (f *fakeStore) GetBookByID(ctx context.Context, id string) (*models.Book, error) {
	if f.getBookByIDFn != nil {
		return f.getBookByIDFn(ctx, id)
	}
	return nil, nil
}

func TestServiceAddToShelf(t *testing.T) {
	t.Parallel()

	stored := &models.UserBook{}
	service := NewService(&fakeStore{
		getBookByIDFn: func(ctx context.Context, id string) (*models.Book, error) {
			return &models.Book{ID: id}, nil
		},
		createFn: func(ctx context.Context, userBook *models.UserBook) error {
			userBook.ID = "shelf-1"
			*stored = *userBook
			return nil
		},
		getByIDAndUserFn: func(ctx context.Context, id, userID string) (*models.UserBook, error) {
			if id != "shelf-1" || userID != "user-1" {
				return nil, errors.New("unexpected lookup")
			}
			copyItem := *stored
			return &copyItem, nil
		},
	})

	rating := 5
	item, err := service.AddToShelf(context.Background(), "user-1", "book-1", models.UserBookStatusReading, "nota", &rating, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item == nil || item.ID != "shelf-1" {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestServiceAddToShelfDuplicate(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeStore{
		getBookByIDFn: func(ctx context.Context, id string) (*models.Book, error) {
			return &models.Book{ID: id}, nil
		},
		existsByUserAndBookFn: func(ctx context.Context, userID, bookID string) (bool, error) {
			return true, nil
		},
	})

	_, err := service.AddToShelf(context.Background(), "user-1", "book-1", models.UserBookStatusReading, "", nil, nil, nil)
	if !errors.Is(err, ErrAlreadyInShelf) {
		t.Fatalf("expected ErrAlreadyInShelf, got %v", err)
	}
}

func TestServiceListByUser(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeStore{
		listByUserFn: func(ctx context.Context, userID string) ([]models.UserBook, error) {
			return []models.UserBook{
				{ID: "shelf-1", UserID: userID, BookID: "book-1", Status: models.UserBookStatusWantToRead},
			}, nil
		},
	})

	items, err := service.ListByUser(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].UserID != "user-1" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestServiceUpdateAndDelete(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	stored := models.UserBook{
		ID:     "shelf-1",
		UserID: "user-1",
		BookID: "book-1",
		Status: models.UserBookStatusReading,
	}
	service := NewService(&fakeStore{
		getByIDAndUserFn: func(ctx context.Context, id, userID string) (*models.UserBook, error) {
			if id == "missing" {
				return nil, nil
			}
			copyItem := stored
			return &copyItem, nil
		},
		updateFn: func(ctx context.Context, userBook *models.UserBook) error {
			stored = *userBook
			return nil
		},
		deleteByIDAndUserFn: func(ctx context.Context, id, userID string) error {
			return nil
		},
	})

	updated, err := service.UpdateShelfItem(context.Background(), "user-1", "shelf-1", models.UserBookStatusCompleted, "ok", nil, nil, &now)
	if err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}
	if updated.Status != models.UserBookStatusCompleted {
		t.Fatalf("status = %s, want %s", updated.Status, models.UserBookStatusCompleted)
	}

	if err := service.RemoveFromShelf(context.Background(), "user-1", "shelf-1"); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}
}
