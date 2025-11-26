package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/derekmwright/todoapp/internal/handlers"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	db, err := pgxpool.New(context.Background(), os.Getenv("DB_URL"))
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}

	h := handlers.New(logger)

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Get("/", h.FilteredTodos(db))
	r.Route("/todos", func(r chi.Router) {
		r.Use(h.CheckSession)
		r.Post("/", h.CreateTodo(db))
		r.Get("/new", h.NewTodo())
		r.Route("/{todoID}", func(r chi.Router) {
			r.Use(h.TodoCtx(db))
			r.Put("/", h.UpdateTodo(db))
			r.Delete("/", h.DeleteTodo(db))
			r.Get("/edit", h.EditTodo())
			r.Post("/completed", h.CompleteTodo(db))
		})
	})
	r.Route("/endpoint", func(r chi.Router) {
		r.Use(h.CheckSession)
		r.Get("/", h.APIEndpoint())
	})

	if err = http.ListenAndServe(":8080", r); err != nil {
		logger.Error("failed to start http server", "error", err)
		os.Exit(1)
	}
}
