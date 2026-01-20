package queue

import (
	"time"

	"github.com/google/uuid"
)

// JobType represents the type of job
type JobType string

const (
	// JobTypeTaskAnalysis is a job for analyzing a single task
	JobTypeTaskAnalysis JobType = "task_analysis"
	// JobTypeReprocessUser is a job for reprocessing all todos for a user
	JobTypeReprocessUser JobType = "reprocess_user"
)

// Job represents a job in the queue
type Job struct {
	ID         uuid.UUID              `json:"id"`
	Type       JobType                `json:"type"`
	UserID     uuid.UUID              `json:"user_id"`
	TodoID     *uuid.UUID             `json:"todo_id,omitempty"` // Optional, for task analysis jobs
	NotBefore  *time.Time             `json:"not_before,omitempty"` // Earliest time to process job (nil = immediate)
	NotAfter   *time.Time             `json:"not_after,omitempty"`  // Latest time to process job (nil = no expiration)
	Metadata   map[string]any         `json:"metadata,omitempty"`   // Job-specific data
	CreatedAt  time.Time              `json:"created_at"`
	RetryCount int                    `json:"retry_count"`
	MaxRetries int                    `json:"max_retries"`
}

// NewJob creates a new job
func NewJob(jobType JobType, userID uuid.UUID, todoID *uuid.UUID) *Job {
	return &Job{
		ID:         uuid.New(),
		Type:       jobType,
		UserID:     userID,
		TodoID:     todoID,
		Metadata:   make(map[string]any),
		CreatedAt:  time.Now(),
		RetryCount: 0,
		MaxRetries: 3,
	}
}

// ShouldProcess checks if the job should be processed now
func (j *Job) ShouldProcess() bool {
	now := time.Now()
	
	// Check NotBefore
	if j.NotBefore != nil && now.Before(*j.NotBefore) {
		return false
	}
	
	// Check NotAfter
	if j.NotAfter != nil && now.After(*j.NotAfter) {
		return false
	}
	
	return true
}

// IsExpired checks if the job has expired
func (j *Job) IsExpired() bool {
	if j.NotAfter == nil {
		return false
	}
	
	return time.Now().After(*j.NotAfter)
}

// CanRetry checks if the job can be retried
func (j *Job) CanRetry() bool {
	return j.RetryCount < j.MaxRetries
}

// IncrementRetry increments the retry count
func (j *Job) IncrementRetry() {
	j.RetryCount++
}
