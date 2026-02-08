package api

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/models"
)

var (
	ErrInvalidInput = errors.New("dados de entrada inválidos")
	ErrBookNotFound = errors.New("livro não encontrado")
)

type Store interface {
	Create(ctx context.Context, book *models.Book) error
	GetByID(ctx context.Context, id string) (*models.Book, error)
}

type Service struct {
	repo Store
}

func NewService(repo Store) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(
	ctx context.Context,
	title string,
	author string,
	isbn string,
	pages int,
	read bool,
	finishedAt *time.Time,
) (*models.Book, error) {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(author) == "" || pages < 0 {
		return nil, ErrInvalidInput
	}

	now := time.Now().UTC()
	book := models.Book{
		Title:      strings.TrimSpace(title),
		Author:     strings.TrimSpace(author),
		ISBN:       strings.TrimSpace(isbn),
		Pages:      pages,
		Read:       read,
		FinishedAt: finishedAt,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.repo.Create(ctx, &book); err != nil {
		return nil, err
	}

	created, err := s.repo.GetByID(ctx, book.ID)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, ErrBookNotFound
	}

	return created, nil
}
