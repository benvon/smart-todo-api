package workers

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/queue"
	"github.com/benvon/smart-todo/internal/services/ai"
)

// TaskAnalyzer processes task analysis jobs
type TaskAnalyzer struct {
	aiProvider   ai.AIProvider
	todoRepo     *database.TodoRepository
	contextRepo  *database.AIContextRepository
	activityRepo *database.UserActivityRepository
	jobQueue     queue.JobQueue // For re-enqueueing jobs with delays
}

// NewTaskAnalyzer creates a new task analyzer
func NewTaskAnalyzer(
	aiProvider ai.AIProvider,
	todoRepo *database.TodoRepository,
	contextRepo *database.AIContextRepository,
	activityRepo *database.UserActivityRepository,
	jobQueue queue.JobQueue,
) *TaskAnalyzer {
	return &TaskAnalyzer{
		aiProvider:   aiProvider,
		todoRepo:     todoRepo,
		contextRepo:  contextRepo,
		activityRepo: activityRepo,
		jobQueue:     jobQueue,
	}
}

// ProcessTaskAnalysisJob processes a task analysis job
func (a *TaskAnalyzer) ProcessTaskAnalysisJob(ctx context.Context, job *queue.Job) error {
	if job.TodoID == nil {
		return fmt.Errorf("todo_id is required for task analysis job")
	}

	// Load todo
	todo, err := a.todoRepo.GetByID(ctx, *job.TodoID)
	if err != nil {
		return fmt.Errorf("failed to get todo: %w", err)
	}

	// Verify todo belongs to user
	if todo.UserID != job.UserID {
		return fmt.Errorf("todo does not belong to user")
	}

	// Load user context
	var userContext *models.AIContext
	context, err := a.contextRepo.GetByUserID(ctx, job.UserID)
	if err == nil {
		userContext = context
	}

	// Check if user has reprocessing paused
	activity, err := a.activityRepo.GetByUserID(ctx, job.UserID)
	if err == nil && activity != nil && activity.ReprocessingPaused {
		log.Printf("Skipping analysis for user %s (reprocessing paused)", job.UserID)
		return nil
	}

	// Set status to processing before starting analysis
	// Only update if currently pending (don't override completed status)
	if todo.Status == models.TodoStatusPending {
		todo.Status = models.TodoStatusProcessing
		if err := a.todoRepo.Update(ctx, todo); err != nil {
			log.Printf("Failed to update todo status to processing: %v", err)
			// Continue with analysis even if status update fails
		} else {
			log.Printf("Set todo %s status to processing", todo.ID)
		}
	}

	// Analyze task (with due date if available)
	var tags []string
	var timeHorizon models.TimeHorizon
	// Check if provider supports due date analysis
	if todo.DueDate != nil {
		if providerWithDueDate, ok := a.aiProvider.(ai.AIProviderWithDueDate); ok {
			tags, timeHorizon, err = providerWithDueDate.AnalyzeTaskWithDueDate(ctx, todo.Text, todo.DueDate, userContext)
		} else {
			tags, timeHorizon, err = a.aiProvider.AnalyzeTask(ctx, todo.Text, userContext)
		}
	} else {
		tags, timeHorizon, err = a.aiProvider.AnalyzeTask(ctx, todo.Text, userContext)
	}
	if err != nil {
		// On error, set status back to pending so it can be retried
		if todo.Status == models.TodoStatusProcessing {
			todo.Status = models.TodoStatusPending
			if updateErr := a.todoRepo.Update(ctx, todo); updateErr != nil {
				log.Printf("Failed to reset todo status to pending after error: %v", updateErr)
			}
		}
		return fmt.Errorf("failed to analyze task: %w", err)
	}

	// Get existing user-defined tags (preserve them)
	existingUserTags := todo.Metadata.GetUserTags()

	// Merge AI tags with user tags (user tags override)
	todo.Metadata.MergeTags(tags, existingUserTags)

	// Update time horizon only if user hasn't manually set it
	// For now, we'll update it - in production, we'd track if user manually set it
	todo.TimeHorizon = timeHorizon

	// Set status to processed after successful analysis (unless it's completed)
	if todo.Status == models.TodoStatusProcessing {
		todo.Status = models.TodoStatusProcessed
	}

	// Update todo
	if err := a.todoRepo.Update(ctx, todo); err != nil {
		return fmt.Errorf("failed to update todo: %w", err)
	}

	log.Printf("Analyzed todo %s: tags=%v, time_horizon=%s, status=%s", todo.ID, tags, timeHorizon, todo.Status)
	return nil
}

