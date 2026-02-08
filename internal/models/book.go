package models

import (
	"time"

	"gorm.io/gorm"
)

type Book struct {
	ID            string         `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Title         string         `gorm:"not null;index" json:"title"`
	Author        string         `gorm:"not null;index" json:"author"`
	ISBN          string         `gorm:"type:varchar(20);index" json:"isbn,omitempty"`
	Description   string         `gorm:"type:text" json:"description,omitempty"`
	CoverURL      string         `gorm:"type:text" json:"cover_url,omitempty"`
	Publisher     string         `gorm:"type:varchar(255)" json:"publisher,omitempty"`
	PublishedYear int            `gorm:"index" json:"published_year,omitempty"`
	Pages         int            `gorm:"not null;default:0" json:"pages"`
	Read          bool           `gorm:"not null;default:false" json:"read"`
	FinishedAt    *time.Time     `json:"finished_at,omitempty"`
	CacheSource   string         `gorm:"type:varchar(50);index" json:"cache_source,omitempty"`
	CachedAt      *time.Time     `gorm:"index" json:"cached_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}
