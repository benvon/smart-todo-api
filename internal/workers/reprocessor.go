package workers

import (
	"context"
	"fmt"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/queue"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Reprocessor handles scheduling reprocessing jobs
type Reprocessor struct {
	jobQueue     queue.JobQueue
	activityRepo database.UserActivityRepositoryInterface
	logger       *zap.Logger
}

// NewReprocessor creates a new reprocessor
func NewReprocessor(jobQueue queue.JobQueue, activityRepo database.UserActivityRepositoryInterface, logger *zap.Logger) *Reprocessor {
	return &Reprocessor{
		jobQueue:     jobQueue,
		activityRepo: activityRepo,
		logger:       logger,
	}
}

// ScheduleReprocessingJobs creates reprocessing jobs for eligible users (2x/day)
func (r *Reprocessor) ScheduleReprocessingJobs(ctx context.Context) error {
	// Get all active users (not paused)
	// In a real implementation, we'd query the database for active users
	// For now, we'll need to add a method to get active users

	// Calculate next scheduled times (e.g., 08:00 and 20:00)
	now := time.Now()
	nextMorning := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, now.Location())
	nextEvening := time.Date(now.Year(), now.Month(), now.Day(), 20, 0, 0, 0, now.Location())

	// If we're past morning time today, schedule for tomorrow
	if now.After(nextMorning) {
		nextMorning = nextMorning.Add(24 * time.Hour)
	}

	// If we're past evening time today, schedule for tomorrow
	if now.After(nextEvening) {
		nextEvening = nextEvening.Add(24 * time.Hour)
	}

	// Schedule reprocessing jobs for eligible users
	eligibleUsers, err := r.GetEligibleUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get eligible users: %w", err)
	}

	// Create reprocessing jobs for each eligible user at both scheduled times
	for _, userID := range eligibleUsers {
		// Schedule morning job
		if err := r.createReprocessingJob(ctx, userID, nextMorning); err != nil {
			r.logger.Warn("failed_to_schedule_morning_reprocessing_job",
				zap.String("user_id", userID.String()),
				zap.Error(err),
			)
			// Continue with other users
		}

		// Schedule evening job
		if err := r.createReprocessingJob(ctx, userID, nextEvening); err != nil {
			r.logger.Warn("failed_to_schedule_evening_reprocessing_job",
				zap.String("user_id", userID.String()),
				zap.Error(err),
			)
			// Continue with other users
		}
	}

	r.logger.Info("scheduled_reprocessing_jobs",
		zap.Int("user_count", len(eligibleUsers)),
		zap.Time("next_morning", nextMorning),
		zap.Time("next_evening", nextEvening),
	)

	return nil
}

// createReprocessingJob creates a reprocessing job for a user
func (r *Reprocessor) createReprocessingJob(ctx context.Context, userID uuid.UUID, notBefore time.Time) error {
	job := queue.NewJob(queue.JobTypeReprocessUser, userID, nil)
	job.NotBefore = &notBefore

	// Set NotAfter to 1 day after scheduled time for garbage collection
	notAfter := notBefore.Add(24 * time.Hour)
	job.NotAfter = &notAfter

	if err := r.jobQueue.Enqueue(ctx, job); err != nil {
		return fmt.Errorf("failed to enqueue reprocessing job: %w", err)
	}

	return nil
}

// GetEligibleUsers returns users who are eligible for reprocessing
// (not paused, within activity window)
func (r *Reprocessor) GetEligibleUsers(ctx context.Context) ([]uuid.UUID, error) {
	return r.activityRepo.GetEligibleUsersForReprocessing(ctx)
}