// ProcessReprocessUserJob processes a reprocess user job
func (a *TaskAnalyzer) ProcessReprocessUserJob(ctx context.Context, job *queue.Job) error {
	// Check if user has reprocessing paused
	activity, err := a.activityRepo.GetByUserID(ctx, job.UserID)
	if err == nil && activity != nil && activity.ReprocessingPaused {
		log.Printf("Skipping reprocessing for user %s (reprocessing paused)", job.UserID)
		return nil
	}

	// Get all pending/processing todos for user
	todos, _, err := a.todoRepo.GetByUserIDPaginated(ctx, job.UserID, nil, nil, 1, 500)
	if err != nil {
		return fmt.Errorf("failed to get todos: %w", err)
	}

	// Filter to pending/processing todos only (exclude processed and completed)
	var todosToProcess []*models.Todo
	for _, todo := range todos {
		if todo.Status == models.TodoStatusPending || todo.Status == models.TodoStatusProcessing {
			todosToProcess = append(todosToProcess, todo)
		}
	}

	// Load user context
	var userContext *models.AIContext
	context, err := a.contextRepo.GetByUserID(ctx, job.UserID)
	if err == nil {
		userContext = context
	}

	// Re-analyze each todo
	updated := 0
	for _, todo := range todosToProcess {
		// Get existing user-defined tags and time horizon override
		existingUserTags := todo.Metadata.GetUserTags()
		originalTimeHorizon := todo.TimeHorizon

		// Analyze task (with due date if available)
		var tags []string
		var timeHorizon models.TimeHorizon
		if todo.DueDate != nil {
			if providerWithDueDate, ok := a.aiProvider.(ai.AIProviderWithDueDate); ok {
				tags, timeHorizon, err = providerWithDueDate.AnalyzeTaskWithDueDate(ctx, todo.Text, todo.DueDate, userContext)
			} else {
				tags, timeHorizon, err = a.aiProvider.AnalyzeTask(ctx, todo.Text, userContext)
			}
		} else {
			tags, timeHorizon, err = a.aiProvider.AnalyzeTask(ctx, todo.Text, userContext)
		}
		if err != nil {
			log.Printf("Failed to analyze todo %s: %v", todo.ID, err)
			continue
		}

		// Merge AI tags with user tags
		todo.Metadata.MergeTags(tags, existingUserTags)

		// Update time horizon only if it changed
		if timeHorizon != originalTimeHorizon {
			todo.TimeHorizon = timeHorizon
			updated++
		}

		// Update todo
		if err := a.todoRepo.Update(ctx, todo); err != nil {
			log.Printf("Failed to update todo %s: %v", todo.ID, err)
			continue
		}
	}

	log.Printf("Reprocessed %d todos for user %s, updated %d time horizons", len(todosToProcess), job.UserID, updated)
	return nil
}

// ProcessJob processes a job based on its type
func (a *TaskAnalyzer) ProcessJob(ctx context.Context, msg *queue.Message) error {
	job := msg.Job

	// Check if job should be processed now (respect NotBefore)
	if !job.ShouldProcess() {
		log.Printf("Job %s not ready yet (NotBefore: %v), skipping", job.ID, job.NotBefore)
		// Re-ack to return to queue and wait
		if ackErr := msg.Ack(); ackErr != nil {
			log.Printf("Failed to ack job for later processing: %v", ackErr)
		}
		return nil
	}

	switch job.Type {
	case queue.JobTypeTaskAnalysis:
		if err := a.ProcessTaskAnalysisJob(ctx, job); err != nil {
			return a.handleJobError(ctx, msg, job, err, "task analysis")
		}
		if ackErr := msg.Ack(); ackErr != nil {
			return fmt.Errorf("failed to ack job: %w", ackErr)
		}
		return nil

	case queue.JobTypeReprocessUser:
		if err := a.ProcessReprocessUserJob(ctx, job); err != nil {
			// Reprocessing failures are less critical, just log
			if nackErr := msg.Nack(false); nackErr != nil { // Don't requeue reprocessing jobs
				log.Printf("Failed to nack reprocessing job: %v", nackErr)
			}
			return fmt.Errorf("reprocessing failed: %w", err)
		}
		if ackErr := msg.Ack(); ackErr != nil {
			return fmt.Errorf("failed to ack reprocessing job: %w", ackErr)
		}
		return nil

	default:
		if nackErr := msg.Nack(false); nackErr != nil { // Unknown job type, send to DLQ
			log.Printf("Failed to nack unknown job type: %v", nackErr)
		}
		return fmt.Errorf("unknown job type: %s", job.Type)
	}
}

