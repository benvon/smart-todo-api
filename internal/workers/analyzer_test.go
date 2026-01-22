package workers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/queue"
	"github.com/benvon/smart-todo/internal/services/ai"
	"github.com/google/uuid"
)

// mockAIProvider is a mock implementation of AIProvider
type mockAIProvider struct {
	analyzeTaskFunc              func(ctx context.Context, text string, userContext *models.AIContext) ([]string, models.TimeHorizon, error)
	analyzeTaskWithDueDateFunc   func(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext) ([]string, models.TimeHorizon, error)
}

func (m *mockAIProvider) AnalyzeTask(ctx context.Context, text string, userContext *models.AIContext) ([]string, models.TimeHorizon, error) {
	if m.analyzeTaskFunc != nil {
		return m.analyzeTaskFunc(ctx, text, userContext)
	}
	return []string{"work"}, models.TimeHorizonNext, nil
}

func (m *mockAIProvider) AnalyzeTaskWithDueDate(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext) ([]string, models.TimeHorizon, error) {
	if m.analyzeTaskWithDueDateFunc != nil {
		return m.analyzeTaskWithDueDateFunc(ctx, text, dueDate, createdAt, userContext)
	}
	// Default implementation
	if m.analyzeTaskFunc != nil {
		return m.analyzeTaskFunc(ctx, text, userContext)
	}
	return []string{"work"}, models.TimeHorizonNext, nil
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
	getByIDFunc              func(ctx context.Context, id uuid.UUID) (*models.Todo, error)
	updateFunc               func(ctx context.Context, todo *models.Todo) error
	getByUserIDPaginatedFunc func(ctx context.Context, userID uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error)
}

func (m *mockTodoRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Todo, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return &models.Todo{
		ID:          id,
		UserID:      uuid.New(),
		Text:        "Test todo",
		Status:      models.TodoStatusPending,
		TimeHorizon: models.TimeHorizonNext,
		Metadata:    models.Metadata{},
	}, nil
}

func (m *mockTodoRepo) Update(ctx context.Context, todo *models.Todo) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, todo)
	}
	return nil
}

func (m *mockTodoRepo) GetByUserIDPaginated(ctx context.Context, userID uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error) {
	if m.getByUserIDPaginatedFunc != nil {
		return m.getByUserIDPaginatedFunc(ctx, userID, timeHorizon, status, page, pageSize)
	}
	return []*models.Todo{}, 0, nil
}

// Ensure mock implements interface
var _ database.TodoRepositoryInterface = (*mockTodoRepo)(nil)

// mockAIContextRepo is a mock implementation of AIContextRepositoryInterface
type mockAIContextRepo struct {
	getByUserIDFunc func(ctx context.Context, userID uuid.UUID) (*models.AIContext, error)
}

func (m *mockAIContextRepo) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.AIContext, error) {
	if m.getByUserIDFunc != nil {
		return m.getByUserIDFunc(ctx, userID)
	}
	return nil, errors.New("not found")
}

// Ensure mock implements interface
var _ database.AIContextRepositoryInterface = (*mockAIContextRepo)(nil)

// mockUserActivityRepo is a mock implementation of UserActivityRepositoryInterface
type mockUserActivityRepo struct {
	getByUserIDFunc func(ctx context.Context, userID uuid.UUID) (*models.UserActivity, error)
}

func (m *mockUserActivityRepo) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.UserActivity, error) {
	if m.getByUserIDFunc != nil {
		return m.getByUserIDFunc(ctx, userID)
	}
	return &models.UserActivity{
		UserID:             userID,
		ReprocessingPaused: false,
	}, nil
}

func (m *mockUserActivityRepo) GetEligibleUsersForReprocessing(ctx context.Context) ([]uuid.UUID, error) {
	return []uuid.UUID{}, nil
}

// Ensure mock implements interface
var _ database.UserActivityRepositoryInterface = (*mockUserActivityRepo)(nil)

