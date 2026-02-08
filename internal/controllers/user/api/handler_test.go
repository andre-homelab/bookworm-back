package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/user/security"
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type handlerStore struct {
	createFn          func(ctx context.Context, user *models.User) error
	getByEmailFn      func(ctx context.Context, email string) (*models.User, error)
	getByIDFn         func(ctx context.Context, id string) (*models.User, error)
	updateProfileFn   func(ctx context.Context, id, name, email string) error
	updatePasswordFn  func(ctx context.Context, id, passwordHash string) error
	updateLastLoginFn func(ctx context.Context, id string, loginAt time.Time) error
}

func (s *handlerStore) Create(ctx context.Context, user *models.User) error {
	if s.createFn != nil {
		return s.createFn(ctx, user)
	}
	return nil
}

func (s *handlerStore) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	if s.getByEmailFn != nil {
		return s.getByEmailFn(ctx, email)
	}
	return nil, nil
}

func (s *handlerStore) GetByID(ctx context.Context, id string) (*models.User, error) {
	if s.getByIDFn != nil {
		return s.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (s *handlerStore) UpdateProfile(ctx context.Context, id, name, email string) error {
	if s.updateProfileFn != nil {
		return s.updateProfileFn(ctx, id, name, email)
	}
	return nil
}

func (s *handlerStore) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	if s.updatePasswordFn != nil {
		return s.updatePasswordFn(ctx, id, passwordHash)
	}
	return nil
}

func (s *handlerStore) UpdateLastLogin(ctx context.Context, id string, loginAt time.Time) error {
	if s.updateLastLoginFn != nil {
		return s.updateLastLoginFn(ctx, id, loginAt)
	}
	return nil
}

func newTestUserHandler(t *testing.T, store Store) *UserHandler {
	t.Helper()
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("JWT_ACCESS_EXPIRY", "15m")
	t.Setenv("JWT_REFRESH_EXPIRY", "168h")
	t.Setenv("BCRYPT_COST", "12")

	tokenManager, err := security.NewTokenManagerFromEnv()
	if err != nil {
		t.Fatalf("token manager error: %v", err)
	}
	service, err := NewService(store, tokenManager, security.NewLoginGuard())
	if err != nil {
		t.Fatalf("service error: %v", err)
	}
	return NewUserHandler(service)
}

func TestUserHandlerRegister(t *testing.T) {
	handler := newTestUserHandler(t, &handlerStore{
		createFn: func(ctx context.Context, user *models.User) error {
			user.ID = "user-1"
			return nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"user@example.com","password":"Senha123","name":"User Test"}`))
	handler.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var user UserPublic
	if err := json.NewDecoder(rec.Body).Decode(&user); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if user.ID != "user-1" {
		t.Fatalf("id = %q, want %q", user.ID, "user-1")
	}
}

func TestUserHandlerLoginInvalidCredentials(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("Senha123"), 12)
	if err != nil {
		t.Fatalf("hash error: %v", err)
	}

	handler := newTestUserHandler(t, &handlerStore{
		getByEmailFn: func(ctx context.Context, email string) (*models.User, error) {
			return &models.User{ID: "user-1", Email: email, PasswordHash: string(hash), IsActive: true}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"user@example.com","password":"errada"}`))
	handler.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestUserHandlerGetMeUnauthorized(t *testing.T) {
	handler := newTestUserHandler(t, &handlerStore{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)

	handler.GetMe(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