// handleJobError handles errors from job processing with intelligent retry logic
func (a *TaskAnalyzer) handleJobError(ctx context.Context, msg *queue.Message, job *queue.Job, err error, jobType string) error {
	// Check if it's a quota error (should not retry immediately)
	if ai.IsQuotaError(err) {
		log.Printf("Quota exceeded for %s job %s: %v", jobType, job.ID, err)

		// For quota errors, re-enqueue with long delay (1 hour minimum)
		retryDelay := ai.GetRetryDelay(err, job.RetryCount)
		notBefore := time.Now().Add(retryDelay)

		log.Printf("Re-enqueueing %s job %s with NotBefore=%v (quota exhausted, retry in %v)",
			jobType, job.ID, notBefore, retryDelay)

		// Create new job with delayed retry
		delayedJob := &queue.Job{
			ID:         job.ID,
			Type:       job.Type,
			UserID:     job.UserID,
			TodoID:     job.TodoID,
			NotBefore:  &notBefore,
			NotAfter:   job.NotAfter,
			Metadata:   job.Metadata,
			CreatedAt:  job.CreatedAt,
			RetryCount: job.RetryCount + 1,
			MaxRetries: job.MaxRetries,
		}

		// Ack the current message
		if ackErr := msg.Ack(); ackErr != nil {
			log.Printf("Failed to ack job before re-enqueue: %v", ackErr)
		}

		// Re-enqueue with delay using NotBefore (RabbitMQ delayed exchange will handle this)
		if a.jobQueue != nil {
			if enqueueErr := a.jobQueue.Enqueue(ctx, delayedJob); enqueueErr != nil {
				log.Printf("Failed to re-enqueue job %s with delay: %v", job.ID, enqueueErr)
				// If re-enqueue fails, send to DLQ
				return fmt.Errorf("quota exhausted, failed to re-enqueue: %w", enqueueErr)
			}
			log.Printf("Successfully re-enqueued %s job %s for retry at %v", jobType, job.ID, notBefore)
			return nil // Successfully handled
		}

		// If no queue access, nack without requeue to prevent spam
		log.Printf("Warning: No queue access, cannot re-enqueue job with delay. Sending to DLQ.")
		if nackErr := msg.Nack(false); nackErr != nil {
			log.Printf("Failed to nack quota error job: %v", nackErr)
		}

		return fmt.Errorf("quota exhausted (job %s): %w", job.ID, err)
	}

	// Check if it's a rate limit error (should retry with backoff)
	if ai.IsRateLimitError(err) {
		log.Printf("Rate limited for %s job %s: %v", jobType, job.ID, err)

		retryDelay := ai.GetRetryDelay(err, job.RetryCount)

		// For rate limits, re-enqueue with delay using NotBefore
		if job.CanRetry() && a.jobQueue != nil {
			notBefore := time.Now().Add(retryDelay)
			delayedJob := &queue.Job{
				ID:         job.ID,
				Type:       job.Type,
				UserID:     job.UserID,
				TodoID:     job.TodoID,
				NotBefore:  &notBefore,
				NotAfter:   job.NotAfter,
				Metadata:   job.Metadata,
				CreatedAt:  job.CreatedAt,
				RetryCount: job.RetryCount + 1,
				MaxRetries: job.MaxRetries,
			}

			// Ack the current message
			if ackErr := msg.Ack(); ackErr != nil {
				log.Printf("Failed to ack rate limited job: %v", ackErr)
			}

			// Re-enqueue with delay
			if enqueueErr := a.jobQueue.Enqueue(ctx, delayedJob); enqueueErr != nil {
				log.Printf("Failed to re-enqueue rate limited job %s: %v", job.ID, enqueueErr)
				// Fall back to nack with requeue
				if nackErr := msg.Nack(true); nackErr != nil {
					log.Printf("Failed to nack rate limited job: %v", nackErr)
				}
				return fmt.Errorf("rate limited, failed to re-enqueue: %w", enqueueErr)
			}

			log.Printf("Rate limited: re-enqueued %s job %s for retry at %v (delay: %v)",
				jobType, job.ID, notBefore, retryDelay)
			return nil // Successfully handled
		}

		// Fallback: nack with requeue (immediate retry)
		if job.CanRetry() {
			job.IncrementRetry()
			log.Printf("Rate limit: will retry job %s immediately (attempt %d/%d)",
				job.ID, job.RetryCount, job.MaxRetries)
			if nackErr := msg.Nack(true); nackErr != nil {
				log.Printf("Failed to nack rate limited job: %v", nackErr)
			}
			// Return error to signal worker to wait before processing next job
			return fmt.Errorf("rate limited (will retry): %w", err)
		}
	}

	// For other errors, use standard retry logic
	if job.CanRetry() {
		job.IncrementRetry()
		log.Printf("%s job %s failed (attempt %d/%d): %v, will retry", jobType, job.ID, job.RetryCount, job.MaxRetries, err)
		if nackErr := msg.Nack(true); nackErr != nil {
			log.Printf("Failed to nack job: %v", nackErr)
		}
		return fmt.Errorf("job failed (will retry): %w", err)
	}

	// Max retries exceeded, send to DLQ
	log.Printf("%s job %s failed after %d retries: %v, sending to DLQ", jobType, job.ID, job.MaxRetries, err)
	if nackErr := msg.Nack(false); nackErr != nil {
		log.Printf("Failed to nack job to DLQ: %v", nackErr)
	}
	return fmt.Errorf("job failed (max retries): %w", err)
}
