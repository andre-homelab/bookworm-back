package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/env"
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/models"
	"github.com/golang-jwt/jwt/v5"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type TokenClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email,omitempty"`
	Type   string `json:"type"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type TokenManager struct {
	secret        []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
	mu            sync.Mutex
	revokedJTIs   map[string]time.Time
}

func NewTokenManagerFromEnv() (*TokenManager, error) {
	secret := strings.TrimSpace(env.Get("JWT_SECRET", ""))
	if len(secret) < 32 {
		return nil, errors.New("JWT_SECRET deve ter no mínimo 32 caracteres após trim de espaços")
	}

	accessExpiry, err := time.ParseDuration(env.Get("JWT_ACCESS_EXPIRY", "15m"))
	if err != nil {
		return nil, errors.New("JWT_ACCESS_EXPIRY inválido")
	}

	refreshExpiry, err := time.ParseDuration(env.Get("JWT_REFRESH_EXPIRY", "168h"))
	if err != nil {
		return nil, errors.New("JWT_REFRESH_EXPIRY inválido")
	}

	return &TokenManager{
		secret:        []byte(secret),
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
		revokedJTIs:   make(map[string]time.Time),
	}, nil
}

func (m *TokenManager) GeneratePair(user *models.User) (*TokenPair, error) {
	access, err := m.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}

	refresh, err := m.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, err
	}

	return &TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

func (m *TokenManager) GenerateAccessToken(user *models.User) (string, error) {
	now := time.Now().UTC()
	claims := TokenClaims{
		UserID: user.ID,
		Email:  user.Email,
		Type:   TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessExpiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *TokenManager) GenerateRefreshToken(userID string) (string, error) {
	now := time.Now().UTC()
	jti, err := generateTokenID()
	if err != nil {
		return "", err
	}

	claims := TokenClaims{
		UserID: userID,
		Type:   TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshExpiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *TokenManager) Parse(token string) (*TokenClaims, error) {
	parsed, err := jwt.ParseWithClaims(token, &TokenClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method == nil || t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("algoritmo de token inválido")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("token inválido")
	}

	claims, ok := parsed.Claims.(*TokenClaims)
	if !ok {
		return nil, errors.New("claims inválidos")
	}
	return claims, nil
}

func (m *TokenManager) IsRefreshTokenRevoked(jti string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupRevokedLocked(time.Now().UTC())
	expiresAt, exists := m.revokedJTIs[jti]
	return exists && expiresAt.After(time.Now().UTC())
}

func (m *TokenManager) RevokeRefreshToken(jti string, expiresAt time.Time) {
	if strings.TrimSpace(jti) == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.revokedJTIs[jti] = expiresAt.UTC()
	m.cleanupRevokedLocked(time.Now().UTC())
}

func (m *TokenManager) cleanupRevokedLocked(now time.Time) {
	for jti, exp := range m.revokedJTIs {
		if !exp.After(now) {
			delete(m.revokedJTIs, jti)
		}
	}
}

func generateTokenID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
