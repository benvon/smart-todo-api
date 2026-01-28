package workers

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/queue"
	"github.com/benvon/smart-todo/internal/services/ai"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// mockAIProvider is a mock implementation of AIProvider
type mockAIProvider struct {
	t                          *testing.T
	analyzeTaskFunc            func(ctx context.Context, text string, userContext *models.AIContext) ([]string, models.TimeHorizon, error)
	analyzeTaskWithDueDateFunc func(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext, tagStats *models.TagStatistics) ([]string, models.TimeHorizon, error)

	// Call tracking
	analyzeTaskCalls []struct {
		text        string
		userContext *models.AIContext
	}
	analyzeTaskWithDueDateCalls []struct {
		text        string
		dueDate     *time.Time
		createdAt   time.Time
		userContext *models.AIContext
	}
}

func (m *mockAIProvider) AnalyzeTask(ctx context.Context, text string, userContext *models.AIContext) ([]string, models.TimeHorizon, error) {
	m.analyzeTaskCalls = append(m.analyzeTaskCalls, struct {
		text        string
		userContext *models.AIContext
	}{text, userContext})
	if m.analyzeTaskFunc == nil {
		m.t.Fatal("AnalyzeTask called but not configured in test - mock requires explicit setup")
	}
	return m.analyzeTaskFunc(ctx, text, userContext)
}

func (m *mockAIProvider) AnalyzeTaskWithDueDate(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext, tagStats *models.TagStatistics) ([]string, models.TimeHorizon, error) {
	m.analyzeTaskWithDueDateCalls = append(m.analyzeTaskWithDueDateCalls, struct {
		text        string
		dueDate     *time.Time
		createdAt   time.Time
		userContext *models.AIContext
	}{text, dueDate, createdAt, userContext})
	if m.analyzeTaskWithDueDateFunc == nil {
		// Fallback to analyzeTaskFunc if available
		if m.analyzeTaskFunc != nil {
			return m.analyzeTaskFunc(ctx, text, userContext)
		}
		m.t.Fatal("AnalyzeTaskWithDueDate called but not configured in test - mock requires explicit setup")
	}
	return m.analyzeTaskWithDueDateFunc(ctx, text, dueDate, createdAt, userContext, tagStats)
}

func (m *mockAIProvider) Chat(ctx context.Context, messages []ai.ChatMessage, userContext *models.AIContext) (*ai.ChatResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockAIProvider) SummarizeContext(ctx context.Context, conversationHistory []ai.ChatMessage) (string, error) {
	return "", errors.New("not implemented")
}

// Ensure mock implements AIProviderWithDueDate interface
var _ ai.AIProviderWithDueDate = (*mockAIProvider)(nil)

// mockTodoRepo is a mock implementation of TodoRepositoryInterface
type mockTodoRepo struct {
	t                        *testing.T
	getByIDFunc              func(ctx context.Context, id uuid.UUID) (*models.Todo, error)
	updateFunc               func(ctx context.Context, todo *models.Todo, oldTags []string) error
	getByUserIDPaginatedFunc func(ctx context.Context, userID uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error)

	// Call tracking (protected by mutex for concurrent access)
	mu                        sync.Mutex
	getByIDCalls              []uuid.UUID
	updateCalls               []*models.Todo
	getByUserIDPaginatedCalls []struct {
		userID         uuid.UUID
		timeHorizon    *models.TimeHorizon
		status         *models.TodoStatus
		page, pageSize int
	}
}

// SetTagStatsRepo is a no-op for the mock (tag change detection handled by concrete implementation)
func (m *mockTodoRepo) SetTagStatsRepo(repo database.TagStatisticsRepositoryInterface) {
	// No-op for mock
}

// SetTagChangeHandler is a no-op for the mock (tag change detection handled by concrete implementation)
func (m *mockTodoRepo) SetTagChangeHandler(handler database.TagChangeHandler) {
	// No-op for mock
}

func (m *mockTodoRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Todo, error) {
	m.mu.Lock()
	m.getByIDCalls = append(m.getByIDCalls, id)
	m.mu.Unlock()
	if m.getByIDFunc == nil {
		m.t.Fatal("GetByID called but not configured in test - mock requires explicit setup")
	}
	return m.getByIDFunc(ctx, id)
}

