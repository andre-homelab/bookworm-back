package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type BookHandler struct {
	bookService Service
}

func NewBookHandler(bookService *Service) *BookHandler {
	return &BookHandler{bookService: *bookService}
}

type CreateBookRequest struct {
	Title      string     `json:"title" example:"Clean Code"`
	Author     string     `json:"author" example:"Robert C. Martin"`
	ISBN       string     `json:"isbn,omitempty" example:"9780132350884"`
	Pages      int        `json:"pages" example:"464"`
	Read       bool       `json:"read" example:"true"`
	FinishedAt *time.Time `json:"finished_at,omitempty" example:"2025-06-15T10:00:00Z"`
}

type ErrorResponse struct {
	Error   string `json:"error" example:"dados inválidos"`
	Message string `json:"message,omitempty" example:"title é obrigatório"`
}

// @Summary     Criar novo livro
// @Description Adiciona um novo livro ao catálogo pessoal
// @Tags        Books
// @Accept      json
// @Produce     json
// @Param       book body CreateBookRequest true "Dados do livro"
// @Success     201 {object} models.Book
// @Failure     400 {object} ErrorResponse
// @Failure     500 {object} ErrorResponse
// @Router      /books [post]
func (h *BookHandler) CreateBook(w http.ResponseWriter, r *http.Request) {
	var req CreateBookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	book, err := h.bookService.Create(
		r.Context(),
		req.Title,
		req.Author,
		req.ISBN,
		req.Pages,
		req.Read,
		req.FinishedAt,
	)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			respondError(w, http.StatusBadRequest, "Dados inválidos", err)
			return
		}
		respondError(w, http.StatusInternalServerError, "Erro ao criar livro", err)
		return
	}

	respondJSON(w, http.StatusCreated, book)
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string, err error) {
	errResp := ErrorResponse{Error: message}
	if err != nil {
		errResp.Message = err.Error()
	}
	respondJSON(w, status, errResp)
}
