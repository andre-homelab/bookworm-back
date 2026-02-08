package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/inputs/api"
	bookapi "github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/book/api"
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/book/repository"
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/database"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect()
	if err != nil {
		log.Fatalf("erro na conexão com o banco: %v", err)
	}

	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("erro ao executar migrations: %v", err)
	}

	repo := repository.NewDBStore(db)
	service := bookapi.NewService(repo)

	if err := api.Execute(ctx, service); err != nil {
		log.Fatalf("erro ao executar API: %v", err)
	}
}
