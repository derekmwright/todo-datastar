package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/derekmwright/todoapp/internal/store"
)

func (h *Handlers) CheckSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := r.Cookie(SessionCookie)
		if err != nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *Handlers) TodoCtx(db *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			idStr := chi.URLParam(r, "todoID")
			id, err := strconv.ParseInt(idStr, 10, 32)
			if err != nil {
				http.NotFound(w, r)
				return
			}

			ts := store.NewTodoStore(db, h.logger)
			todo, err := ts.GetByID(r.Context(), int32(id))
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					http.NotFound(w, r)
					return
				}
				h.internalServerError(w, err)
				return
			}

			ctx := context.WithValue(r.Context(), "todoCtx", todo)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
