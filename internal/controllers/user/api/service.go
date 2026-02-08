package api

import (
	"context"
	"errors"
	"log"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/user/security"
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/env"
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/models"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidInput       = errors.New("dados de entrada inválidos")
	ErrInvalidCredentials = errors.New("credenciais inválidas")
	ErrUnauthorized       = errors.New("não autorizado")
	ErrUserNotFound       = errors.New("usuário não encontrado")
	ErrEmailAlreadyExists = errors.New("email já cadastrado")
	ErrInactiveUser       = errors.New("usuário inativo")
)

type Store interface {
	Create(ctx context.Context, user *models.User) error
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByID(ctx context.Context, id string) (*models.User, error)
	UpdateProfile(ctx context.Context, id, name, email string) error
	UpdatePassword(ctx context.Context, id, passwordHash string) error
	UpdateLastLogin(ctx context.Context, id string, loginAt time.Time) error
}

type Service struct {
	repo              Store
	tokenManager      *security.TokenManager
	loginGuard        *security.LoginGuard
	bcryptCost        int
	dummyPasswordHash []byte
}

type UserPublic struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	Name          string     `json:"name"`
	LastLoginAt   *time.Time `json:"last_login_at,omitempty"`
	IsActive      bool       `json:"is_active"`
	EmailVerified bool       `json:"email_verified"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type LoginResult struct {
	User   UserPublic         `json:"user"`
	Tokens security.TokenPair `json:"tokens"`
}

func NewService(repo Store, tokenManager *security.TokenManager, loginGuard *security.LoginGuard) (*Service, error) {
	bcryptCost := readBcryptCost()
	dummyHash, err := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcryptCost)
	if err != nil {
		return nil, err
	}

	return &Service{
		repo:              repo,
		tokenManager:      tokenManager,
		loginGuard:        loginGuard,
		bcryptCost:        bcryptCost,
		dummyPasswordHash: dummyHash,
	}, nil
}

func (s *Service) Register(ctx context.Context, email, password, name string) (*UserPublic, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	normalizedName := strings.TrimSpace(name)

	if !isValidEmail(normalizedEmail) || !isStrongPassword(password) || !isValidName(normalizedName) {
		return nil, ErrInvalidInput
	}

	existing, err := s.repo.GetByEmail(ctx, normalizedEmail)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	user := models.User{
		Email:         normalizedEmail,
		PasswordHash:  string(passwordHash),
		Name:          normalizedName,
		IsActive:      true,
		EmailVerified: false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.repo.Create(ctx, &user); err != nil {
		return nil, err
	}
	log.Printf("audit: user registered email=%s user_id=%s", user.Email, user.ID)

	return toUserPublic(&user), nil
}

func (s *Service) Login(ctx context.Context, email, password, ip string) (*LoginResult, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" || strings.TrimSpace(password) == "" {
		return nil, ErrInvalidInput
	}

	if err := s.loginGuard.AllowAttempt(ip, normalizedEmail); err != nil {
		return nil, err
	}

	user, err := s.repo.GetByEmail(ctx, normalizedEmail)
	if err != nil {
		return nil, err
	}
	if user == nil {
		_ = bcrypt.CompareHashAndPassword(s.dummyPasswordHash, []byte(password))
		s.loginGuard.RegisterFailure(normalizedEmail)
		log.Printf("audit: login failure email=%s reason=invalid_credentials", normalizedEmail)
		return nil, ErrInvalidCredentials
	}
	if !user.IsActive {
		s.loginGuard.RegisterFailure(normalizedEmail)
		log.Printf("audit: login failure email=%s reason=inactive_user", normalizedEmail)
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		s.loginGuard.RegisterFailure(normalizedEmail)
		log.Printf("audit: login failure email=%s reason=invalid_credentials", normalizedEmail)
		return nil, ErrInvalidCredentials
	}

	s.loginGuard.RegisterSuccess(normalizedEmail)

	now := time.Now().UTC()
	if err := s.repo.UpdateLastLogin(ctx, user.ID, now); err != nil {
		return nil, err
	}
	user.LastLoginAt = &now
	log.Printf("audit: login success email=%s user_id=%s", user.Email, user.ID)

	tokens, err := s.tokenManager.GeneratePair(user)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		User:   *toUserPublic(user),
		Tokens: *tokens,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*security.TokenPair, error) {
	claims, err := s.tokenManager.Parse(strings.TrimSpace(refreshToken))
	if err != nil {
		return nil, ErrUnauthorized
	}
	if claims.Type != security.TokenTypeRefresh {
		return nil, ErrUnauthorized
	}
	if s.tokenManager.IsRefreshTokenRevoked(claims.ID) {
		return nil, ErrUnauthorized
	}

	user, err := s.repo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil || !user.IsActive {
		return nil, ErrUnauthorized
	}

	if claims.ExpiresAt != nil {
		s.tokenManager.RevokeRefreshToken(claims.ID, claims.ExpiresAt.Time)
	}

	return s.tokenManager.GeneratePair(user)
}

func (s *Service) GetMe(ctx context.Context, userID string) (*UserPublic, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrUnauthorized
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	return toUserPublic(user), nil
}

func (s *Service) UpdateMe(ctx context.Context, userID, name, email string) (*UserPublic, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	normalizedName := strings.TrimSpace(name)
	if strings.TrimSpace(userID) == "" || !isValidEmail(normalizedEmail) || !isValidName(normalizedName) {
		return nil, ErrInvalidInput
	}

	current, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrUserNotFound
	}

	if !strings.EqualFold(current.Email, normalizedEmail) {
		existing, err := s.repo.GetByEmail(ctx, normalizedEmail)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.ID != current.ID {
			return nil, ErrEmailAlreadyExists
		}
	}

	if err := s.repo.UpdateProfile(ctx, current.ID, normalizedName, normalizedEmail); err != nil {
		return nil, err
	}

	updated, err := s.repo.GetByID(ctx, current.ID)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, ErrUserNotFound
	}
	return toUserPublic(updated), nil
}

func (s *Service) UpdatePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(currentPassword) == "" || !isStrongPassword(newPassword) {
		return ErrInvalidInput
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrInvalidCredentials
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.bcryptCost)
	if err != nil {
		return err
	}

	if err := s.repo.UpdatePassword(ctx, user.ID, string(newHash)); err != nil {
		return err
	}
	log.Printf("audit: password changed user_id=%s", user.ID)
	return nil
}

func toUserPublic(user *models.User) *UserPublic {
	if user == nil {
		return nil
	}

	return &UserPublic{
		ID:            user.ID,
		Email:         user.Email,
		Name:          user.Name,
		LastLoginAt:   user.LastLoginAt,
		IsActive:      user.IsActive,
		EmailVerified: user.EmailVerified,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}
}

func isValidEmail(value string) bool {
	addr, err := mail.ParseAddress(value)
	if err != nil {
		return false
	}
	return strings.EqualFold(addr.Address, value)
}

func isValidName(value string) bool {
	length := len(strings.TrimSpace(value))
	return length >= 2 && length <= 100
}

func isStrongPassword(value string) bool {
	if len(value) < 8 {
		return false
	}

	hasLetter := false
	hasNumber := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			hasLetter = true
		}
		if r >= '0' && r <= '9' {
			hasNumber = true
		}
	}
	return hasLetter && hasNumber
}

func readBcryptCost() int {
	raw := strings.TrimSpace(env.Get("BCRYPT_COST", "12"))
	cost, err := strconv.Atoi(raw)
	if err != nil {
		return 12
	}
	if cost < 12 {
		return 12
	}
	if cost > 14 {
		return 14
	}
	return cost
}
