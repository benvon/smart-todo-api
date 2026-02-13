package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/benvon/smart-todo/internal/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ErrTodoNotFound is returned when a todo is not found (e.g. by id or by user_id+id).
var ErrTodoNotFound = errors.New("todo not found")

const (
	// MaxPageSize is the maximum page size for pagination queries
	MaxPageSize = 500
)

// TagChangeHandler handles tag change events (callback to avoid circular dependencies)
type TagChangeHandler func(ctx context.Context, userID uuid.UUID) error

// TodoRepository handles todo database operations
type TodoRepository struct {
	db               *DB
	tagStatsRepo     TagStatisticsRepositoryInterface // Optional: for automatic tag change detection
	tagChangeHandler TagChangeHandler                 // Optional: callback when tags change
	logger           *zap.Logger                      // Optional: for structured logging
}

// NewTodoRepository creates a new todo repository
func NewTodoRepository(db *DB) *TodoRepository {
	return &TodoRepository{db: db}
}

// SetLogger sets the logger for the repository
func (r *TodoRepository) SetLogger(logger *zap.Logger) {
	r.logger = logger
}

// SetTagChangeHandler sets a callback to be invoked when tags change
func (r *TodoRepository) SetTagChangeHandler(handler TagChangeHandler) {
	r.tagChangeHandler = handler
}

// SetTagStatsRepo sets the tag statistics repository for automatic tag change detection
func (r *TodoRepository) SetTagStatsRepo(repo TagStatisticsRepositoryInterface) {
	r.tagStatsRepo = repo
}

