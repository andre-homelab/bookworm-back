package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/user/security"
)

type contextKey string

const (
	ContextUserIDKey contextKey = "user_id"
)

type authUserReader interface {
	GetByID(ctx context.Context, id string) (*UserPublic, error)
}

type UserReader interface {
	GetByID(ctx context.Context, id string) (*UserPublic, error)
}

type AuthMiddleware struct {
	tokenManager *security.TokenManager
	service      *Service
}

func NewAuthMiddleware(tokenManager *security.TokenManager, service *Service) *AuthMiddleware {
	return &AuthMiddleware{tokenManager: tokenManager, service: service}
}

func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r.Header.Get("Authorization"))
		if token == "" {
			respondError(w, http.StatusUnauthorized, "Não autorizado", nil)
			return
		}

		claims, err := m.tokenManager.Parse(token)
		if err != nil || claims.Type != security.TokenTypeAccess {
			respondError(w, http.StatusUnauthorized, "Não autorizado", nil)
			return
		}

		user, err := m.service.GetMe(r.Context(), claims.UserID)
		if err != nil || user == nil || !user.IsActive {
			respondError(w, http.StatusUnauthorized, "Não autorizado", nil)
			return
		}

		ctx := context.WithValue(r.Context(), ContextUserIDKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(ContextUserIDKey).(string)
	return userID, ok && strings.TrimSpace(userID) != ""
}

func extractBearerToken(headerValue string) string {
	parts := strings.SplitN(strings.TrimSpace(headerValue), " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
