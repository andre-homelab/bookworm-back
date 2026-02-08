package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/inputs/api"
	bookapi "github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/book/api"
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/book/cache"
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/book/provider"
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/book/repository"
	mybookapi "github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/mybook/api"
	mybookrepo "github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/mybook/repository"
	userapi "github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/user/api"
	userrepo "github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/user/repository"
	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/user/security"
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
	memoryCache := cache.NewLRUCache(1000)
	googleProvider := provider.NewGoogleBooksProvider(3 * time.Second)
	openLibraryProvider := provider.NewOpenLibraryProvider(3 * time.Second)
	externalProvider := provider.NewCompositeProvider(googleProvider, openLibraryProvider)
	bookService := bookapi.NewService(repo, memoryCache, externalProvider)

	userRepository := userrepo.NewDBStore(db)
	tokenManager, err := security.NewTokenManagerFromEnv()
	if err != nil {
		log.Fatalf("erro ao configurar tokens JWT: %v", err)
	}
	loginGuard := security.NewLoginGuard()
	userService, err := userapi.NewService(userRepository, tokenManager, loginGuard)
	if err != nil {
		log.Fatalf("erro ao configurar serviço de usuários: %v", err)
	}
	authMiddleware := userapi.NewAuthMiddleware(tokenManager, userService)
	myBookRepository := mybookrepo.NewDBStore(db)
	myBookService := mybookapi.NewService(myBookRepository)

	if err := api.Execute(ctx, bookService, userService, myBookService, authMiddleware); err != nil {
		log.Fatalf("erro ao executar API: %v", err)
	}
}
