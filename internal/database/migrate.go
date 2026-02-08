package database

import (
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/models"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS pgcrypto;").Error; err != nil {
		return err
	}

	return db.AutoMigrate(&models.Book{}, &models.User{}, &models.UserBook{})
}
