package database

import (
	"context"

	"github.com/benvon/smart-todo/internal/models"
	"github.com/google/uuid"
)

// TodoRepositoryInterface defines the interface for todo repository operations
// This interface enables better testability by allowing mock implementations
type TodoRepositoryInterface interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Todo, error)
	GetByUserIDAndID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*models.Todo, error)
	Update(ctx context.Context, todo *models.Todo, oldTags []string) error
	Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
	GetByUserIDPaginated(ctx context.Context, userID uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error)
	SetTagStatsRepo(repo TagStatisticsRepositoryInterface) // Optional: for tag change detection
	SetTagChangeHandler(handler TagChangeHandler)          // Optional: callback when tags change
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

// TagStatisticsRepositoryInterface defines the interface for tag statistics repository operations
type TagStatisticsRepositoryInterface interface {
	GetByUserID(ctx context.Context, userID uuid.UUID) (*models.TagStatistics, error)
	GetByUserIDOrCreate(ctx context.Context, userID uuid.UUID) (*models.TagStatistics, error)
	UpdateStatistics(ctx context.Context, stats *models.TagStatistics) (bool, error)
	MarkTainted(ctx context.Context, userID uuid.UUID) (bool, error)
}

// Ensure concrete types implement the interfaces
var (
	_ TodoRepositoryInterface          = (*TodoRepository)(nil)
	_ AIContextRepositoryInterface     = (*AIContextRepository)(nil)
	_ UserActivityRepositoryInterface  = (*UserActivityRepository)(nil)
	_ TagStatisticsRepositoryInterface = (*TagStatisticsRepository)(nil)
)