func (m *mockTodoRepo) Update(ctx context.Context, todo *models.Todo, oldTags []string) error {
	m.mu.Lock()
	m.updateCalls = append(m.updateCalls, todo)
	m.mu.Unlock()
	if m.updateFunc == nil {
		m.t.Fatal("Update called but not configured in test - mock requires explicit setup")
	}
	return m.updateFunc(ctx, todo, oldTags)
}

func (m *mockTodoRepo) GetByUserIDPaginated(ctx context.Context, userID uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error) {
	m.mu.Lock()
	m.getByUserIDPaginatedCalls = append(m.getByUserIDPaginatedCalls, struct {
		userID         uuid.UUID
		timeHorizon    *models.TimeHorizon
		status         *models.TodoStatus
		page, pageSize int
	}{userID, timeHorizon, status, page, pageSize})
	m.mu.Unlock()
	if m.getByUserIDPaginatedFunc == nil {
		m.t.Fatal("GetByUserIDPaginated called but not configured in test - mock requires explicit setup")
	}
	return m.getByUserIDPaginatedFunc(ctx, userID, timeHorizon, status, page, pageSize)
}

// Ensure mock implements interface
var _ database.TodoRepositoryInterface = (*mockTodoRepo)(nil)

// mockAIContextRepo is a mock implementation of AIContextRepositoryInterface
type mockAIContextRepo struct {
	t               *testing.T
	getByUserIDFunc func(ctx context.Context, userID uuid.UUID) (*models.AIContext, error)

	// Call tracking
	getByUserIDCalls []uuid.UUID
}

func (m *mockAIContextRepo) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.AIContext, error) {
	m.getByUserIDCalls = append(m.getByUserIDCalls, userID)
	if m.getByUserIDFunc == nil {
		// Default behavior: return nil (not found) - this is acceptable for tests that don't need context
		return nil, errors.New("not found")
	}
	return m.getByUserIDFunc(ctx, userID)
}

// Ensure mock implements interface
var _ database.AIContextRepositoryInterface = (*mockAIContextRepo)(nil)

// mockUserActivityRepo is a mock implementation of UserActivityRepositoryInterface
type mockUserActivityRepo struct {
	t                                   *testing.T
	getByUserIDFunc                     func(ctx context.Context, userID uuid.UUID) (*models.UserActivity, error)
	getEligibleUsersForReprocessingFunc func(ctx context.Context) ([]uuid.UUID, error)

	// Call tracking
	getByUserIDCalls      []uuid.UUID
	getEligibleUsersCalls int
}

func (m *mockUserActivityRepo) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.UserActivity, error) {
	m.getByUserIDCalls = append(m.getByUserIDCalls, userID)
	if m.getByUserIDFunc == nil {
		// Default behavior: return non-paused activity - this is acceptable for tests that don't need specific activity state
		return &models.UserActivity{
			UserID:             userID,
			ReprocessingPaused: false,
		}, nil
	}
	return m.getByUserIDFunc(ctx, userID)
}

func (m *mockUserActivityRepo) GetEligibleUsersForReprocessing(ctx context.Context) ([]uuid.UUID, error) {
	m.getEligibleUsersCalls++
	if m.getEligibleUsersForReprocessingFunc == nil {
		// Default behavior: return empty list - this is acceptable for tests that don't need eligible users
		return []uuid.UUID{}, nil
	}
	return m.getEligibleUsersForReprocessingFunc(ctx)
}

// Ensure mock implements interface
var _ database.UserActivityRepositoryInterface = (*mockUserActivityRepo)(nil)

// mockJobQueue is a mock implementation of JobQueue
type mockJobQueue struct {
	t           *testing.T
	enqueueFunc func(ctx context.Context, job *queue.Job) error

	// Call tracking
	enqueueCalls []*queue.Job
}

func (m *mockJobQueue) Enqueue(ctx context.Context, job *queue.Job) error {
	m.enqueueCalls = append(m.enqueueCalls, job)
	if m.enqueueFunc == nil {
		// Default behavior: succeed silently - this is acceptable for tests that don't need to verify enqueue behavior
		return nil
	}
	return m.enqueueFunc(ctx, job)
}

func (m *mockJobQueue) Dequeue(ctx context.Context) (*queue.Message, error) {
	return nil, errors.New("not implemented")
}

func (m *mockJobQueue) Consume(ctx context.Context, prefetchCount int) (<-chan *queue.Message, <-chan error, error) {
	return nil, nil, errors.New("not implemented")
}

func (m *mockJobQueue) Close() error {
	return nil
}

