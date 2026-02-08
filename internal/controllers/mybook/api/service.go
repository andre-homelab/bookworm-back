package api

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/models"
)

var (
	ErrInvalidInput   = errors.New("dados de entrada inválidos")
	ErrBookNotFound   = errors.New("livro não encontrado")
	ErrMyBookNotFound = errors.New("livro da estante não encontrado")
	ErrAlreadyInShelf = errors.New("livro já está na estante do usuário")
)

type Store interface {
	ListByUser(ctx context.Context, userID string) ([]models.UserBook, error)
	Create(ctx context.Context, userBook *models.UserBook) error
	GetByIDAndUser(ctx context.Context, id, userID string) (*models.UserBook, error)
	Update(ctx context.Context, userBook *models.UserBook) error
	DeleteByIDAndUser(ctx context.Context, id, userID string) error
	ExistsByUserAndBook(ctx context.Context, userID, bookID string) (bool, error)
	GetBookByID(ctx context.Context, id string) (*models.Book, error)
}

type Service struct {
	repo Store
}

func NewService(repo Store) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListByUser(ctx context.Context, userID string) ([]models.UserBook, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrInvalidInput
	}
	return s.repo.ListByUser(ctx, strings.TrimSpace(userID))
}

func (s *Service) AddToShelf(
	ctx context.Context,
	userID, bookID, status, notes string,
	rating *int,
	startedAt, finishedAt *time.Time,
) (*models.UserBook, error) {
	normalizedUserID := strings.TrimSpace(userID)
	normalizedBookID := strings.TrimSpace(bookID)
	normalizedStatus := strings.TrimSpace(status)

	if normalizedUserID == "" || normalizedBookID == "" || !isValidStatus(normalizedStatus) || !isValidRating(rating) {
		return nil, ErrInvalidInput
	}

	book, err := s.repo.GetBookByID(ctx, normalizedBookID)
	if err != nil {
		return nil, err
	}
	if book == nil {
		return nil, ErrBookNotFound
	}

	exists, err := s.repo.ExistsByUserAndBook(ctx, normalizedUserID, normalizedBookID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrAlreadyInShelf
	}

	now := time.Now().UTC()
	userBook := models.UserBook{
		UserID:     normalizedUserID,
		BookID:     normalizedBookID,
		Status:     normalizedStatus,
		Rating:     rating,
		Notes:      strings.TrimSpace(notes),
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.repo.Create(ctx, &userBook); err != nil {
		return nil, err
	}

	created, err := s.repo.GetByIDAndUser(ctx, userBook.ID, normalizedUserID)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, ErrMyBookNotFound
	}
	return created, nil
}

func (s *Service) UpdateShelfItem(
	ctx context.Context,
	userID, shelfID, status, notes string,
	rating *int,
	startedAt, finishedAt *time.Time,
) (*models.UserBook, error) {
	normalizedUserID := strings.TrimSpace(userID)
	normalizedShelfID := strings.TrimSpace(shelfID)
	normalizedStatus := strings.TrimSpace(status)

	if normalizedUserID == "" || normalizedShelfID == "" || !isValidStatus(normalizedStatus) || !isValidRating(rating) {
		return nil, ErrInvalidInput
	}

	current, err := s.repo.GetByIDAndUser(ctx, normalizedShelfID, normalizedUserID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrMyBookNotFound
	}

	current.Status = normalizedStatus
	current.Rating = rating
	current.Notes = strings.TrimSpace(notes)
	current.StartedAt = startedAt
	current.FinishedAt = finishedAt
	current.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, current); err != nil {
		return nil, err
	}

	updated, err := s.repo.GetByIDAndUser(ctx, current.ID, normalizedUserID)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, ErrMyBookNotFound
	}
	return updated, nil
}

func (s *Service) RemoveFromShelf(ctx context.Context, userID, shelfID string) error {
	normalizedUserID := strings.TrimSpace(userID)
	normalizedShelfID := strings.TrimSpace(shelfID)
	if normalizedUserID == "" || normalizedShelfID == "" {
		return ErrInvalidInput
	}

	current, err := s.repo.GetByIDAndUser(ctx, normalizedShelfID, normalizedUserID)
	if err != nil {
		return err
	}
	if current == nil {
		return ErrMyBookNotFound
	}

	if err := s.repo.DeleteByIDAndUser(ctx, normalizedShelfID, normalizedUserID); err != nil {
		return err
	}
	return nil
}

func isValidStatus(status string) bool {
	switch status {
	case models.UserBookStatusReading, models.UserBookStatusCompleted, models.UserBookStatusWantToRead, models.UserBookStatusAbandoned:
		return true
	default:
		return false
	}
}

func isValidRating(rating *int) bool {
	if rating == nil {
		return true
	}
	return *rating >= 1 && *rating <= 5
}
