package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/user/security"
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type fakeStore struct {
	createFn          func(ctx context.Context, user *models.User) error
	getByEmailFn      func(ctx context.Context, email string) (*models.User, error)
	getByIDFn         func(ctx context.Context, id string) (*models.User, error)
	updateProfileFn   func(ctx context.Context, id, name, email string) error
	updatePasswordFn  func(ctx context.Context, id, passwordHash string) error
	updateLastLoginFn func(ctx context.Context, id string, loginAt time.Time) error
}

func (f *fakeStore) Create(ctx context.Context, user *models.User) error {
	if f.createFn != nil {
		return f.createFn(ctx, user)
	}
	return nil
}

func (f *fakeStore) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	if f.getByEmailFn != nil {
		return f.getByEmailFn(ctx, email)
	}
	return nil, nil
}

func (f *fakeStore) GetByID(ctx context.Context, id string) (*models.User, error) {
	if f.getByIDFn != nil {
		return f.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (f *fakeStore) UpdateProfile(ctx context.Context, id, name, email string) error {
	if f.updateProfileFn != nil {
		return f.updateProfileFn(ctx, id, name, email)
	}
	return nil
}

func (f *fakeStore) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	if f.updatePasswordFn != nil {
		return f.updatePasswordFn(ctx, id, passwordHash)
	}
	return nil
}

func (f *fakeStore) UpdateLastLogin(ctx context.Context, id string, loginAt time.Time) error {
	if f.updateLastLoginFn != nil {
		return f.updateLastLoginFn(ctx, id, loginAt)
	}
	return nil
}

func newTestService(t *testing.T, store Store) *Service {
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
	return service
}

func TestServiceRegister(t *testing.T) {
	store := &fakeStore{
		createFn: func(ctx context.Context, user *models.User) error {
			user.ID = "user-1"
			return nil
		},
	}
	svc := newTestService(t, store)

	user, err := svc.Register(context.Background(), "user@example.com", "Senha123", "Usuário")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil || user.ID != "user-1" {
		t.Fatalf("unexpected user: %+v", user)
	}
}

func TestServiceLogin(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("Senha123"), 12)
	if err != nil {
		t.Fatalf("hash error: %v", err)
	}

	store := &fakeStore{
		getByEmailFn: func(ctx context.Context, email string) (*models.User, error) {
			return &models.User{
				ID:           "user-1",
				Email:        email,
				Name:         "Usuário",
				PasswordHash: string(hash),
				IsActive:     true,
			}, nil
		},
	}
	svc := newTestService(t, store)

	login, err := svc.Login(context.Background(), "user@example.com", "Senha123", "127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if login == nil || login.Tokens.AccessToken == "" || login.Tokens.RefreshToken == "" {
		t.Fatalf("unexpected login result: %+v", login)
	}
}

func TestServiceUpdatePasswordInvalidCurrent(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("Senha123"), 12)
	if err != nil {
		t.Fatalf("hash error: %v", err)
	}

	store := &fakeStore{
		getByIDFn: func(ctx context.Context, id string) (*models.User, error) {
			return &models.User{ID: id, PasswordHash: string(hash)}, nil
		},
	}
	svc := newTestService(t, store)

	err = svc.UpdatePassword(context.Background(), "user-1", "senha-invalida", "NovaSenha123")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}