// Create creates a new todo
func (r *TodoRepository) Create(ctx context.Context, todo *models.Todo) error {
	query := `
		INSERT INTO todos (id, user_id, text, time_horizon, status, metadata, due_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`

	metadataJSON, err := json.Marshal(todo.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	var dueDate sql.NullTime
	if todo.DueDate != nil {
		dueDate = sql.NullTime{Time: *todo.DueDate, Valid: true}
	}

	now := time.Now()
	err = r.db.QueryRowContext(ctx, query,
		todo.ID,
		todo.UserID,
		todo.Text,
		todo.TimeHorizon,
		todo.Status,
		metadataJSON,
		dueDate,
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
	var dueDate sql.NullTime

	query := `
		SELECT id, user_id, text, time_horizon, status, metadata, due_date, created_at, updated_at, completed_at
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
		&dueDate,
		&todo.CreatedAt,
		&todo.UpdatedAt,
		&completedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrTodoNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get todo: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &todo.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	// Initialize TagSources if nil
	if todo.Metadata.TagSources == nil {
		todo.Metadata.TagSources = make(map[string]models.TagSource)
	}

	if dueDate.Valid {
		todo.DueDate = &dueDate.Time
	}

	if completedAt.Valid {
		todo.CompletedAt = &completedAt.Time
	}

	return todo, nil
}

// GetByUserIDAndID retrieves a todo by user ID and todo ID. Enforces tenant scope at the DB layer.
// Use this for request-scoped access instead of GetByID to prevent cross-user data access.
func (r *TodoRepository) GetByUserIDAndID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*models.Todo, error) {
	todo := &models.Todo{}
	var metadataJSON []byte
	var completedAt sql.NullTime
	var dueDate sql.NullTime

	query := `
		SELECT id, user_id, text, time_horizon, status, metadata, due_date, created_at, updated_at, completed_at
		FROM todos
		WHERE user_id = $1 AND id = $2
	`

	err := r.db.QueryRowContext(ctx, query, userID, id).Scan(
		&todo.ID,
		&todo.UserID,
		&todo.Text,
		&todo.TimeHorizon,
		&todo.Status,
		&metadataJSON,
		&dueDate,
		&todo.CreatedAt,
		&todo.UpdatedAt,
		&completedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrTodoNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get todo: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &todo.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	if todo.Metadata.TagSources == nil {
		todo.Metadata.TagSources = make(map[string]models.TagSource)
	}

	if dueDate.Valid {
		todo.DueDate = &dueDate.Time
	}

	if completedAt.Valid {
		todo.CompletedAt = &completedAt.Time
	}

	return todo, nil
}

// GetByUserID retrieves all todos for a user, optionally filtered by time_horizon and status
// Deprecated: Use GetByUserIDPaginated for better performance with large datasets
func (r *TodoRepository) GetByUserID(ctx context.Context, userID uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus) ([]*models.Todo, error) {
	todos, _, err := r.GetByUserIDPaginated(ctx, userID, timeHorizon, status, 1, MaxPageSize)
	return todos, err
}

// GetByUserIDPaginated retrieves todos for a user with pagination support
func (r *TodoRepository) GetByUserIDPaginated(ctx context.Context, userID uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error) {
	whereClause, countQuery, countArgs, argIndex := buildTodoListWhereClause(userID, timeHorizon, status)

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count todos: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, text, time_horizon, status, metadata, due_date, created_at, updated_at, completed_at
		FROM todos
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)
	args := append(append([]any(nil), countArgs...), pageSize, (page-1)*pageSize)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query todos: %w", err)
	}
	defer func() { _ = rows.Close() }()

	todos, err := scanTodoRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return todos, total, nil
}

// buildTodoListWhereClause builds WHERE clause and count query for todo list filtering.
func buildTodoListWhereClause(userID uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus) (whereClause, countQuery string, args []any, nextArgIndex int) {
	whereClause = "WHERE user_id = $1"
	countQuery = "SELECT COUNT(*) FROM todos WHERE user_id = $1"
	args = []any{userID}
	nextArgIndex = 2
	if timeHorizon != nil {
		whereClause += fmt.Sprintf(" AND time_horizon = $%d", nextArgIndex)
		countQuery += fmt.Sprintf(" AND time_horizon = $%d", nextArgIndex)
		args = append(args, string(*timeHorizon))
		nextArgIndex++
	}
	if status != nil {
		whereClause += fmt.Sprintf(" AND status = $%d", nextArgIndex)
		countQuery += fmt.Sprintf(" AND status = $%d", nextArgIndex)
		args = append(args, string(*status))
		nextArgIndex++
	}
	return whereClause, countQuery, args, nextArgIndex
}

// scanTodoRows scans all rows into todos. Caller must close rows.
func scanTodoRows(rows *sql.Rows) ([]*models.Todo, error) {
	var todos []*models.Todo
	for rows.Next() {
		todo, err := scanTodoRow(rows)
		if err != nil {
			return nil, err
		}
		todos = append(todos, todo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating todos: %w", err)
	}
	return todos, nil
}

// scanTodoRow scans the current row into a Todo.
func scanTodoRow(rows *sql.Rows) (*models.Todo, error) {
	todo := &models.Todo{}
	var metadataJSON []byte
	var completedAt sql.NullTime
	var dueDate sql.NullTime
	if err := rows.Scan(
		&todo.ID,
		&todo.UserID,
		&todo.Text,
		&todo.TimeHorizon,
		&todo.Status,
		&metadataJSON,
		&dueDate,
		&todo.CreatedAt,
		&todo.UpdatedAt,
		&completedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan todo: %w", err)
	}
	if err := json.Unmarshal(metadataJSON, &todo.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	if todo.Metadata.TagSources == nil {
		todo.Metadata.TagSources = make(map[string]models.TagSource)
	}
	if dueDate.Valid {
		todo.DueDate = &dueDate.Time
	}
	if completedAt.Valid {
		todo.CompletedAt = &completedAt.Time
	}
	return todo, nil
}

// Update updates an existing todo
// oldTags should be the CategoryTags from the existing todo before the update (pass nil to skip tag change detection)
func (r *TodoRepository) Update(ctx context.Context, todo *models.Todo, oldTags []string) error {
	tagsChanged := r.detectAndLogTagChange(todo, oldTags)

	metadataJSON, err := json.Marshal(todo.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	dueDate := todoDueDateNullTime(todo.DueDate)
	completedAt := todoCompletedAtNullTime(todo.CompletedAt)
	now := time.Now()

	query := `
		UPDATE todos
		SET text = $2, time_horizon = $3, status = $4, metadata = $5, due_date = $6, updated_at = $7, completed_at = $8
		WHERE id = $1 AND user_id = $9
		RETURNING updated_at
	`
	err = r.db.QueryRowContext(ctx, query,
		todo.ID, todo.Text, todo.TimeHorizon, todo.Status,
		metadataJSON, dueDate, now, completedAt, todo.UserID,
	).Scan(&todo.UpdatedAt)
	if err == sql.ErrNoRows {
		return ErrTodoNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to update todo: %w", err)
	}

	r.invokeTagChangeHandlerIfNeeded(ctx, todo, tagsChanged)
	return nil
}

func (r *TodoRepository) detectAndLogTagChange(todo *models.Todo, oldTags []string) bool {
	if r.tagStatsRepo == nil || oldTags == nil {
		return false
	}
	changed := !tagsEqual(oldTags, todo.Metadata.CategoryTags)
	if changed && r.logger != nil {
		r.logger.Debug("tag_change_detected",
			zap.String("todo_id", todo.ID.String()),
			zap.String("user_id", todo.UserID.String()),
			zap.Strings("old_tags", oldTags),
			zap.Strings("new_tags", todo.Metadata.CategoryTags),
		)
	}
	return changed
}

func todoDueDateNullTime(d *time.Time) sql.NullTime {
	if d == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *d, Valid: true}
}

func todoCompletedAtNullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func (r *TodoRepository) invokeTagChangeHandlerIfNeeded(ctx context.Context, todo *models.Todo, tagsChanged bool) {
	if !tagsChanged || r.tagChangeHandler == nil {
		return
	}
	if r.logger != nil {
		r.logger.Debug("invoking_tag_change_handler",
			zap.String("todo_id", todo.ID.String()),
			zap.String("user_id", todo.UserID.String()),
		)
	}
	if err := r.tagChangeHandler(ctx, todo.UserID); err != nil {
		if r.logger != nil {
			r.logger.Warn("tag_change_handler_failed",
				zap.String("user_id", todo.UserID.String()),
				zap.String("todo_id", todo.ID.String()),
				zap.Error(err),
			)
		}
		return
	}
	if r.logger != nil {
		r.logger.Debug("tag_change_handler_completed",
			zap.String("user_id", todo.UserID.String()),
			zap.String("todo_id", todo.ID.String()),
		)
	}
}

// tagsEqual compares two tag slices for equality (order-independent)
// Handles nil slices as empty slices
func tagsEqual(a, b []string) bool {
	if a == nil {
		a = []string{}
	}
	if b == nil {
		b = []string{}
	}
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	return tagCountsEqual(tagCounts(a), tagCounts(b))
}

// tagCounts returns a map of tag -> count for the slice (handles duplicates).
func tagCounts(s []string) map[string]int {
	m := make(map[string]int)
	for _, tag := range s {
		m[tag]++
	}
	return m
}

// tagCountsEqual reports whether two tag-count maps are equal.
func tagCountsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for tag, count := range a {
		if b[tag] != count {
			return false
		}
	}
	return true
}

// Delete deletes a todo by user ID and todo ID. Enforces tenant scope at the DB layer.
func (r *TodoRepository) Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	query := `DELETE FROM todos WHERE id = $1 AND user_id = $2`

	result, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete todo: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrTodoNotFound
	}

	return nil
}
