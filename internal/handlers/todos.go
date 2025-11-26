package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starfederation/datastar-go/datastar"

	"github.com/derekmwright/todoapp/internal/components"
	"github.com/derekmwright/todoapp/internal/store"
)

func (h *Handlers) FilteredTodos(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter := store.TodoFilter{}

		inputFilter := &struct {
			Show string `json:"show"`
		}{}

		if err := datastar.ReadSignals(r, inputFilter); err != nil {
			h.internalServerError(w, err)
		}

		if inputFilter.Show == "all" {
			filter.Done = nil
		}

		if inputFilter.Show == "done" {
			done := true
			filter.Done = &done
		}

		if inputFilter.Show == "open" {
			done := false
			filter.Done = &done
		}

		if r.Header.Get("Datastar-Request") != "true" {
			// Default hard refresh to show only open todos
			done := false
			filter.Done = &done
		}

		ts := store.NewTodoStore(db, h.logger)
		todos, err := ts.List(r.Context(), filter)
		if err != nil {
			h.internalServerError(w, err)
		}

		if r.Header.Get("Datastar-Request") != "true" {
			if err = components.Layout("Todo App", components.TodoList(todos)).Render(r.Context(), w); err != nil {
				h.internalServerError(w, err)
			}
			return
		}

		h.renderView(components.TodoList(todos), nil, w, r)
	}
}

// Form Views
func (h *Handlers) NewTodo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Datastar-Request") != "true" {
			if err := components.Layout("Todo App", components.TodoNew()).Render(r.Context(), w); err != nil {
				h.internalServerError(w, err)
			}
			return
		}

		clearSignals := struct {
			Name        *string `json:"name"`
			Description *string `json:"description"`
		}{}

		h.renderView(components.TodoNew(), &clearSignals, w, r)
	}
}

func (h *Handlers) EditTodo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		todo := r.Context().Value("todoCtx").(*store.Todo)
		if r.Header.Get("Datastar-Request") != "true" {
			if err := components.Layout("Todo App", components.TodoEdit(todo)).Render(r.Context(), w); err != nil {
				h.internalServerError(w, err)
			}
			return
		}

		signals := struct {
			ID          int32  `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		}{
			ID:          todo.ID,
			Name:        todo.Name,
			Description: todo.Description,
		}

		h.renderView(components.TodoEdit(todo), signals, w, r)
	}
}

// CRUD actions
func (h *Handlers) CreateTodo(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input := struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}{}

		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			h.internalServerError(w, err)
		}

		if input.Name == "" {
			h.internalServerError(w, errors.New("name is required"))
			return
		}

		todo := &store.Todo{
			Name:        input.Name,
			Description: input.Description,
		}

		ts := store.NewTodoStore(db, h.logger)
		if err := ts.Insert(r.Context(), todo); err != nil {
			h.internalServerError(w, err)
		}

		go func() {
			session, err := r.Cookie(SessionCookie)
			if err != nil {
				return
			}
			ch := h.connections[session.Value]

			clearSignals := struct {
				Name        *string `json:"name"`
				Description *string `json:"description"`
			}{}

			ch <- SignalsUpdate{Signals: &clearSignals}
		}()

		h.FilteredTodos(db).ServeHTTP(w, r)
	}
}

func (h *Handlers) DeleteTodo(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {}
}

func (h *Handlers) UpdateTodo(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		todo := r.Context().Value("todoCtx").(*store.Todo)

		input := struct {
			ID          int32  `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		}{}

		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			h.internalServerError(w, err)
			return
		}

		if input.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}

		todo.Name = input.Name
		todo.Description = input.Description

		ts := store.NewTodoStore(db, h.logger)
		if err := ts.Update(r.Context(), todo); err != nil {
			h.internalServerError(w, err)
		}

		go func() {
			session, err := r.Cookie(SessionCookie)
			if err != nil {
				return
			}
			ch := h.connections[session.Value]

			clearSignals := struct {
				ID          *int32  `json:"id"`
				Name        *string `json:"name"`
				Description *string `json:"description"`
			}{}

			ch <- SignalsUpdate{Signals: &clearSignals}
		}()

		h.FilteredTodos(db).ServeHTTP(w, r)
	}
}

func (h *Handlers) CompleteTodo(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "todoID"), 10, 32)
		if err != nil {
			h.internalServerError(w, err)
		}

		ts := store.NewTodoStore(db, h.logger)
		if err = ts.SetCompleted(r.Context(), int32(id)); err != nil {
			h.internalServerError(w, err)
		}

		h.FilteredTodos(db).ServeHTTP(w, r)
	}
}
