package store

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Todo struct {
	ID          int32
	Name        string
	Description string
	Done        bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time
}

type TodoStore struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

func NewTodoStore(db *pgxpool.Pool, logger *slog.Logger) *TodoStore {
	return &TodoStore{
		db:     db,
		logger: logger,
	}
}

type TodoFilter struct {
	Done      *bool
	StartDate time.Time
	EndDate   time.Time
}

func (t *TodoStore) List(ctx context.Context, filter TodoFilter) ([]Todo, error) {
	todos := make([]Todo, 0)
	rows, err := t.db.Query(
		ctx,
		`
SELECT id, name, description, done, created_at, updated_at, completed_at
FROM todos
WHERE done = COALESCE($1, done)
ORDER BY created_at
`,
		filter.Done,
	)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	for rows.Next() {
		todo := Todo{}
		if err = rows.Scan(
			&todo.ID,
			&todo.Name,
			&todo.Description,
			&todo.Done,
			&todo.CreatedAt,
			&todo.UpdatedAt,
			&todo.CompletedAt,
		); err != nil {
			return nil, err
		}

		todos = append(todos, todo)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return todos, nil
}

func (t *TodoStore) Insert(ctx context.Context, todo *Todo) error {
	if err := t.db.QueryRow(
		ctx,
		"INSERT INTO todos (name, description) VALUES ($1, $2) RETURNING id, created_at, updated_at, completed_at",
		todo.Name,
		todo.Description,
	).Scan(
		&todo.ID,
		&todo.CreatedAt,
		&todo.UpdatedAt,
		&todo.CompletedAt,
	); err != nil {
		return err
	}

	return nil
}

func (t *TodoStore) GetByID(ctx context.Context, id int32) (*Todo, error) {
	todo := &Todo{}
	if err := t.db.QueryRow(
		ctx,
		"SELECT id, name, description, done, created_at, updated_at, completed_at FROM todos WHERE id = $1",
		id,
	).Scan(
		&todo.ID,
		&todo.Name,
		&todo.Description,
		&todo.Done,
		&todo.CreatedAt,
		&todo.UpdatedAt,
		&todo.CompletedAt,
	); err != nil {
		return nil, err
	}

	return todo, nil
}

func (t *TodoStore) Update(ctx context.Context, todo *Todo) error {
	if err := t.db.QueryRow(
		ctx,
		"UPDATE todos SET name = $1, description = $2, updated_at = $3 WHERE id = $4 RETURNING updated_at",
		todo.Name,
		todo.Description,
		time.Now(),
		todo.ID,
	).Scan(
		&todo.UpdatedAt,
	); err != nil {
		return err
	}

	return nil
}

func (t *TodoStore) SetCompleted(ctx context.Context, id int32) error {
	if err := t.db.QueryRow(
		ctx,
		"UPDATE todos SET done = true, completed_at = $1 WHERE id = $2",
		time.Now(),
		id,
	).Scan(); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
	}

	return nil
}
