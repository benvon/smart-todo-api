package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/benvon/smart-todo/internal/models"
)

// TodoRepository handles todo database operations
type TodoRepository struct {
	db *DB
}

// NewTodoRepository creates a new todo repository
func NewTodoRepository(db *DB) *TodoRepository {
	return &TodoRepository{db: db}
}

// Create creates a new todo
func (r *TodoRepository) Create(ctx context.Context, todo *models.Todo) error {
	query := `
		INSERT INTO todos (id, user_id, text, time_horizon, status, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at
	`
	
	metadataJSON, err := json.Marshal(todo.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	now := time.Now()
	err = r.db.QueryRowContext(ctx, query,
		todo.ID,
		todo.UserID,
		todo.Text,
		todo.TimeHorizon,
		todo.Status,
		metadataJSON,
		now,
		now,
	).Scan(&todo.CreatedAt, &todo.UpdatedAt)
	
	if err != nil {
		return fmt.Errorf("failed to create todo: %w", err)
	}
	
	return nil
}

// GetByID retrieves a todo by ID
func (r *TodoRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Todo, error) {
	todo := &models.Todo{}
	var metadataJSON []byte
	var completedAt sql.NullTime
	
	query := `
		SELECT id, user_id, text, time_horizon, status, metadata, created_at, updated_at, completed_at
		FROM todos
		WHERE id = $1
	`
	
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&todo.ID,
		&todo.UserID,
		&todo.Text,
		&todo.TimeHorizon,
		&todo.Status,
		&metadataJSON,
		&todo.CreatedAt,
		&todo.UpdatedAt,
		&completedAt,
	)
	
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("todo not found: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get todo: %w", err)
	}
	
	if err := json.Unmarshal(metadataJSON, &todo.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	
	if completedAt.Valid {
		todo.CompletedAt = &completedAt.Time
	}
	
	return todo, nil
}

// GetByUserID retrieves all todos for a user, optionally filtered by time_horizon and status
func (r *TodoRepository) GetByUserID(ctx context.Context, userID uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus) ([]*models.Todo, error) {
	query := `
		SELECT id, user_id, text, time_horizon, status, metadata, created_at, updated_at, completed_at
		FROM todos
		WHERE user_id = $1
	`
	args := []any{userID}
	argIndex := 2
	
	if timeHorizon != nil {
		query += fmt.Sprintf(" AND time_horizon = $%d", argIndex)
		args = append(args, string(*timeHorizon))
		argIndex++
	}
	
	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, string(*status))
		argIndex++
	}
	
	query += " ORDER BY created_at DESC"
	
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query todos: %w", err)
	}
	defer rows.Close()
	
	var todos []*models.Todo
	for rows.Next() {
		todo := &models.Todo{}
		var metadataJSON []byte
		var completedAt sql.NullTime
		
		err := rows.Scan(
			&todo.ID,
			&todo.UserID,
			&todo.Text,
			&todo.TimeHorizon,
			&todo.Status,
			&metadataJSON,
			&todo.CreatedAt,
			&todo.UpdatedAt,
			&completedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan todo: %w", err)
		}
		
		if err := json.Unmarshal(metadataJSON, &todo.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
		
		if completedAt.Valid {
			todo.CompletedAt = &completedAt.Time
		}
		
		todos = append(todos, todo)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating todos: %w", err)
	}
	
	return todos, nil
}

// Update updates an existing todo
func (r *TodoRepository) Update(ctx context.Context, todo *models.Todo) error {
	query := `
		UPDATE todos
		SET text = $2, time_horizon = $3, status = $4, metadata = $5, updated_at = $6, completed_at = $7
		WHERE id = $1
		RETURNING updated_at
	`
	
	metadataJSON, err := json.Marshal(todo.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	var completedAt sql.NullTime
	if todo.CompletedAt != nil {
		completedAt = sql.NullTime{Time: *todo.CompletedAt, Valid: true}
	}
	
	now := time.Now()
	err = r.db.QueryRowContext(ctx, query,
		todo.ID,
		todo.Text,
		todo.TimeHorizon,
		todo.Status,
		metadataJSON,
		now,
		completedAt,
	).Scan(&todo.UpdatedAt)
	
	if err == sql.ErrNoRows {
		return fmt.Errorf("todo not found")
	}
	if err != nil {
		return fmt.Errorf("failed to update todo: %w", err)
	}
	
	return nil
}

// Delete deletes a todo by ID
func (r *TodoRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM todos WHERE id = $1`
	
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete todo: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("todo not found")
	}
	
	return nil
}
