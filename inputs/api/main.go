package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	_ "github.com/andre-felipe-wonsik-alves/bookworm-back/docs"
	bookapi "github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/book/api"
	mybookapi "github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/mybook/api"
	userapi "github.com/andre-felipe-wonsik-alves/bookworm-back/internal/controllers/user/api"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title           Bookworm API
// @version         1.0
// @description     API REST para gerenciamento de livros pessoais
// @termsOfService  http://swagger.io/terms/

// @license.name  MIT
// @license.url   http://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /api/v1
func Execute(
	ctx context.Context,
	bookService *bookapi.Service,
	userService *userapi.Service,
	myBookService *mybookapi.Service,
	authMiddleware *userapi.AuthMiddleware,
) error {
	bookHandler := bookapi.NewBookHandler(bookService)
	userHandler := userapi.NewUserHandler(userService)
	myBookHandler := mybookapi.NewMyBookHandler(myBookService)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.RequestID)
	r.Use(securityHeaders)

	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("http://localhost:8080/swagger/doc.json"),
	))

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", userHandler.Register)
			r.Post("/login", userHandler.Login)
			r.Post("/refresh", userHandler.Refresh)
		})

		r.Group(func(private chi.Router) {
			private.Use(authMiddleware.RequireAuth)

			private.Post("/auth/logout", userHandler.Logout)

			private.Route("/users", func(r chi.Router) {
				r.Get("/me", userHandler.GetMe)
				r.Put("/me", userHandler.UpdateMe)
				r.Put("/me/password", userHandler.UpdatePassword)
			})

			private.Route("/books", func(r chi.Router) {
				r.Get("/", bookHandler.ListBooks)
				r.Post("/", bookHandler.CreateBook)
				r.Put("/{id}", bookHandler.UpdateBook)
				r.Delete("/{id}", bookHandler.DeleteBook)
				r.Get("/search", bookHandler.SearchByTitleOrAuthor)
				r.Get("/search/isbn/{isbn}", bookHandler.SearchByISBN)
			})

			private.Route("/my-books", func(r chi.Router) {
				r.Get("/", myBookHandler.ListMyBooks)
				r.Post("/", myBookHandler.AddMyBook)
				r.Put("/{id}", myBookHandler.UpdateMyBook)
				r.Delete("/{id}", myBookHandler.DeleteMyBook)
			})
		})
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	log.Println("servidor rodando em http://localhost:8080")
	log.Println("swagger em http://localhost:8080/swagger/index.html")

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}

	return nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
