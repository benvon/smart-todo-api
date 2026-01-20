package workers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/queue"
	"github.com/google/uuid"
)

// mockJobQueueForReprocessor is a mock implementation of JobQueue for reprocessor tests
type mockJobQueueForReprocessor struct {
	enqueueFunc func(ctx context.Context, job *queue.Job) error
}

func (m *mockJobQueueForReprocessor) Enqueue(ctx context.Context, job *queue.Job) error {
	if m.enqueueFunc != nil {
		return m.enqueueFunc(ctx, job)
	}
	return nil
}

func (m *mockJobQueueForReprocessor) Dequeue(ctx context.Context) (*queue.Message, error) {
	return nil, errors.New("not implemented")
}

func (m *mockJobQueueForReprocessor) Consume(ctx context.Context, prefetchCount int) (<-chan *queue.Message, <-chan error, error) {
	return nil, nil, errors.New("not implemented")
}

func (m *mockJobQueueForReprocessor) Close() error {
	return nil
}

// Ensure mock implements interface
var _ queue.JobQueue = (*mockJobQueueForReprocessor)(nil)

// mockUserActivityRepoForReprocessor is a mock implementation of UserActivityRepositoryInterface for reprocessor tests
type mockUserActivityRepoForReprocessor struct {
	getByUserIDFunc                     func(ctx context.Context, userID uuid.UUID) (*models.UserActivity, error)
	getEligibleUsersForReprocessingFunc func(ctx context.Context) ([]uuid.UUID, error)
}

func (m *mockUserActivityRepoForReprocessor) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.UserActivity, error) {
	if m.getByUserIDFunc != nil {
		return m.getByUserIDFunc(ctx, userID)
	}
	return &models.UserActivity{
		UserID:             userID,
		ReprocessingPaused: false,
	}, nil
}

func (m *mockUserActivityRepoForReprocessor) GetEligibleUsersForReprocessing(ctx context.Context) ([]uuid.UUID, error) {
	if m.getEligibleUsersForReprocessingFunc != nil {
		return m.getEligibleUsersForReprocessingFunc(ctx)
	}
	return []uuid.UUID{}, nil
}

// Ensure mock implements interface
var _ database.UserActivityRepositoryInterface = (*mockUserActivityRepoForReprocessor)(nil)

