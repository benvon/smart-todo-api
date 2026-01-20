package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/benvon/smart-todo/internal/models"
)

// TodoRepositoryInterface defines the interface for todo repository operations
// This interface enables better testability by allowing mock implementations
type TodoRepositoryInterface interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Todo, error)
	Update(ctx context.Context, todo *models.Todo) error
	GetByUserIDPaginated(ctx context.Context, userID uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error)
}

// AIContextRepositoryInterface defines the interface for AI context repository operations
type AIContextRepositoryInterface interface {
	GetByUserID(ctx context.Context, userID uuid.UUID) (*models.AIContext, error)
}

// UserActivityRepositoryInterface defines the interface for user activity repository operations
type UserActivityRepositoryInterface interface {
	GetByUserID(ctx context.Context, userID uuid.UUID) (*models.UserActivity, error)
	GetEligibleUsersForReprocessing(ctx context.Context) ([]uuid.UUID, error)
}

// Ensure concrete types implement the interfaces
var (
	_ TodoRepositoryInterface        = (*TodoRepository)(nil)
	_ AIContextRepositoryInterface   = (*AIContextRepository)(nil)
	_ UserActivityRepositoryInterface = (*UserActivityRepository)(nil)
)
