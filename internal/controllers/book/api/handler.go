package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
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

type UpdateBookRequest struct {
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

// @Summary     Listar todos os livros
// @Description Retorna todos os livros armazenados no banco
// @Tags        Books
// @Produce     json
// @Success     200 {array} models.Book
// @Failure     500 {object} ErrorResponse
// @Router      /books [get]
func (h *BookHandler) ListBooks(w http.ResponseWriter, r *http.Request) {
	books, err := h.bookService.ListBooks(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Erro ao listar livros", err)
		return
	}

	respondJSON(w, http.StatusOK, books)
}

// @Summary     Buscar livro por ISBN
// @Description Busca com estratégia de cache em 3 níveis (RAM, PostgreSQL, API externa)
// @Tags        Books
// @Produce     json
// @Param       isbn path string true "ISBN do livro"
// @Success     200 {object} models.Book
// @Failure     400 {object} ErrorResponse
// @Failure     404 {object} ErrorResponse
// @Failure     500 {object} ErrorResponse
// @Router      /books/search/isbn/{isbn} [get]
func (h *BookHandler) SearchByISBN(w http.ResponseWriter, r *http.Request) {
	isbn := chi.URLParam(r, "isbn")

	book, source, err := h.bookService.SearchByISBN(r.Context(), isbn)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			respondError(w, http.StatusBadRequest, "ISBN inválido", err)
			return
		}
		if errors.Is(err, ErrBookNotFound) {
			respondError(w, http.StatusNotFound, "Livro não encontrado", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "Erro ao buscar livro", err)
		return
	}

	w.Header().Set("X-Cache-Source", string(source))
	respondJSON(w, http.StatusOK, book)
}

// @Summary     Buscar livros por título ou autor
// @Description Busca com estratégia de cache em 3 níveis (RAM, PostgreSQL, API externa)
// @Tags        Books
// @Produce     json
// @Param       q query string true "Texto de busca"
// @Success     200 {array} models.Book
// @Failure     400 {object} ErrorResponse
// @Failure     404 {object} ErrorResponse
// @Failure     500 {object} ErrorResponse
// @Router      /books/search [get]
func (h *BookHandler) SearchByTitleOrAuthor(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	books, source, err := h.bookService.SearchByTitleOrAuthor(r.Context(), query)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			respondError(w, http.StatusBadRequest, "Parâmetro q é obrigatório", err)
			return
		}
		if errors.Is(err, ErrBookNotFound) {
			respondError(w, http.StatusNotFound, "Livro não encontrado", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "Erro ao buscar livros", err)
		return
	}

	w.Header().Set("X-Cache-Source", string(source))
	respondJSON(w, http.StatusOK, books)
}

// @Summary     Atualizar livro por ID
// @Description Atualiza os dados principais de um livro existente
// @Tags        Books
// @Accept      json
// @Produce     json
// @Param       id path string true "ID do livro"
// @Param       book body UpdateBookRequest true "Dados do livro"
// @Success     200 {object} models.Book
// @Failure     400 {object} ErrorResponse
// @Failure     404 {object} ErrorResponse
// @Failure     500 {object} ErrorResponse
// @Router      /books/{id} [put]
func (h *BookHandler) UpdateBook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req UpdateBookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "JSON inválido", err)
		return
	}

	book, err := h.bookService.Update(
		r.Context(),
		id,
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
		if errors.Is(err, ErrBookNotFound) {
			respondError(w, http.StatusNotFound, "Livro não encontrado", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "Erro ao atualizar livro", err)
		return
	}

	respondJSON(w, http.StatusOK, book)
}

// @Summary     Remover livro por ID
// @Description Remove um livro do catálogo pessoal
// @Tags        Books
// @Produce     json
// @Param       id path string true "ID do livro"
// @Success     204
// @Failure     400 {object} ErrorResponse
// @Failure     404 {object} ErrorResponse
// @Failure     500 {object} ErrorResponse
// @Router      /books/{id} [delete]
func (h *BookHandler) DeleteBook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	err := h.bookService.Delete(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			respondError(w, http.StatusBadRequest, "ID inválido", err)
			return
		}
		if errors.Is(err, ErrBookNotFound) {
			respondError(w, http.StatusNotFound, "Livro não encontrado", nil)
			return
		}
		respondError(w, http.StatusInternalServerError, "Erro ao remover livro", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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
