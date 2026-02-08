package models

import (
	"time"
)

const (
	UserBookStatusReading    = "reading"
	UserBookStatusCompleted  = "completed"
	UserBookStatusWantToRead = "want_to_read"
	UserBookStatusAbandoned  = "abandoned"
)

type UserBook struct {
	ID         string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID     string     `gorm:"type:uuid;not null;index;uniqueIndex:idx_user_book_unique" json:"user_id"`
	BookID     string     `gorm:"type:uuid;not null;index;uniqueIndex:idx_user_book_unique" json:"book_id"`
	Status     string     `gorm:"type:varchar(20);not null;index" json:"status"`
	Rating     *int       `gorm:"index" json:"rating,omitempty"`
	Notes      string     `gorm:"type:text" json:"notes,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Book       Book       `gorm:"foreignKey:BookID;references:ID;constraint:OnDelete:CASCADE" json:"book"`
}
