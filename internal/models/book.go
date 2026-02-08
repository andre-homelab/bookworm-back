package models

import (
	"time"

	"gorm.io/gorm"
)

type Book struct {
	ID         string         `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Title      string         `gorm:"not null" json:"title"`
	Author     string         `gorm:"not null" json:"author"`
	ISBN       string         `gorm:"type:varchar(20);index" json:"isbn,omitempty"`
	Pages      int            `gorm:"not null;default:0" json:"pages"`
	Read       bool           `gorm:"not null;default:false" json:"read"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}