func (m *mockJobQueue) HealthCheck(ctx context.Context) error {
	return nil
}

// Ensure mock implements interface
var _ queue.JobQueue = (*mockJobQueue)(nil)

// mockMessage is a mock implementation of MessageInterface
type mockMessage struct {
	job      *queue.Job
	ackFunc  func() error
	nackFunc func(requeue bool) error
}

func (m *mockMessage) Ack() error {
	if m.ackFunc != nil {
		return m.ackFunc()
	}
	return nil
}

func (m *mockMessage) Nack(requeue bool) error {
	if m.nackFunc != nil {
		return m.nackFunc(requeue)
	}
	return nil
}

func (m *mockMessage) GetJob() *queue.Job {
	return m.job
}

// Ensure mock implements interface
var _ queue.MessageInterface = (*mockMessage)(nil)

func TestTaskAnalyzer_ProcessTaskAnalysisJob(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	todoID := uuid.New()

	tests := []struct {
		name         string
		job          *queue.Job
		setupMocks   func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue)
		expectError  bool
		validateTodo func(*testing.T, *models.Todo)
	}{
		{
			name: "successful analysis",
			job: &queue.Job{
				ID:     uuid.New(),
				Type:   queue.JobTypeTaskAnalysis,
				UserID: userID,
				TodoID: &todoID,
			},
			setupMocks: func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue) {
				aiProvider := &mockAIProvider{
					analyzeTaskWithDueDateFunc: func(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext, tagStats *models.TagStatistics) ([]string, models.TimeHorizon, error) {
						return []string{"work", "urgent"}, models.TimeHorizonSoon, nil
					},
				}
				todoRepo := &mockTodoRepo{
					getByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Todo, error) {
						return &models.Todo{
							ID:          id,
							UserID:      userID,
							Text:        "Complete project",
							Status:      models.TodoStatusPending,
							TimeHorizon: models.TimeHorizonNext,
							Metadata:    models.Metadata{},
						}, nil
					},
					updateFunc: func(ctx context.Context, todo *models.Todo, oldTags []string) error {
						return nil
					},
				}
				return aiProvider, todoRepo, &mockAIContextRepo{}, &mockUserActivityRepo{}, &mockJobQueue{}
			},
			expectError: false,
			validateTodo: func(t *testing.T, todo *models.Todo) {
				if todo.Status != models.TodoStatusProcessed {
					t.Errorf("Expected status to be processed, got %s", todo.Status)
				}
				if todo.TimeHorizon != models.TimeHorizonSoon {
					t.Errorf("Expected time horizon to be soon, got %s", todo.TimeHorizon)
				}
			},
		},
		{
			name: "preserves user-set time horizon",
			job: &queue.Job{
				ID:     uuid.New(),
				Type:   queue.JobTypeTaskAnalysis,
				UserID: userID,
				TodoID: &todoID,
			},
			setupMocks: func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue) {
				aiProvider := &mockAIProvider{
					analyzeTaskWithDueDateFunc: func(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext, tagStats *models.TagStatistics) ([]string, models.TimeHorizon, error) {
						// AI suggests "later" but user has set it to "next"
						return []string{"work"}, models.TimeHorizonLater, nil
					},
				}
				override := true
				todoRepo := &mockTodoRepo{
					getByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Todo, error) {
						return &models.Todo{
							ID:          id,
							UserID:      userID,
							Text:        "Complete project",
							Status:      models.TodoStatusPending,
							TimeHorizon: models.TimeHorizonNext, // User set this
							Metadata: models.Metadata{
								TimeHorizonUserOverride: &override, // User override flag set
							},
						}, nil
					},
					updateFunc: func(ctx context.Context, todo *models.Todo, oldTags []string) error {
						return nil
					},
				}
				return aiProvider, todoRepo, &mockAIContextRepo{}, &mockUserActivityRepo{}, &mockJobQueue{}
			},
			expectError: false,
			validateTodo: func(t *testing.T, todo *models.Todo) {
				// Should preserve user-set time horizon even though AI suggests different
				if todo.TimeHorizon != models.TimeHorizonNext {
					t.Errorf("Expected time horizon to be preserved as 'next' (user-set), got %s", todo.TimeHorizon)
				}
				if todo.Status != models.TodoStatusProcessed {
					t.Errorf("Expected status to be processed, got %s", todo.Status)
				}
			},
		},
		{
			name: "missing todo_id",
			job: &queue.Job{
				ID:     uuid.New(),
				Type:   queue.JobTypeTaskAnalysis,
				UserID: userID,
				TodoID: nil,
			},
			setupMocks: func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue) {
				// No mocks need to be configured - error happens before any calls
				return &mockAIProvider{}, &mockTodoRepo{}, &mockAIContextRepo{}, &mockUserActivityRepo{}, &mockJobQueue{}
			},
			expectError: true,
		},
		{
			name: "todo not found",
			job: &queue.Job{
				ID:     uuid.New(),
				Type:   queue.JobTypeTaskAnalysis,
				UserID: userID,
				TodoID: &todoID,
			},
			setupMocks: func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue) {
				todoRepo := &mockTodoRepo{
					getByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Todo, error) {
						return nil, errors.New("not found")
					},
				}
				// Other mocks don't need configuration - error happens before they're called
				return &mockAIProvider{}, todoRepo, &mockAIContextRepo{}, &mockUserActivityRepo{}, &mockJobQueue{}
			},
			expectError: true,
		},
		{
			name: "todo belongs to different user",
			job: &queue.Job{
				ID:     uuid.New(),
				Type:   queue.JobTypeTaskAnalysis,
				UserID: userID,
				TodoID: &todoID,
			},
			setupMocks: func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue) {
				todoRepo := &mockTodoRepo{
					getByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Todo, error) {
						return &models.Todo{
							ID:     id,
							UserID: uuid.New(), // Different user
							Text:   "Test todo",
						}, nil
					},
				}
				// Other mocks don't need configuration - error happens before they're called
				return &mockAIProvider{}, todoRepo, &mockAIContextRepo{}, &mockUserActivityRepo{}, &mockJobQueue{}
			},
			expectError: true,
		},
		{
			name: "reprocessing paused",
			job: &queue.Job{
				ID:     uuid.New(),
				Type:   queue.JobTypeTaskAnalysis,
				UserID: userID,
				TodoID: &todoID,
			},
			setupMocks: func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue) {
				todoRepo := &mockTodoRepo{
					getByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Todo, error) {
						return &models.Todo{
							ID:     id,
							UserID: userID,
							Text:   "Test todo",
						}, nil
					},
				}
				activityRepo := &mockUserActivityRepo{
					getByUserIDFunc: func(ctx context.Context, userID uuid.UUID) (*models.UserActivity, error) {
						return &models.UserActivity{
							UserID:             userID,
							ReprocessingPaused: true,
						}, nil
					},
				}
				// AI provider and context repo don't need configuration - processing stops early
				return &mockAIProvider{}, todoRepo, &mockAIContextRepo{}, activityRepo, &mockJobQueue{}
			},
			expectError: false, // Should skip silently
		},
		{
			name: "analysis error",
			job: &queue.Job{
				ID:     uuid.New(),
				Type:   queue.JobTypeTaskAnalysis,
				UserID: userID,
				TodoID: &todoID,
			},
			setupMocks: func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue) {
				aiProvider := &mockAIProvider{
					analyzeTaskWithDueDateFunc: func(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext, tagStats *models.TagStatistics) ([]string, models.TimeHorizon, error) {
						return nil, "", errors.New("analysis failed")
					},
				}
				todoRepo := &mockTodoRepo{
					getByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Todo, error) {
						return &models.Todo{
							ID:          id,
							UserID:      userID,
							Text:        "Test todo",
							Status:      models.TodoStatusPending,
							TimeHorizon: models.TimeHorizonNext,
							Metadata:    models.Metadata{},
						}, nil
					},
					updateFunc: func(ctx context.Context, todo *models.Todo, oldTags []string) error {
						return nil
					},
				}
				return aiProvider, todoRepo, &mockAIContextRepo{}, &mockUserActivityRepo{}, &mockJobQueue{}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create mocks with test context
			aiProvider, todoRepo, contextRepo, activityRepo, jobQueue := tt.setupMocks()
			aiProvider.t = t
			todoRepo.t = t
			contextRepo.t = t
			activityRepo.t = t
			jobQueue.t = t

			analyzer := NewTaskAnalyzer(
				aiProvider,
				todoRepo,
				contextRepo,
				activityRepo,
				nil, // tagStatsRepo - not needed for this test
				jobQueue,
				zap.NewNop(), // test logger
			)

			err := analyzer.ProcessTaskAnalysisJob(context.Background(), tt.job)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				} else {
					// Validate error is meaningful
					if err.Error() == "" {
						t.Error("Expected error message but got empty string")
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Validate todo if validation function provided
				if tt.validateTodo != nil {
					// Get the updated todo from the last update call
					todoRepo.mu.Lock()
					if len(todoRepo.updateCalls) > 0 {
						updatedTodo := todoRepo.updateCalls[len(todoRepo.updateCalls)-1]
						todoRepo.mu.Unlock()
						tt.validateTodo(t, updatedTodo)
					} else {
						todoRepo.mu.Unlock()
						t.Error("Expected Update to be called but it wasn't")
					}
				}
			}
		})
	}
}

