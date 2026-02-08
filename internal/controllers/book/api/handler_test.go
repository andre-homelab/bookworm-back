package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/book/cache"
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/book/provider"
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/models"
	"github.com/go-chi/chi/v5"
)

type stubStore struct {
	createFn func(ctx context.Context, book *models.Book) error
	listFn   func(ctx context.Context) ([]models.Book, error)
	getFn    func(ctx context.Context, id string) (*models.Book, error)
	updateFn func(ctx context.Context, book *models.Book) error
	deleteFn func(ctx context.Context, id string) error
	findFn   func(ctx context.Context, isbn string) (*models.Book, error)
	searchFn func(ctx context.Context, query string, limit int) ([]models.Book, error)
	upsertFn func(ctx context.Context, book *models.Book) error
}

func (s *stubStore) Create(ctx context.Context, book *models.Book) error {
	if s.createFn != nil {
		return s.createFn(ctx, book)
	}
	return nil
}

func (s *stubStore) ListAll(ctx context.Context) ([]models.Book, error) {
	if s.listFn != nil {
		return s.listFn(ctx)
	}
	return nil, nil
}

func (s *stubStore) GetByID(ctx context.Context, id string) (*models.Book, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return nil, nil
}

func (s *stubStore) Update(ctx context.Context, book *models.Book) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, book)
	}
	return nil
}

func (s *stubStore) DeleteByID(ctx context.Context, id string) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func (s *stubStore) FindByISBN(ctx context.Context, isbn string) (*models.Book, error) {
	if s.findFn != nil {
		return s.findFn(ctx, isbn)
	}
	return nil, nil
}

func (s *stubStore) SearchByTitleOrAuthor(ctx context.Context, query string, limit int) ([]models.Book, error) {
	if s.searchFn != nil {
		return s.searchFn(ctx, query, limit)
	}
	return nil, nil
}

func (s *stubStore) UpsertFromExternal(ctx context.Context, book *models.Book) error {
	if s.upsertFn != nil {
		return s.upsertFn(ctx, book)
	}
	return nil
}

type stubProvider struct{}

func (s *stubProvider) SearchByISBN(ctx context.Context, isbn string) (*models.Book, error) {
	return nil, provider.ErrNoResults
}

func (s *stubProvider) SearchByQuery(ctx context.Context, query string) ([]models.Book, error) {
	return nil, provider.ErrNoResults
}

func decodeErrResp(t *testing.T, rec *httptest.ResponseRecorder) ErrorResponse {
	t.Helper()
	var errResp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	return errResp
}

func newTestHandler(store Store) *BookHandler {
	svc := NewService(store, cache.NewLRUCache(20), &stubProvider{})
	return NewBookHandler(svc)
}

func withURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestBookHandlerCreateBook(t *testing.T) {
	t.Parallel()

	t.Run("invalid json", func(t *testing.T) {
		t.Parallel()

		handler := newTestHandler(&stubStore{})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/books", strings.NewReader("{"))

		handler.CreateBook(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
		errResp := decodeErrResp(t, rec)
		if errResp.Error != "JSON inválido" {
			t.Fatalf("error = %q, want %q", errResp.Error, "JSON inválido")
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		t.Parallel()

		handler := newTestHandler(&stubStore{})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/books", strings.NewReader(`{"title":"","author":"A"}`))

		handler.CreateBook(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
		errResp := decodeErrResp(t, rec)
		if errResp.Error != "Dados inválidos" {
			t.Fatalf("error = %q, want %q", errResp.Error, "Dados inválidos")
		}
	})

	t.Run("repository error", func(t *testing.T) {
		t.Parallel()

		store := &stubStore{
			createFn: func(ctx context.Context, book *models.Book) error {
				return errors.New("db down")
			},
		}
		handler := newTestHandler(store)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/books", strings.NewReader(`{"title":"Livro","author":"Autor"}`))

		handler.CreateBook(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		store := &stubStore{
			createFn: func(ctx context.Context, book *models.Book) error {
				book.ID = "book-1"
				return nil
			},
			getFn: func(ctx context.Context, id string) (*models.Book, error) {
				return &models.Book{ID: id, Title: "Livro", Author: "Autor"}, nil
			},
		}
		handler := newTestHandler(store)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/books", strings.NewReader(`{"title":"Livro","author":"Autor"}`))

		handler.CreateBook(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
		}
		var got models.Book
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got.ID != "book-1" {
			t.Fatalf("id = %q, want %q", got.ID, "book-1")
		}
	})
}

func TestBookHandlerListBooks(t *testing.T) {
	t.Parallel()

	t.Run("repository error", func(t *testing.T) {
		t.Parallel()

		store := &stubStore{
			listFn: func(ctx context.Context) ([]models.Book, error) {
				return nil, errors.New("db down")
			},
		}
		handler := newTestHandler(store)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/books", nil)

		handler.ListBooks(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		store := &stubStore{
			listFn: func(ctx context.Context) ([]models.Book, error) {
				return []models.Book{
					{ID: "book-1", Title: "Livro 1", Author: "Autor 1"},
					{ID: "book-2", Title: "Livro 2", Author: "Autor 2"},
				}, nil
			},
		}
		handler := newTestHandler(store)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/books", nil)

		handler.ListBooks(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		var got []models.Book
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len = %d, want %d", len(got), 2)
		}
	})
}

func TestBookHandlerUpdateBook(t *testing.T) {
	t.Parallel()

	t.Run("invalid json", func(t *testing.T) {
		t.Parallel()

		handler := newTestHandler(&stubStore{})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/books/book-1", strings.NewReader("{"))
		req = withURLParam(req, "id", "book-1")

		handler.UpdateBook(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		store := &stubStore{
			getFn: func(ctx context.Context, id string) (*models.Book, error) {
				return nil, nil
			},
		}
		handler := newTestHandler(store)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/books/missing", strings.NewReader(`{"title":"Livro","author":"Autor","pages":10}`))
		req = withURLParam(req, "id", "missing")

		handler.UpdateBook(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		store := &stubStore{
			getFn: func(ctx context.Context, id string) (*models.Book, error) {
				return &models.Book{ID: id, Title: "Livro", Author: "Autor", Pages: 10}, nil
			},
			updateFn: func(ctx context.Context, book *models.Book) error {
				return nil
			},
		}
		handler := newTestHandler(store)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/books/book-1", strings.NewReader(`{"title":"Livro 2","author":"Autor 2","pages":20}`))
		req = withURLParam(req, "id", "book-1")

		handler.UpdateBook(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}

func TestBookHandlerDeleteBook(t *testing.T) {
	t.Parallel()

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		store := &stubStore{
			getFn: func(ctx context.Context, id string) (*models.Book, error) {
				return nil, nil
			},
		}
		handler := newTestHandler(store)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/books/missing", nil)
		req = withURLParam(req, "id", "missing")

		handler.DeleteBook(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		store := &stubStore{
			getFn: func(ctx context.Context, id string) (*models.Book, error) {
				return &models.Book{ID: id}, nil
			},
			deleteFn: func(ctx context.Context, id string) error {
				return nil
			},
		}
		handler := newTestHandler(store)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/books/book-1", nil)
		req = withURLParam(req, "id", "book-1")

		handler.DeleteBook(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
		}
	})
}
