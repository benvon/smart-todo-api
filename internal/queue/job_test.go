package queue

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewJob(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	todoID := uuid.New()

	job := NewJob(JobTypeTaskAnalysis, userID, &todoID)

	if job.ID == uuid.Nil {
		t.Error("Expected job ID to be set")
	}
	if job.Type != JobTypeTaskAnalysis {
		t.Errorf("Expected job type to be %s, got %s", JobTypeTaskAnalysis, job.Type)
	}
	if job.UserID != userID {
		t.Errorf("Expected user ID to be %s, got %s", userID, job.UserID)
	}
	if job.TodoID == nil || *job.TodoID != todoID {
		t.Errorf("Expected todo ID to be %s, got %v", todoID, job.TodoID)
	}
	if job.Metadata == nil {
		t.Error("Expected metadata to be initialized")
	}
	if job.RetryCount != 0 {
		t.Errorf("Expected retry count to be 0, got %d", job.RetryCount)
	}
	if job.MaxRetries != 3 {
		t.Errorf("Expected max retries to be 3, got %d", job.MaxRetries)
	}
}

func TestJob_ShouldProcess(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	now := time.Now()

	tests := []struct {
		name      string
		job       *Job
		want      bool
		setupTime func() time.Time
	}{
		{
			name: "no time constraints",
			job: &Job{
				ID:         uuid.New(),
				Type:       JobTypeTaskAnalysis,
				UserID:     userID,
				NotBefore:  nil,
				NotAfter:   nil,
			},
			want: true,
		},
		{
			name: "not before in past",
			job: &Job{
				ID:         uuid.New(),
				Type:       JobTypeTaskAnalysis,
				UserID:     userID,
				NotBefore:  timePtr(now.Add(-1 * time.Hour)),
				NotAfter:   nil,
			},
			want: true,
		},
		{
			name: "not before in future",
			job: &Job{
				ID:         uuid.New(),
				Type:       JobTypeTaskAnalysis,
				UserID:     userID,
				NotBefore:  timePtr(now.Add(1 * time.Hour)),
				NotAfter:   nil,
			},
			want: false,
		},
		{
			name: "not after in past",
			job: &Job{
				ID:         uuid.New(),
				Type:       JobTypeTaskAnalysis,
				UserID:     userID,
				NotBefore:  nil,
				NotAfter:   timePtr(now.Add(-1 * time.Hour)),
			},
			want: false,
		},
		{
			name: "not after in future",
			job: &Job{
				ID:         uuid.New(),
				Type:       JobTypeTaskAnalysis,
				UserID:     userID,
				NotBefore:  nil,
				NotAfter:   timePtr(now.Add(1 * time.Hour)),
			},
			want: true,
		},
		{
			name: "within time window",
			job: &Job{
				ID:         uuid.New(),
				Type:       JobTypeTaskAnalysis,
				UserID:     userID,
				NotBefore:  timePtr(now.Add(-1 * time.Hour)),
				NotAfter:   timePtr(now.Add(1 * time.Hour)),
			},
			want: true,
		},
		{
			name: "outside time window - before",
			job: &Job{
				ID:         uuid.New(),
				Type:       JobTypeTaskAnalysis,
				UserID:     userID,
				NotBefore:  timePtr(now.Add(1 * time.Hour)),
				NotAfter:   timePtr(now.Add(2 * time.Hour)),
			},
			want: false,
		},
		{
			name: "outside time window - after",
			job: &Job{
				ID:         uuid.New(),
				Type:       JobTypeTaskAnalysis,
				UserID:     userID,
				NotBefore:  timePtr(now.Add(-2 * time.Hour)),
				NotAfter:   timePtr(now.Add(-1 * time.Hour)),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.job.ShouldProcess()
			if got != tt.want {
				t.Errorf("ShouldProcess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJob_IsExpired(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	now := time.Now()

	tests := []struct {
		name string
		job  *Job
		want bool
	}{
		{
			name: "no expiration",
			job: &Job{
				ID:        uuid.New(),
				Type:      JobTypeTaskAnalysis,
				UserID:    userID,
				NotAfter:  nil,
			},
			want: false,
		},
		{
			name: "expired",
			job: &Job{
				ID:        uuid.New(),
				Type:      JobTypeTaskAnalysis,
				UserID:    userID,
				NotAfter:  timePtr(now.Add(-1 * time.Hour)),
			},
			want: true,
		},
		{
			name: "not expired",
			job: &Job{
				ID:        uuid.New(),
				Type:      JobTypeTaskAnalysis,
				UserID:    userID,
				NotAfter:  timePtr(now.Add(1 * time.Hour)),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.job.IsExpired()
			if got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJob_CanRetry(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	tests := []struct {
		name      string
		retryCount int
		maxRetries int
		want      bool
	}{
		{
			name:       "can retry - no retries yet",
			retryCount: 0,
			maxRetries: 3,
			want:       true,
		},
		{
			name:       "can retry - one retry",
			retryCount: 1,
			maxRetries: 3,
			want:       true,
		},
		{
			name:       "can retry - max retries minus one",
			retryCount: 2,
			maxRetries: 3,
			want:       true,
		},
		{
			name:       "cannot retry - at max retries",
			retryCount: 3,
			maxRetries: 3,
			want:       false,
		},
		{
			name:       "cannot retry - exceeded max retries",
			retryCount: 4,
			maxRetries: 3,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			job := &Job{
				ID:         uuid.New(),
				Type:       JobTypeTaskAnalysis,
				UserID:     userID,
				RetryCount: tt.retryCount,
				MaxRetries: tt.maxRetries,
			}
			got := job.CanRetry()
			if got != tt.want {
				t.Errorf("CanRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJob_IncrementRetry(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	job := &Job{
		ID:         uuid.New(),
		Type:       JobTypeTaskAnalysis,
		UserID:     userID,
		RetryCount: 0,
		MaxRetries: 3,
	}

	job.IncrementRetry()
	if job.RetryCount != 1 {
		t.Errorf("Expected retry count to be 1 after increment, got %d", job.RetryCount)
	}

	job.IncrementRetry()
	if job.RetryCount != 2 {
		t.Errorf("Expected retry count to be 2 after second increment, got %d", job.RetryCount)
	}

	job.IncrementRetry()
	if job.RetryCount != 3 {
		t.Errorf("Expected retry count to be 3 after third increment, got %d", job.RetryCount)
	}
}

// Helper function to create time pointers
func timePtr(t time.Time) *time.Time {
	return &t
}
