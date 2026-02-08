package env

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("aviso: não foi possível carregar .env:", err)
	}
}

func Get(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
