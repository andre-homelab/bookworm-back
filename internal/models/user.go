package models

import (
	"time"
)

type User struct {
	ID            string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Email         string     `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	PasswordHash  string     `gorm:"column:password_hash;not null" json:"-"`
	Name          string     `gorm:"type:varchar(100);not null" json:"name"`
	LastLoginAt   *time.Time `gorm:"index" json:"last_login_at,omitempty"`
	IsActive      bool       `gorm:"not null;default:true;index" json:"is_active"`
	EmailVerified bool       `gorm:"not null;default:false" json:"email_verified"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
