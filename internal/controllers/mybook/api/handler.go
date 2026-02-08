package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	userapi "github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/user/api"
	"github.com/go-chi/chi/v5"
)

type MyBookHandler struct {
	service *Service
}

type AddMyBookRequest struct {
	BookID     string     `json:"book_id" example:"f4ac9f2d-3c5d-4709-8c7f-63cb9ef0f4bb"`
	Status     string     `json:"status" example:"reading"`
	Rating     *int       `json:"rating,omitempty" example:"5"`
	Notes      string     `json:"notes,omitempty" example:"Lendo no kindle"`
	StartedAt  *time.Time `json:"started_at,omitempty" example:"2026-02-08T10:00:00Z"`
	FinishedAt *time.Time `json:"finished_at,omitempty" example:"2026-02-10T10:00:00Z"`
}

type UpdateMyBookRequest struct {
	Status     string     `json:"status" example:"completed"`
	Rating     *int       `json:"rating,omitempty" example:"5"`
	Notes      string     `json:"notes,omitempty" example:"Excelente livro"`
	StartedAt  *time.Time `json:"started_at,omitempty" example:"2026-02-08T10:00:00Z"`
	FinishedAt *time.Time `json:"finished_at,omitempty" example:"2026-02-10T10:00:00Z"`
}

func NewMyBookHandler(service *Service) *MyBookHandler {
	return &MyBookHandler{service: service}
}

// @Summary     Listar estante do usuário
// @Description Retorna apenas os livros da estante do usuário autenticado
// @Tags        MyBooks
// @Produce     json
// @Success     200 {array} models.UserBook
// @Failure     401 {object} userapi.ErrorResponse
// @Failure     500 {object} userapi.ErrorResponse
// @Router      /my-books [get]
func (h *MyBookHandler) ListMyBooks(w http.ResponseWriter, r *http.Request) {
	userID, ok := userapi.UserIDFromContext(r.Context())
	if !ok {
		userapiRespondError(w, http.StatusUnauthorized, "Não autorizado", nil)
		return
	}

	books, err := h.service.ListByUser(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			userapiRespondError(w, http.StatusBadRequest, "Dados inválidos", err)
			return
		}
		userapiRespondError(w, http.StatusInternalServerError, "Erro ao listar estante", err)
		return
	}
	userapiRespondJSON(w, http.StatusOK, books)
}

// @Summary     Adicionar livro na estante
// @Description Vincula um livro existente à estante do usuário autenticado
// @Tags        MyBooks
// @Accept      json
// @Produce     json
// @Param       body body AddMyBookRequest true "Dados da estante"
// @Success     201 {object} models.UserBook
// @Failure     400 {object} userapi.ErrorResponse
// @Failure     401 {object} userapi.ErrorResponse
// @Failure     404 {object} userapi.ErrorResponse
// @Failure     409 {object} userapi.ErrorResponse
// @Failure     500 {object} userapi.ErrorResponse
// @Router      /my-books [post]
func (h *MyBookHandler) AddMyBook(w http.ResponseWriter, r *http.Request) {
	userID, ok := userapi.UserIDFromContext(r.Context())
	if !ok {
		userapiRespondError(w, http.StatusUnauthorized, "Não autorizado", nil)
		return
	}

	var req AddMyBookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		userapiRespondError(w, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	item, err := h.service.AddToShelf(r.Context(), userID, req.BookID, req.Status, req.Notes, req.Rating, req.StartedAt, req.FinishedAt)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			userapiRespondError(w, http.StatusBadRequest, "Dados inválidos", err)
		case errors.Is(err, ErrBookNotFound):
			userapiRespondError(w, http.StatusNotFound, "Livro não encontrado", nil)
		case errors.Is(err, ErrAlreadyInShelf):
			userapiRespondError(w, http.StatusConflict, "Livro já está na estante", nil)
		default:
			userapiRespondError(w, http.StatusInternalServerError, "Erro ao adicionar livro na estante", err)
		}
		return
	}

	userapiRespondJSON(w, http.StatusCreated, item)
}

// @Summary     Atualizar item da estante
// @Description Atualiza status/rating/notas de um item da estante do usuário autenticado
// @Tags        MyBooks
// @Accept      json
// @Produce     json
// @Param       id path string true "ID do item da estante"
// @Param       body body UpdateMyBookRequest true "Dados da estante"
// @Success     200 {object} models.UserBook
// @Failure     400 {object} userapi.ErrorResponse
// @Failure     401 {object} userapi.ErrorResponse
// @Failure     404 {object} userapi.ErrorResponse
// @Failure     500 {object} userapi.ErrorResponse
// @Router      /my-books/{id} [put]
func (h *MyBookHandler) UpdateMyBook(w http.ResponseWriter, r *http.Request) {
	userID, ok := userapi.UserIDFromContext(r.Context())
	if !ok {
		userapiRespondError(w, http.StatusUnauthorized, "Não autorizado", nil)
		return
	}
	id := chi.URLParam(r, "id")

	var req UpdateMyBookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		userapiRespondError(w, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	item, err := h.service.UpdateShelfItem(r.Context(), userID, id, req.Status, req.Notes, req.Rating, req.StartedAt, req.FinishedAt)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			userapiRespondError(w, http.StatusBadRequest, "Dados inválidos", err)
		case errors.Is(err, ErrMyBookNotFound):
			userapiRespondError(w, http.StatusNotFound, "Livro da estante não encontrado", nil)
		default:
			userapiRespondError(w, http.StatusInternalServerError, "Erro ao atualizar estante", err)
		}
		return
	}

	userapiRespondJSON(w, http.StatusOK, item)
}

// @Summary     Remover livro da estante
// @Description Remove um item da estante do usuário autenticado
// @Tags        MyBooks
// @Produce     json
// @Param       id path string true "ID do item da estante"
// @Success     204
// @Failure     400 {object} userapi.ErrorResponse
// @Failure     401 {object} userapi.ErrorResponse
// @Failure     404 {object} userapi.ErrorResponse
// @Failure     500 {object} userapi.ErrorResponse
// @Router      /my-books/{id} [delete]
func (h *MyBookHandler) DeleteMyBook(w http.ResponseWriter, r *http.Request) {
	userID, ok := userapi.UserIDFromContext(r.Context())
	if !ok {
		userapiRespondError(w, http.StatusUnauthorized, "Não autorizado", nil)
		return
	}
	id := chi.URLParam(r, "id")

	err := h.service.RemoveFromShelf(r.Context(), userID, id)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			userapiRespondError(w, http.StatusBadRequest, "Dados inválidos", err)
		case errors.Is(err, ErrMyBookNotFound):
			userapiRespondError(w, http.StatusNotFound, "Livro da estante não encontrado", nil)
		default:
			userapiRespondError(w, http.StatusInternalServerError, "Erro ao remover livro da estante", err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Reusa o mesmo formato de erro/json do módulo de usuário.
func userapiRespondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func userapiRespondError(w http.ResponseWriter, status int, message string, err error) {
	errResp := userapi.ErrorResponse{Error: message}
	if err != nil {
		errResp.Message = err.Error()
	}
	userapiRespondJSON(w, status, errResp)
}
