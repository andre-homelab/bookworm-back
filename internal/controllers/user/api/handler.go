package api

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/user/security"
)

type UserHandler struct {
	service *Service
}

type RegisterRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"Senha123"`
	Name     string `json:"name" example:"Usuário Teste"`
}

type LoginRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"Senha123"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" example:"token..."`
}

type UpdateMeRequest struct {
	Email string `json:"email" example:"new-email@example.com"`
	Name  string `json:"name" example:"Novo Nome"`
}

type UpdatePasswordRequest struct {
	CurrentPassword string `json:"current_password" example:"Senha123"`
	NewPassword     string `json:"new_password" example:"NovaSenha123"`
}

func NewUserHandler(service *Service) *UserHandler {
	return &UserHandler{service: service}
}

// @Summary     Registrar usuário
// @Description Cria uma nova conta de usuário sem autenticar automaticamente
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       body body RegisterRequest true "Dados de registro"
// @Success     201 {object} UserPublic
// @Failure     400 {object} ErrorResponse
// @Failure     409 {object} ErrorResponse
// @Failure     500 {object} ErrorResponse
// @Router      /auth/register [post]
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	user, err := h.service.Register(r.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			respondError(w, http.StatusBadRequest, "Dados inválidos", err)
		case errors.Is(err, ErrEmailAlreadyExists):
			respondError(w, http.StatusConflict, "Email já cadastrado", nil)
		default:
			respondError(w, http.StatusInternalServerError, "Erro ao registrar usuário", err)
		}
		return
	}

	respondJSON(w, http.StatusCreated, user)
}

// @Summary     Login
// @Description Autentica usuário e retorna access/refresh tokens JWT
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       body body LoginRequest true "Credenciais"
// @Success     200 {object} LoginResult
// @Failure     400 {object} ErrorResponse
// @Failure     401 {object} ErrorResponse
// @Failure     429 {object} ErrorResponse
// @Failure     500 {object} ErrorResponse
// @Router      /auth/login [post]
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	ip := clientIP(r)
	loginResult, err := h.service.Login(r.Context(), req.Email, req.Password, ip)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			respondError(w, http.StatusBadRequest, "Dados inválidos", err)
		case errors.Is(err, security.ErrRateLimitExceeded), errors.Is(err, security.ErrEmailTemporarilyBlocked):
			respondError(w, http.StatusTooManyRequests, "Muitas tentativas. Tente novamente mais tarde", nil)
		case errors.Is(err, ErrInvalidCredentials):
			respondError(w, http.StatusUnauthorized, "Credenciais inválidas", nil)
		default:
			respondError(w, http.StatusInternalServerError, "Erro ao autenticar", err)
		}
		return
	}

	respondJSON(w, http.StatusOK, loginResult)
}

// @Summary     Refresh de token
// @Description Gera novo par de tokens a partir de refresh token válido
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       body body RefreshRequest true "Refresh token"
// @Success     200 {object} security.TokenPair
// @Failure     400 {object} ErrorResponse
// @Failure     401 {object} ErrorResponse
// @Failure     500 {object} ErrorResponse
// @Router      /auth/refresh [post]
func (h *UserHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	tokens, err := h.service.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			respondError(w, http.StatusUnauthorized, "Não autorizado", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "Erro ao renovar token", err)
		return
	}

	respondJSON(w, http.StatusOK, tokens)
}

// @Summary     Logout
// @Description Efetua logout no cliente autenticado
// @Tags        Auth
// @Produce     json
// @Success     204
// @Router      /auth/logout [post]
func (h *UserHandler) Logout(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// @Summary     Perfil do usuário logado
// @Description Retorna dados do usuário autenticado
// @Tags        Users
// @Produce     json
// @Success     200 {object} UserPublic
// @Failure     401 {object} ErrorResponse
// @Failure     404 {object} ErrorResponse
// @Failure     500 {object} ErrorResponse
// @Router      /users/me [get]
func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Não autorizado", nil)
		return
	}

	user, err := h.service.GetMe(r.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			respondError(w, http.StatusNotFound, "Usuário não encontrado", nil)
		default:
			respondError(w, http.StatusInternalServerError, "Erro ao buscar usuário", err)
		}
		return
	}

	respondJSON(w, http.StatusOK, user)
}

// @Summary     Atualizar perfil
// @Description Atualiza nome e email do usuário autenticado
// @Tags        Users
// @Accept      json
// @Produce     json
// @Param       body body UpdateMeRequest true "Dados do perfil"
// @Success     200 {object} UserPublic
// @Failure     400 {object} ErrorResponse
// @Failure     401 {object} ErrorResponse
// @Failure     409 {object} ErrorResponse
// @Failure     500 {object} ErrorResponse
// @Router      /users/me [put]
func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Não autorizado", nil)
		return
	}

	var req UpdateMeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	user, err := h.service.UpdateMe(r.Context(), userID, req.Name, req.Email)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			respondError(w, http.StatusBadRequest, "Dados inválidos", err)
		case errors.Is(err, ErrEmailAlreadyExists):
			respondError(w, http.StatusConflict, "Email já cadastrado", nil)
		case errors.Is(err, ErrUserNotFound):
			respondError(w, http.StatusNotFound, "Usuário não encontrado", nil)
		default:
			respondError(w, http.StatusInternalServerError, "Erro ao atualizar perfil", err)
		}
		return
	}

	respondJSON(w, http.StatusOK, user)
}

// @Summary     Atualizar senha
// @Description Atualiza senha do usuário autenticado exigindo senha atual
// @Tags        Users
// @Accept      json
// @Produce     json
// @Param       body body UpdatePasswordRequest true "Dados da senha"
// @Success     204
// @Failure     400 {object} ErrorResponse
// @Failure     401 {object} ErrorResponse
// @Failure     404 {object} ErrorResponse
// @Failure     500 {object} ErrorResponse
// @Router      /users/me/password [put]
func (h *UserHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Não autorizado", nil)
		return
	}

	var req UpdatePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	err := h.service.UpdatePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			respondError(w, http.StatusBadRequest, "Dados inválidos", err)
		case errors.Is(err, ErrInvalidCredentials):
			respondError(w, http.StatusUnauthorized, "Credenciais inválidas", nil)
		case errors.Is(err, ErrUserNotFound):
			respondError(w, http.StatusNotFound, "Usuário não encontrado", nil)
		default:
			respondError(w, http.StatusInternalServerError, "Erro ao atualizar senha", err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func clientIP(r *http.Request) string {
	forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return ip
}
