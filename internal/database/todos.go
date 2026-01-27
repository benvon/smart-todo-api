package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/benvon/smart-todo/internal/models"
)

const (
	// MaxPageSize is the maximum page size for pagination queries
	MaxPageSize = 500
)

// TagChangeHandler handles tag change events (callback to avoid circular dependencies)
type TagChangeHandler func(ctx context.Context, userID uuid.UUID) error

// TodoRepository handles todo database operations
type TodoRepository struct {
	db                *DB
	tagStatsRepo      TagStatisticsRepositoryInterface // Optional: for automatic tag change detection
	tagChangeHandler  TagChangeHandler                 // Optional: callback when tags change
}

// NewTodoRepository creates a new todo repository
func NewTodoRepository(db *DB) *TodoRepository {
	return &TodoRepository{db: db}
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
		return nil, fmt.Errorf("todo not found: %w", err)
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

// GetByUserID retrieves all todos for a user, optionally filtered by time_horizon and status
// Deprecated: Use GetByUserIDPaginated for better performance with large datasets
func (r *TodoRepository) GetByUserID(ctx context.Context, userID uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus) ([]*models.Todo, error) {
	todos, _, err := r.GetByUserIDPaginated(ctx, userID, timeHorizon, status, 1, MaxPageSize)
	return todos, err
}

// GetByUserIDPaginated retrieves todos for a user with pagination support
func (r *TodoRepository) GetByUserIDPaginated(ctx context.Context, userID uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error) {
	// Build base query for counting
	countQuery := `SELECT COUNT(*) FROM todos WHERE user_id = $1`
	countArgs := []any{userID}
	argIndex := 2

	// Build WHERE clause for filtering
	whereClause := "WHERE user_id = $1"
	if timeHorizon != nil {
		whereClause += fmt.Sprintf(" AND time_horizon = $%d", argIndex)
		countQuery += fmt.Sprintf(" AND time_horizon = $%d", argIndex)
		countArgs = append(countArgs, string(*timeHorizon))
		argIndex++
	}

	if status != nil {
		whereClause += fmt.Sprintf(" AND status = $%d", argIndex)
		countQuery += fmt.Sprintf(" AND status = $%d", argIndex)
		countArgs = append(countArgs, string(*status))
		argIndex++
	}

	// Get total count
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count todos: %w", err)
	}

	// Build main query with pagination
	query := fmt.Sprintf(`
		SELECT id, user_id, text, time_horizon, status, metadata, due_date, created_at, updated_at, completed_at
		FROM todos
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args := countArgs
	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query todos: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Log error but continue - rows may already be closed
			// This is in database layer, logging would require passing logger
			// The error is non-critical as rows are already processed
			_ = err // Explicitly ignore error to satisfy linter
		}
	}()

	var todos []*models.Todo
	for rows.Next() {
		todo := &models.Todo{}
		var metadataJSON []byte
		var completedAt sql.NullTime
		var dueDate sql.NullTime

		err := rows.Scan(
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
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan todo: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &todo.Metadata); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal metadata: %w", err)
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

		todos = append(todos, todo)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating todos: %w", err)
	}

	return todos, total, nil
}

// Update updates an existing todo
func (r *TodoRepository) Update(ctx context.Context, todo *models.Todo) error {
	// Detect tag changes if tag statistics support is enabled
	var oldTags []string
	var newTags []string
	var tagsChanged bool
	if r.tagStatsRepo != nil {
		// Load existing todo to compare tags
		existing, err := r.GetByID(ctx, todo.ID)
		if err == nil {
			// Normalize nil slices to empty slices for comparison
			if existing.Metadata.CategoryTags == nil {
				oldTags = []string{}
			} else {
				oldTags = existing.Metadata.CategoryTags
			}
			if todo.Metadata.CategoryTags == nil {
				newTags = []string{}
			} else {
				newTags = todo.Metadata.CategoryTags
			}
			tagsChanged = !tagsEqual(oldTags, newTags)
			if tagsChanged {
				log.Printf("Tag change detected for todo %s (user %s): old=%v, new=%v", todo.ID, todo.UserID, oldTags, newTags)
			} else {
				log.Printf("No tag change for todo %s (user %s): tags=%v", todo.ID, todo.UserID, newTags)
			}
		} else {
			// If GetByID fails, we can't compare tags, so we'll skip tag change detection
			// This might happen if the todo was just created and hasn't been persisted yet
			log.Printf("Could not load existing todo %s for tag comparison: %v (skipping tag change detection)", todo.ID, err)
		}
	} else {
		log.Printf("Tag stats repo not configured for todo repository, skipping tag change detection")
	}

	query := `
		UPDATE todos
		SET text = $2, time_horizon = $3, status = $4, metadata = $5, due_date = $6, updated_at = $7, completed_at = $8
		WHERE id = $1
		RETURNING updated_at
	`
	
	metadataJSON, err := json.Marshal(todo.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	var dueDate sql.NullTime
	if todo.DueDate != nil {
		dueDate = sql.NullTime{Time: *todo.DueDate, Valid: true}
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
		dueDate,
		now,
		completedAt,
	).Scan(&todo.UpdatedAt)
	
	if err == sql.ErrNoRows {
		return fmt.Errorf("todo not found")
	}
	if err != nil {
		return fmt.Errorf("failed to update todo: %w", err)
	}

	// If tags changed, invoke the tag change handler
	if tagsChanged && r.tagChangeHandler != nil {
		log.Printf("Invoking tag change handler for user %s (todo %s)", todo.UserID, todo.ID)
		if err := r.tagChangeHandler(ctx, todo.UserID); err != nil {
			// Log error but don't fail the update
			// Tag analysis can happen later
			log.Printf("Tag change handler failed for user %s: %v", todo.UserID, err)
		} else {
			log.Printf("Tag change handler completed successfully for user %s", todo.UserID)
		}
	} else if tagsChanged && r.tagChangeHandler == nil {
		log.Printf("Tags changed for todo %s but no tag change handler configured", todo.ID)
	}
	
	return nil
}

// tagsEqual compares two tag slices for equality (order-independent)
// Handles nil slices as empty slices
func tagsEqual(a, b []string) bool {
	// Normalize nil to empty slice
	if a == nil {
		a = []string{}
	}
	if b == nil {
		b = []string{}
	}
	
	if len(a) != len(b) {
		return false
	}
	
	// Both empty - equal
	if len(a) == 0 {
		return true
	}
	
	// Create maps for O(n) comparison
	mapA := make(map[string]int)
	mapB := make(map[string]int)
	
	for _, tag := range a {
		mapA[tag]++
	}
	for _, tag := range b {
		mapB[tag]++
	}
	
	if len(mapA) != len(mapB) {
		return false
	}
	
	for tag, count := range mapA {
		if mapB[tag] != count {
			return false
		}
	}
	
	return true
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