func TestReprocessor_ScheduleReprocessingJobs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupMocks  func() (*mockJobQueueForReprocessor, *mockUserActivityRepoForReprocessor)
		expectError bool
		validate    func(*testing.T, []*queue.Job)
	}{
		{
			name: "successful scheduling",
			setupMocks: func() (*mockJobQueueForReprocessor, *mockUserActivityRepoForReprocessor) {
				userID1 := uuid.New()
				userID2 := uuid.New()

				enqueuedJobs := []*queue.Job{}

				jobQueue := &mockJobQueueForReprocessor{
					enqueueFunc: func(ctx context.Context, job *queue.Job) error {
						enqueuedJobs = append(enqueuedJobs, job)
						return nil
					},
				}

				activityRepo := &mockUserActivityRepoForReprocessor{
					getEligibleUsersForReprocessingFunc: func(ctx context.Context) ([]uuid.UUID, error) {
						return []uuid.UUID{userID1, userID2}, nil
					},
				}

				return jobQueue, activityRepo
			},
			expectError: false,
		},
		{
			name: "no eligible users",
			setupMocks: func() (*mockJobQueueForReprocessor, *mockUserActivityRepoForReprocessor) {
				jobQueue := &mockJobQueueForReprocessor{}
				activityRepo := &mockUserActivityRepoForReprocessor{
					getEligibleUsersForReprocessingFunc: func(ctx context.Context) ([]uuid.UUID, error) {
						return []uuid.UUID{}, nil
					},
				}
				return jobQueue, activityRepo
			},
			expectError: false,
		},
		{
			name: "error getting eligible users",
			setupMocks: func() (*mockJobQueueForReprocessor, *mockUserActivityRepoForReprocessor) {
				jobQueue := &mockJobQueueForReprocessor{}
				activityRepo := &mockUserActivityRepoForReprocessor{
					getEligibleUsersForReprocessingFunc: func(ctx context.Context) ([]uuid.UUID, error) {
						return nil, errors.New("database error")
					},
				}
				return jobQueue, activityRepo
			},
			expectError: true,
		},
		{
			name: "error enqueueing job",
			setupMocks: func() (*mockJobQueueForReprocessor, *mockUserActivityRepoForReprocessor) {
				userID := uuid.New()

				jobQueue := &mockJobQueueForReprocessor{
					enqueueFunc: func(ctx context.Context, job *queue.Job) error {
						return errors.New("queue error")
					},
				}

				activityRepo := &mockUserActivityRepoForReprocessor{
					getEligibleUsersForReprocessingFunc: func(ctx context.Context) ([]uuid.UUID, error) {
						return []uuid.UUID{userID}, nil
					},
				}

				return jobQueue, activityRepo
			},
			expectError: false, // Errors are logged but don't fail the entire operation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			jobQueue, activityRepo := tt.setupMocks()

			reprocessor := NewReprocessor(jobQueue, activityRepo)

			err := reprocessor.ScheduleReprocessingJobs(context.Background())

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

func TestReprocessor_GetEligibleUsers(t *testing.T) {
	t.Parallel()

	userID1 := uuid.New()
	userID2 := uuid.New()

	tests := []struct {
		name        string
		setupMocks  func() *mockUserActivityRepoForReprocessor
		want        []uuid.UUID
		expectError bool
	}{
		{
			name: "successful retrieval",
			setupMocks: func() *mockUserActivityRepoForReprocessor {
				return &mockUserActivityRepoForReprocessor{
					getEligibleUsersForReprocessingFunc: func(ctx context.Context) ([]uuid.UUID, error) {
						return []uuid.UUID{userID1, userID2}, nil
					},
				}
			},
			want:        []uuid.UUID{userID1, userID2},
			expectError: false,
		},
		{
			name: "empty result",
			setupMocks: func() *mockUserActivityRepoForReprocessor {
				return &mockUserActivityRepoForReprocessor{
					getEligibleUsersForReprocessingFunc: func(ctx context.Context) ([]uuid.UUID, error) {
						return []uuid.UUID{}, nil
					},
				}
			},
			want:        []uuid.UUID{},
			expectError: false,
		},
		{
			name: "error from repository",
			setupMocks: func() *mockUserActivityRepoForReprocessor {
				return &mockUserActivityRepoForReprocessor{
					getEligibleUsersForReprocessingFunc: func(ctx context.Context) ([]uuid.UUID, error) {
						return nil, errors.New("database error")
					},
				}
			},
			want:        nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			activityRepo := tt.setupMocks()
			reprocessor := NewReprocessor(&mockJobQueueForReprocessor{}, activityRepo)

			got, err := reprocessor.GetEligibleUsers(context.Background())

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(got) != len(tt.want) {
					t.Errorf("Expected %d users, got %d", len(tt.want), len(got))
				}
				for i, id := range tt.want {
					if i < len(got) && got[i] != id {
						t.Errorf("Expected user ID %s at index %d, got %s", id, i, got[i])
					}
				}
			}
		})
	}
}

func TestReprocessor_createReprocessingJob(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	now := time.Now()
	notBefore := now.Add(2 * time.Hour)

	tests := []struct {
		name        string
		userID      uuid.UUID
		notBefore   time.Time
		setupMocks  func() *mockJobQueueForReprocessor
		expectError bool
		validateJob func(*testing.T, *queue.Job)
	}{
		{
			name:      "successful job creation",
			userID:    userID,
			notBefore: notBefore,
			setupMocks: func() *mockJobQueueForReprocessor {
				return &mockJobQueueForReprocessor{
					enqueueFunc: func(ctx context.Context, job *queue.Job) error {
						return nil
					},
				}
			},
			expectError: false,
			validateJob: func(t *testing.T, job *queue.Job) {
				if job.Type != queue.JobTypeReprocessUser {
					t.Errorf("Expected job type to be %s, got %s", queue.JobTypeReprocessUser, job.Type)
				}
				if job.UserID != userID {
					t.Errorf("Expected user ID to be %s, got %s", userID, job.UserID)
				}
				if job.NotBefore == nil {
					t.Error("Expected NotBefore to be set")
				}
				if job.NotAfter == nil {
					t.Error("Expected NotAfter to be set")
				}
			},
		},
		{
			name:      "error enqueueing",
			userID:    userID,
			notBefore: notBefore,
			setupMocks: func() *mockJobQueueForReprocessor {
				return &mockJobQueueForReprocessor{
					enqueueFunc: func(ctx context.Context, job *queue.Job) error {
						return errors.New("queue error")
					},
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			jobQueue := tt.setupMocks()
			reprocessor := NewReprocessor(jobQueue, &mockUserActivityRepoForReprocessor{})

			err := reprocessor.createReprocessingJob(context.Background(), tt.userID, tt.notBefore)

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
