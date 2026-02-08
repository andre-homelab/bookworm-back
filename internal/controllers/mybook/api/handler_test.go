package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	userapi "github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/user/api"
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/models"
	"github.com/go-chi/chi/v5"
)

func withUser(req *http.Request, userID string) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), userapi.ContextUserIDKey, userID))
}

func withURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestHandlerListMyBooks(t *testing.T) {
	t.Parallel()

	handler := NewMyBookHandler(NewService(&fakeStore{
		listByUserFn: func(ctx context.Context, userID string) ([]models.UserBook, error) {
			return []models.UserBook{{ID: "shelf-1", UserID: userID}}, nil
		},
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/my-books", nil)
	req = withUser(req, "user-1")

	handler.ListMyBooks(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandlerAddMyBook(t *testing.T) {
	t.Parallel()

	handler := NewMyBookHandler(NewService(&fakeStore{
		getBookByIDFn: func(ctx context.Context, id string) (*models.Book, error) {
			return &models.Book{ID: id}, nil
		},
		createFn: func(ctx context.Context, userBook *models.UserBook) error {
			userBook.ID = "shelf-1"
			return nil
		},
		getByIDAndUserFn: func(ctx context.Context, id, userID string) (*models.UserBook, error) {
			return &models.UserBook{ID: id, UserID: userID, BookID: "book-1", Status: models.UserBookStatusReading}, nil
		},
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/my-books", strings.NewReader(`{"book_id":"book-1","status":"reading"}`))
	req = withUser(req, "user-1")

	handler.AddMyBook(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestHandlerDeleteMyBook(t *testing.T) {
	t.Parallel()

	handler := NewMyBookHandler(NewService(&fakeStore{
		getByIDAndUserFn: func(ctx context.Context, id, userID string) (*models.UserBook, error) {
			return &models.UserBook{ID: id, UserID: userID}, nil
		},
		deleteByIDAndUserFn: func(ctx context.Context, id, userID string) error {
			return nil
		},
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/my-books/shelf-1", nil)
	req = withUser(req, "user-1")
	req = withURLParam(req, "id", "shelf-1")

	handler.DeleteMyBook(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