// mockJobQueue is a mock implementation of JobQueue
type mockJobQueue struct {
	enqueueFunc func(ctx context.Context, job *queue.Job) error
}

func (m *mockJobQueue) Enqueue(ctx context.Context, job *queue.Job) error {
	if m.enqueueFunc != nil {
		return m.enqueueFunc(ctx, job)
	}
	return nil
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
					analyzeTaskFunc: func(ctx context.Context, text string, userContext *models.AIContext) ([]string, models.TimeHorizon, error) {
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
					updateFunc: func(ctx context.Context, todo *models.Todo) error {
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
			name: "missing todo_id",
			job: &queue.Job{
				ID:     uuid.New(),
				Type:   queue.JobTypeTaskAnalysis,
				UserID: userID,
				TodoID: nil,
			},
			setupMocks: func() (*mockAIProvider, *mockTodoRepo, *mockAIContextRepo, *mockUserActivityRepo, *mockJobQueue) {
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
					analyzeTaskFunc: func(ctx context.Context, text string, userContext *models.AIContext) ([]string, models.TimeHorizon, error) {
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
					updateFunc: func(ctx context.Context, todo *models.Todo) error {
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

			aiProvider, todoRepo, contextRepo, activityRepo, jobQueue := tt.setupMocks()

			analyzer := NewTaskAnalyzer(
				aiProvider,
				todoRepo,
				contextRepo,
				activityRepo,
				jobQueue,
			)

			err := analyzer.ProcessTaskAnalysisJob(context.Background(), tt.job)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
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
					analyzeTaskFunc: func(ctx context.Context, text string, userContext *models.AIContext) ([]string, models.TimeHorizon, error) {
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
					analyzeTaskFunc: func(ctx context.Context, text string, userContext *models.AIContext) ([]string, models.TimeHorizon, error) {
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
				return &mockAIProvider{}, &mockTodoRepo{}, &mockAIContextRepo{}, &mockUserActivityRepo{}, &mockJobQueue{}
			},
			expectError: false, // Should skip silently
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			aiProvider, todoRepo, contextRepo, activityRepo, jobQueue := tt.setupMocks()

			analyzer := NewTaskAnalyzer(
				aiProvider,
				todoRepo,
				contextRepo,
				activityRepo,
				jobQueue,
			)

			msg := &mockMessage{
				job: tt.job,
			}

			err := analyzer.ProcessJob(context.Background(), msg)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
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
					analyzeTaskWithDueDateFunc: func(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext) ([]string, models.TimeHorizon, error) {
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
					updateFunc: func(ctx context.Context, todo *models.Todo) error {
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
					analyzeTaskWithDueDateFunc: func(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext) ([]string, models.TimeHorizon, error) {
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
					updateFunc: func(ctx context.Context, todo *models.Todo) error {
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
					analyzeTaskWithDueDateFunc: func(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext) ([]string, models.TimeHorizon, error) {
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
					updateFunc: func(ctx context.Context, todo *models.Todo) error {
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
					analyzeTaskWithDueDateFunc: func(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext) ([]string, models.TimeHorizon, error) {
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
					updateFunc: func(ctx context.Context, todo *models.Todo) error {
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
			
			// Wrap setupMocks to capture createdAt
			aiProvider, todoRepo, contextRepo, activityRepo, jobQueue := tt.setupMocks()
			
			// Wrap the analyzeTaskWithDueDateFunc to capture createdAt
			originalFunc := aiProvider.analyzeTaskWithDueDateFunc
			aiProvider.analyzeTaskWithDueDateFunc = func(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext) ([]string, models.TimeHorizon, error) {
				capturedCreatedAt = createdAt
				if originalFunc != nil {
					return originalFunc(ctx, text, dueDate, createdAt, userContext)
				}
				return []string{"work"}, models.TimeHorizonSoon, nil
			}

			analyzer := NewTaskAnalyzer(
				aiProvider,
				todoRepo,
				contextRepo,
				activityRepo,
				jobQueue,
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