func TestTaskAnalyzer_ProcessJob(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	todoID := uuid.New()

	tests := []struct {
		name        string
		job         *queue.Job
		setupMocks  func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue)
		expectError bool
	}{
		{
			name: "task analysis job",
			job: &queue.Job{
				ID:     uuid.New(),
				Type:   queue.JobTypeTaskAnalysis,
				UserID: userID,
				TodoID: &todoID,
			},
			setupMocks: func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue) {
				aiProvider := &mockAIProvider{
					analyzeTaskWithDueDateFunc: func(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext, tagStats *models.TagStatistics) ([]string, models.TimeHorizon, error) {
						return []string{"work"}, models.TimeHorizonNext, nil
					},
				}
				todoRepo := &mockTodoRepo{
					getByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Todo, error) {
						return &models.Todo{
							ID:     id,
							UserID: userID,
							Text:   "Test todo",
							Status: models.TodoStatusPending,
						}, nil
					},
					updateFunc: func(ctx context.Context, todo *models.Todo, oldTags []string) error {
						return nil
					},
				}
				return aiProvider, todoRepo, &mockAIContextRepo{}, &mockUserActivityRepo{}, &mockJobQueue{}
			},
			expectError: false,
		},
		{
			name: "reprocess user job",
			job: &queue.Job{
				ID:     uuid.New(),
				Type:   queue.JobTypeReprocessUser,
				UserID: userID,
			},
			setupMocks: func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue) {
				aiProvider := &mockAIProvider{
					analyzeTaskWithDueDateFunc: func(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext, tagStats *models.TagStatistics) ([]string, models.TimeHorizon, error) {
						return []string{"work"}, models.TimeHorizonNext, nil
					},
				}
				todoRepo := &mockTodoRepo{
					getByUserIDPaginatedFunc: func(ctx context.Context, userID uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error) {
						return []*models.Todo{}, 0, nil
					},
				}
				return aiProvider, todoRepo, &mockAIContextRepo{}, &mockUserActivityRepo{}, &mockJobQueue{}
			},
			expectError: false,
		},
		{
			name: "unknown job type",
			job: &queue.Job{
				ID:     uuid.New(),
				Type:   queue.JobType("unknown"),
				UserID: userID,
			},
			setupMocks: func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue) {
				// No mocks need to be configured - error happens before any calls
				return &mockAIProvider{}, &mockTodoRepo{}, &mockAIContextRepo{}, &mockUserActivityRepo{}, &mockJobQueue{}
			},
			expectError: true,
		},
		{
			name: "job not ready yet",
			job: &queue.Job{
				ID:        uuid.New(),
				Type:      queue.JobTypeTaskAnalysis,
				UserID:    userID,
				TodoID:    &todoID,
				NotBefore: timePtr(time.Now().Add(1 * time.Hour)),
			},
			setupMocks: func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue) {
				// No mocks need to be configured - job is skipped before any calls
				return &mockAIProvider{}, &mockTodoRepo{}, &mockAIContextRepo{}, &mockUserActivityRepo{}, &mockJobQueue{}
			},
			expectError: false, // Should skip silently
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create mocks with test context
			aiProvider, todoRepo, contextRepo, activityRepo, jobQueue := tt.setupMocks()
			aiProvider.t = t
			todoRepo.t = t
			contextRepo.t = t
			activityRepo.t = t
			jobQueue.t = t

			analyzer := NewTaskAnalyzer(
				aiProvider,
				todoRepo,
				contextRepo,
				activityRepo,
				nil, // tagStatsRepo - not needed for this test
				jobQueue,
				zap.NewNop(), // test logger
			)

			msg := &mockMessage{
				job: tt.job,
			}

			err := analyzer.ProcessJob(context.Background(), msg)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				} else {
					// Validate error is meaningful
					if err.Error() == "" {
						t.Error("Expected error message but got empty string")
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func TestTaskAnalyzer_TimeContext(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	todoID := uuid.New()
	fixedTime := time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC)
	timeEnteredStr := "2024-03-15T10:00:00Z" // 4.5 hours before fixedTime
	timeEnteredParsed, _ := time.Parse(time.RFC3339, timeEnteredStr)

	tests := []struct {
		name         string
		todo         *models.Todo
		setupMocks   func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue)
		validateTime func(*testing.T, time.Time) // Validates the createdAt passed to AI
	}{
		{
			name: "uses TimeEntered from metadata when available",
			todo: &models.Todo{
				ID:          todoID,
				UserID:      userID,
				Text:        "Test todo",
				Status:      models.TodoStatusPending,
				TimeHorizon: models.TimeHorizonNext,
				CreatedAt:   fixedTime,
				Metadata: models.Metadata{
					TimeEntered: &timeEnteredStr,
				},
			},
			setupMocks: func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue) {
				aiProvider := &mockAIProvider{
					analyzeTaskWithDueDateFunc: func(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext, tagStats *models.TagStatistics) ([]string, models.TimeHorizon, error) {
						return []string{"work"}, models.TimeHorizonSoon, nil
					},
				}
				todoRepo := &mockTodoRepo{
					getByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Todo, error) {
						return &models.Todo{
							ID:          id,
							UserID:      userID,
							Text:        "Test todo",
							Status:      models.TodoStatusPending,
							TimeHorizon: models.TimeHorizonNext,
							CreatedAt:   fixedTime,
							Metadata: models.Metadata{
								TimeEntered: &timeEnteredStr,
							},
						}, nil
					},
					updateFunc: func(ctx context.Context, todo *models.Todo, oldTags []string) error {
						return nil
					},
				}
				return aiProvider, todoRepo, &mockAIContextRepo{}, &mockUserActivityRepo{}, &mockJobQueue{}
			},
			validateTime: func(t *testing.T, createdAt time.Time) {
				// Should use TimeEntered from metadata, not CreatedAt
				if !createdAt.Equal(timeEnteredParsed) {
					t.Errorf("Expected createdAt to be %s (from TimeEntered), got %s", timeEnteredParsed, createdAt)
				}
			},
		},
		{
			name: "falls back to CreatedAt when TimeEntered is missing",
			todo: &models.Todo{
				ID:          todoID,
				UserID:      userID,
				Text:        "Test todo",
				Status:      models.TodoStatusPending,
				TimeHorizon: models.TimeHorizonNext,
				CreatedAt:   fixedTime,
				Metadata:    models.Metadata{}, // No TimeEntered
			},
			setupMocks: func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue) {
				aiProvider := &mockAIProvider{
					analyzeTaskWithDueDateFunc: func(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext, tagStats *models.TagStatistics) ([]string, models.TimeHorizon, error) {
						return []string{"work"}, models.TimeHorizonSoon, nil
					},
				}
				todoRepo := &mockTodoRepo{
					getByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Todo, error) {
						return &models.Todo{
							ID:          id,
							UserID:      userID,
							Text:        "Test todo",
							Status:      models.TodoStatusPending,
							TimeHorizon: models.TimeHorizonNext,
							CreatedAt:   fixedTime,
							Metadata:    models.Metadata{},
						}, nil
					},
					updateFunc: func(ctx context.Context, todo *models.Todo, oldTags []string) error {
						return nil
					},
				}
				return aiProvider, todoRepo, &mockAIContextRepo{}, &mockUserActivityRepo{}, &mockJobQueue{}
			},
			validateTime: func(t *testing.T, createdAt time.Time) {
				// Should fall back to CreatedAt
				if !createdAt.Equal(fixedTime) {
					t.Errorf("Expected createdAt to be %s (from CreatedAt), got %s", fixedTime, createdAt)
				}
			},
		},
		{
			name: "falls back to CreatedAt when TimeEntered is invalid",
			todo: &models.Todo{
				ID:          todoID,
				UserID:      userID,
				Text:        "Test todo",
				Status:      models.TodoStatusPending,
				TimeHorizon: models.TimeHorizonNext,
				CreatedAt:   fixedTime,
				Metadata: models.Metadata{
					TimeEntered: stringPtr("invalid-date"), // Invalid format
				},
			},
			setupMocks: func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue) {
				aiProvider := &mockAIProvider{
					analyzeTaskWithDueDateFunc: func(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext, tagStats *models.TagStatistics) ([]string, models.TimeHorizon, error) {
						return []string{"work"}, models.TimeHorizonSoon, nil
					},
				}
				todoRepo := &mockTodoRepo{
					getByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Todo, error) {
						return &models.Todo{
							ID:          id,
							UserID:      userID,
							Text:        "Test todo",
							Status:      models.TodoStatusPending,
							TimeHorizon: models.TimeHorizonNext,
							CreatedAt:   fixedTime,
							Metadata: models.Metadata{
								TimeEntered: stringPtr("invalid-date"),
							},
						}, nil
					},
					updateFunc: func(ctx context.Context, todo *models.Todo, oldTags []string) error {
						return nil
					},
				}
				return aiProvider, todoRepo, &mockAIContextRepo{}, &mockUserActivityRepo{}, &mockJobQueue{}
			},
			validateTime: func(t *testing.T, createdAt time.Time) {
				// Should fall back to CreatedAt when parsing fails
				if !createdAt.Equal(fixedTime) {
					t.Errorf("Expected createdAt to be %s (fallback to CreatedAt), got %s", fixedTime, createdAt)
				}
			},
		},
		{
			name: "passes creation time even without due date",
			todo: &models.Todo{
				ID:          todoID,
				UserID:      userID,
				Text:        "Test todo without due date",
				Status:      models.TodoStatusPending,
				TimeHorizon: models.TimeHorizonNext,
				CreatedAt:   fixedTime,
				DueDate:     nil,
				Metadata: models.Metadata{
					TimeEntered: &timeEnteredStr,
				},
			},
			setupMocks: func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue) {
				aiProvider := &mockAIProvider{
					analyzeTaskWithDueDateFunc: func(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext, tagStats *models.TagStatistics) ([]string, models.TimeHorizon, error) {
						return []string{"work"}, models.TimeHorizonSoon, nil
					},
				}
				todoRepo := &mockTodoRepo{
					getByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Todo, error) {
						return &models.Todo{
							ID:          id,
							UserID:      userID,
							Text:        "Test todo without due date",
							Status:      models.TodoStatusPending,
							TimeHorizon: models.TimeHorizonNext,
							CreatedAt:   fixedTime,
							DueDate:     nil,
							Metadata: models.Metadata{
								TimeEntered: &timeEnteredStr,
							},
						}, nil
					},
					updateFunc: func(ctx context.Context, todo *models.Todo, oldTags []string) error {
						return nil
					},
				}
				return aiProvider, todoRepo, &mockAIContextRepo{}, &mockUserActivityRepo{}, &mockJobQueue{}
			},
			validateTime: func(t *testing.T, createdAt time.Time) {
				// Should still pass creation time even without due date
				if !createdAt.Equal(timeEnteredParsed) {
					t.Errorf("Expected createdAt to be %s, got %s", timeEnteredParsed, createdAt)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedCreatedAt time.Time

			// Create mocks with test context
			aiProvider, todoRepo, contextRepo, activityRepo, jobQueue := tt.setupMocks()
			aiProvider.t = t
			todoRepo.t = t
			contextRepo.t = t
			activityRepo.t = t
			jobQueue.t = t

			// Wrap the analyzeTaskWithDueDateFunc to capture createdAt
			originalFunc := aiProvider.analyzeTaskWithDueDateFunc
			aiProvider.analyzeTaskWithDueDateFunc = func(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext, tagStats *models.TagStatistics) ([]string, models.TimeHorizon, error) {
				capturedCreatedAt = createdAt
				if originalFunc != nil {
					return originalFunc(ctx, text, dueDate, createdAt, userContext, tagStats)
				}
				return []string{"work"}, models.TimeHorizonSoon, nil
			}

			analyzer := NewTaskAnalyzer(
				aiProvider,
				todoRepo,
				contextRepo,
				activityRepo,
				nil, // tagStatsRepo - not needed for this test
				jobQueue,
				zap.NewNop(), // test logger
			)

			job := &queue.Job{
				ID:     uuid.New(),
				Type:   queue.JobTypeTaskAnalysis,
				UserID: userID,
				TodoID: &todoID,
			}

			err := analyzer.ProcessTaskAnalysisJob(context.Background(), job)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.validateTime != nil {
				tt.validateTime(t, capturedCreatedAt)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
