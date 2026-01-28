package workers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/queue"
	"github.com/benvon/smart-todo/internal/services/ai"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TagStatsCache represents a cached tag statistics entry
type TagStatsCache struct {
	stats   *models.TagStatistics
	expires time.Time
	mu      sync.RWMutex
}

// TaskAnalyzer processes task analysis jobs
type TaskAnalyzer struct {
	aiProvider    ai.AIProvider
	todoRepo      database.TodoRepositoryInterface
	contextRepo   database.AIContextRepositoryInterface
	activityRepo  database.UserActivityRepositoryInterface
	tagStatsRepo  database.TagStatisticsRepositoryInterface // For loading tag statistics to guide AI
	jobQueue      queue.JobQueue                            // For re-enqueueing jobs with delays
	tagStatsCache map[uuid.UUID]*TagStatsCache              // Cache for tag statistics by user ID
	cacheMu       sync.RWMutex                              // Mutex for cache map
	cacheTTL      time.Duration                             // Cache TTL (default 3 minutes)
	logger        *zap.Logger
}

// NewTaskAnalyzer creates a new task analyzer
func NewTaskAnalyzer(
	aiProvider ai.AIProvider,
	todoRepo database.TodoRepositoryInterface,
	contextRepo database.AIContextRepositoryInterface,
	activityRepo database.UserActivityRepositoryInterface,
	tagStatsRepo database.TagStatisticsRepositoryInterface,
	jobQueue queue.JobQueue,
	logger *zap.Logger,
) *TaskAnalyzer {
	return &TaskAnalyzer{
		aiProvider:    aiProvider,
		todoRepo:      todoRepo,
		contextRepo:   contextRepo,
		activityRepo:  activityRepo,
		tagStatsRepo:  tagStatsRepo,
		jobQueue:      jobQueue,
		tagStatsCache: make(map[uuid.UUID]*TagStatsCache),
		cacheTTL:      3 * time.Minute, // Default 3 minutes TTL
		logger:        logger,
	}
}

// getTagStatistics retrieves tag statistics for a user, using cache when available
func (a *TaskAnalyzer) getTagStatistics(ctx context.Context, userID uuid.UUID) (*models.TagStatistics, error) {
	if a.tagStatsRepo == nil {
		return nil, nil
	}

	// Check cache first
	a.cacheMu.RLock()
	cache, exists := a.tagStatsCache[userID]
	if exists {
		cache.mu.RLock()
		if time.Now().Before(cache.expires) && cache.stats != nil {
			stats := cache.stats
			cache.mu.RUnlock()
			a.cacheMu.RUnlock()
			return stats, nil
		}
		cache.mu.RUnlock()
	}
	a.cacheMu.RUnlock()

	// Fetch from database
	stats, err := a.tagStatsRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if stats == nil {
		return nil, nil
	}

	// Update cache
	a.cacheMu.Lock()
	a.tagStatsCache[userID] = &TagStatsCache{
		stats:   stats,
		expires: time.Now().Add(a.cacheTTL),
		mu:      sync.RWMutex{},
	}
	a.cacheMu.Unlock()

	return stats, nil
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

	// Save original tags for tag change detection
	originalTags := todo.Metadata.CategoryTags

	// Verify todo belongs to user
	if todo.UserID != job.UserID {
		return fmt.Errorf("todo does not belong to user")
	}

	// Load user context
	var userContext *models.AIContext
	aiContext, err := a.contextRepo.GetByUserID(ctx, job.UserID)
	if err == nil {
		userContext = aiContext
	}

	// Load tag statistics to guide AI tag selection
	var tagStats *models.TagStatistics
	stats, err := a.getTagStatistics(ctx, job.UserID)
	if err == nil && stats != nil {
		tagStats = stats
	}

	// Check if user has reprocessing paused
	activity, err := a.activityRepo.GetByUserID(ctx, job.UserID)
	if err == nil && activity != nil && activity.ReprocessingPaused {
		a.logger.Debug("skipping_analysis_reprocessing_paused",
			zap.String("user_id", job.UserID.String()),
		)
		return nil
	}

	// Set status to processing before starting analysis
	// Only update if currently pending (don't override completed status)
	if todo.Status == models.TodoStatusPending {
		todo.Status = models.TodoStatusProcessing
		if err := a.todoRepo.Update(ctx, todo, originalTags); err != nil {
			a.logger.Warn("failed_to_update_todo_status_to_processing",
				zap.String("todo_id", todo.ID.String()),
				zap.Error(err),
			)
			// Continue with analysis even if status update fails
		} else {
			a.logger.Debug("set_todo_status_to_processing",
				zap.String("todo_id", todo.ID.String()),
			)
		}
	}

	// Get creation time from metadata if available, otherwise use CreatedAt
	createdAt := todo.CreatedAt
	if todo.Metadata.TimeEntered != nil && *todo.Metadata.TimeEntered != "" {
		if parsedTime, parseErr := time.Parse(time.RFC3339, *todo.Metadata.TimeEntered); parseErr == nil {
			createdAt = parsedTime
		}
		// If parsing fails, fall back to CreatedAt
	}

	// Add user_id and todo_id to context for logging
	ctxWithIDs := context.WithValue(ctx, ai.UserIDContextKey(), job.UserID)
	ctxWithIDs = context.WithValue(ctxWithIDs, ai.TodoIDContextKey(), todo.ID)

	// Analyze task (with due date if available)
	var tags []string
	var timeHorizon models.TimeHorizon
	// Check if provider supports due date analysis
	if todo.DueDate != nil {
		if providerWithDueDate, ok := a.aiProvider.(ai.AIProviderWithDueDate); ok {
			tags, timeHorizon, err = providerWithDueDate.AnalyzeTaskWithDueDate(ctxWithIDs, todo.Text, todo.DueDate, createdAt, userContext, tagStats)
		} else {
			tags, timeHorizon, err = a.aiProvider.AnalyzeTask(ctxWithIDs, todo.Text, userContext)
		}
	} else {
		// Even without due date, use AnalyzeTaskWithDueDate if available to pass creation time and tag stats
		if providerWithDueDate, ok := a.aiProvider.(ai.AIProviderWithDueDate); ok {
			tags, timeHorizon, err = providerWithDueDate.AnalyzeTaskWithDueDate(ctxWithIDs, todo.Text, nil, createdAt, userContext, tagStats)
		} else {
			tags, timeHorizon, err = a.aiProvider.AnalyzeTask(ctxWithIDs, todo.Text, userContext)
		}
	}
	if err != nil {
		// On error, set status back to pending so it can be retried
		if todo.Status == models.TodoStatusProcessing {
			todo.Status = models.TodoStatusPending
			if updateErr := a.todoRepo.Update(ctx, todo, originalTags); updateErr != nil {
				a.logger.Warn("failed_to_reset_todo_status_to_pending",
					zap.String("todo_id", todo.ID.String()),
					zap.Error(updateErr),
				)
			}
		}
		return fmt.Errorf("failed to analyze task: %w", err)
	}

	// Get existing user-defined tags (preserve them)
	existingUserTags := todo.Metadata.GetUserTags()

	// Merge AI tags with user tags (user tags override)
	todo.Metadata.MergeTags(tags, existingUserTags)

	// Update time horizon only if user hasn't manually set it
	// Check if user has manually overridden the time horizon
	if todo.Metadata.TimeHorizonUserOverride == nil || !*todo.Metadata.TimeHorizonUserOverride {
		todo.TimeHorizon = timeHorizon
	}
	// If TimeHorizonUserOverride is true, preserve the existing time_horizon

	// Set status to processed after successful analysis (unless it's completed)
	if todo.Status == models.TodoStatusProcessing {
		todo.Status = models.TodoStatusProcessed
	}

	// Update todo (tag change detection is handled automatically by the repository)
	if err := a.todoRepo.Update(ctx, todo, originalTags); err != nil {
		return fmt.Errorf("failed to update todo: %w", err)
	}

	a.logger.Info("analyzed_todo",
		zap.String("todo_id", todo.ID.String()),
		zap.Strings("tags", tags),
		zap.String("time_horizon", string(timeHorizon)),
		zap.String("status", string(todo.Status)),
		zap.String("user_id", job.UserID.String()),
	)
	return nil
}

// ProcessReprocessUserJob processes a reprocess user job
func (a *TaskAnalyzer) ProcessReprocessUserJob(ctx context.Context, job *queue.Job) error {
	// Check if user has reprocessing paused
	activity, err := a.activityRepo.GetByUserID(ctx, job.UserID)
	if err == nil && activity != nil && activity.ReprocessingPaused {
		a.logger.Debug("skipping_reprocessing_paused",
			zap.String("user_id", job.UserID.String()),
		)
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
	aiContext, err := a.contextRepo.GetByUserID(ctx, job.UserID)
	if err == nil {
		userContext = aiContext
	}

	// Re-analyze each todo
	updated := 0
	for _, todo := range todosToProcess {
		// Save original tags for tag change detection
		originalTags := todo.Metadata.CategoryTags

		// Get existing user-defined tags and time horizon override
		existingUserTags := todo.Metadata.GetUserTags()
		originalTimeHorizon := todo.TimeHorizon

		// Get creation time from metadata if available, otherwise use CreatedAt
		createdAt := todo.CreatedAt
		if todo.Metadata.TimeEntered != nil && *todo.Metadata.TimeEntered != "" {
			if parsedTime, parseErr := time.Parse(time.RFC3339, *todo.Metadata.TimeEntered); parseErr == nil {
				createdAt = parsedTime
			}
			// If parsing fails, fall back to CreatedAt
		}

		// Load tag statistics for this user (reuse from cache)
		var tagStats *models.TagStatistics
		stats, err := a.getTagStatistics(ctx, job.UserID)
		if err == nil && stats != nil {
			tagStats = stats
		}

		// Add user_id and todo_id to context for logging
		ctxWithIDs := context.WithValue(ctx, ai.UserIDContextKey(), job.UserID)
		ctxWithIDs = context.WithValue(ctxWithIDs, ai.TodoIDContextKey(), todo.ID)

		// Analyze task (with due date if available)
		var tags []string
		var timeHorizon models.TimeHorizon
		if todo.DueDate != nil {
			if providerWithDueDate, ok := a.aiProvider.(ai.AIProviderWithDueDate); ok {
				tags, timeHorizon, err = providerWithDueDate.AnalyzeTaskWithDueDate(ctxWithIDs, todo.Text, todo.DueDate, createdAt, userContext, tagStats)
			} else {
				tags, timeHorizon, err = a.aiProvider.AnalyzeTask(ctxWithIDs, todo.Text, userContext)
			}
		} else {
			// Even without due date, use AnalyzeTaskWithDueDate if available to pass creation time and tag stats
			if providerWithDueDate, ok := a.aiProvider.(ai.AIProviderWithDueDate); ok {
				tags, timeHorizon, err = providerWithDueDate.AnalyzeTaskWithDueDate(ctxWithIDs, todo.Text, nil, createdAt, userContext, tagStats)
			} else {
				tags, timeHorizon, err = a.aiProvider.AnalyzeTask(ctxWithIDs, todo.Text, userContext)
			}
		}
		if err != nil {
			a.logger.Error("failed_to_analyze_todo",
				zap.String("todo_id", todo.ID.String()),
				zap.String("user_id", job.UserID.String()),
				zap.Error(err),
			)
			continue
		}

		// Merge AI tags with user tags
		todo.Metadata.MergeTags(tags, existingUserTags)

		// Update time horizon only if user hasn't manually set it
		// Check if user has manually overridden the time horizon
		if todo.Metadata.TimeHorizonUserOverride == nil || !*todo.Metadata.TimeHorizonUserOverride {
			if timeHorizon != originalTimeHorizon {
				todo.TimeHorizon = timeHorizon
				updated++
			}
		}
		// If TimeHorizonUserOverride is true, preserve the existing time_horizon

		// Update todo (tag change detection is handled automatically by the repository)
		if err := a.todoRepo.Update(ctx, todo, originalTags); err != nil {
			a.logger.Error("failed_to_update_todo",
				zap.String("todo_id", todo.ID.String()),
				zap.String("user_id", job.UserID.String()),
				zap.Error(err),
			)
			continue
		}
	}

	a.logger.Info("reprocessed_todos",
		zap.Int("todos_processed", len(todosToProcess)),
		zap.Int("time_horizons_updated", updated),
		zap.String("user_id", job.UserID.String()),
	)
	return nil
}

// ProcessJob processes a job based on its type
func (a *TaskAnalyzer) ProcessJob(ctx context.Context, msg queue.MessageInterface) error {
	job := msg.GetJob()

	// Check if job should be processed now (respect NotBefore)
	if !job.ShouldProcess() {
		a.logger.Debug("job_not_ready_yet",
			zap.String("job_id", job.ID.String()),
			zap.String("job_type", string(job.Type)),
			zap.Any("not_before", job.NotBefore),
		)
		// Re-ack to return to queue and wait
		if ackErr := msg.Ack(); ackErr != nil {
			a.logger.Warn("failed_to_ack_job_for_later_processing",
				zap.String("job_id", job.ID.String()),
				zap.Error(ackErr),
			)
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
				a.logger.Warn("failed_to_nack_reprocessing_job",
					zap.String("job_id", job.ID.String()),
					zap.Error(nackErr),
				)
			}
			return fmt.Errorf("reprocessing failed: %w", err)
		}
		if ackErr := msg.Ack(); ackErr != nil {
			return fmt.Errorf("failed to ack reprocessing job: %w", ackErr)
		}
		return nil

	default:
		if nackErr := msg.Nack(false); nackErr != nil { // Unknown job type, send to DLQ
			a.logger.Error("failed_to_nack_unknown_job_type",
				zap.String("job_id", job.ID.String()),
				zap.String("job_type", string(job.Type)),
				zap.Error(nackErr),
			)
		}
		return fmt.Errorf("unknown job type: %s", job.Type)
	}
}

// handleJobError handles errors from job processing with intelligent retry logic
func (a *TaskAnalyzer) handleJobError(ctx context.Context, msg queue.MessageInterface, job *queue.Job, err error, jobType string) error {
	// Check if it's a quota error (should not retry immediately)
	if ai.IsQuotaError(err) {
		a.logger.Warn("quota_exceeded",
			zap.String("job_type", jobType),
			zap.String("job_id", job.ID.String()),
			zap.Error(err),
		)

		// For quota errors, re-enqueue with long delay (1 hour minimum)
		retryDelay := ai.GetRetryDelay(err, job.RetryCount)
		notBefore := time.Now().Add(retryDelay)

		a.logger.Info("re_enqueueing_job_quota_exhausted",
			zap.String("job_type", jobType),
			zap.String("job_id", job.ID.String()),
			zap.Time("not_before", notBefore),
			zap.Duration("retry_delay", retryDelay),
		)

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
			a.logger.Warn("failed_to_ack_job_before_re_enqueue",
				zap.String("job_id", job.ID.String()),
				zap.Error(ackErr),
			)
		}

		// Re-enqueue with delay using NotBefore (RabbitMQ delayed exchange will handle this)
		if a.jobQueue != nil {
			if enqueueErr := a.jobQueue.Enqueue(ctx, delayedJob); enqueueErr != nil {
				a.logger.Error("failed_to_re_enqueue_job_with_delay",
					zap.String("job_id", job.ID.String()),
					zap.Error(enqueueErr),
				)
				// If re-enqueue fails, send to DLQ
				return fmt.Errorf("quota exhausted, failed to re-enqueue: %w", enqueueErr)
			}
			a.logger.Info("successfully_re_enqueued_job",
				zap.String("job_type", jobType),
				zap.String("job_id", job.ID.String()),
				zap.Time("retry_at", notBefore),
			)
			return nil // Successfully handled
		}

		// If no queue access, nack without requeue to prevent spam
		a.logger.Warn("no_queue_access_cannot_re_enqueue",
			zap.String("job_id", job.ID.String()),
		)
		if nackErr := msg.Nack(false); nackErr != nil {
			a.logger.Warn("failed_to_nack_quota_error_job",
				zap.String("job_id", job.ID.String()),
				zap.Error(nackErr),
			)
		}

		return fmt.Errorf("quota exhausted (job %s): %w", job.ID, err)
	}

	// Check if it's a rate limit error (should retry with backoff)
	if ai.IsRateLimitError(err) {
		a.logger.Warn("rate_limited",
			zap.String("job_type", jobType),
			zap.String("job_id", job.ID.String()),
			zap.Error(err),
		)

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
				a.logger.Warn("failed_to_ack_rate_limited_job",
					zap.String("job_id", job.ID.String()),
					zap.Error(ackErr),
				)
			}

			// Re-enqueue with delay
			if enqueueErr := a.jobQueue.Enqueue(ctx, delayedJob); enqueueErr != nil {
				a.logger.Error("failed_to_re_enqueue_rate_limited_job",
					zap.String("job_id", job.ID.String()),
					zap.Error(enqueueErr),
				)
				// Fall back to nack with requeue
				if nackErr := msg.Nack(true); nackErr != nil {
					a.logger.Warn("failed_to_nack_rate_limited_job",
						zap.String("job_id", job.ID.String()),
						zap.Error(nackErr),
					)
				}
				return fmt.Errorf("rate limited, failed to re-enqueue: %w", enqueueErr)
			}

			a.logger.Info("rate_limited_re_enqueued",
				zap.String("job_type", jobType),
				zap.String("job_id", job.ID.String()),
				zap.Time("retry_at", notBefore),
				zap.Duration("retry_delay", retryDelay),
			)
			return nil // Successfully handled
		}

		// Fallback: nack with requeue (immediate retry)
		if job.CanRetry() {
			job.IncrementRetry()
			a.logger.Info("rate_limit_will_retry_immediately",
				zap.String("job_id", job.ID.String()),
				zap.Int("attempt", job.RetryCount),
				zap.Int("max_retries", job.MaxRetries),
			)
			if nackErr := msg.Nack(true); nackErr != nil {
				a.logger.Warn("failed_to_nack_rate_limited_job",
					zap.String("job_id", job.ID.String()),
					zap.Error(nackErr),
				)
			}
			// Return error to signal worker to wait before processing next job
			return fmt.Errorf("rate limited (will retry): %w", err)
		}
	}

	// For other errors, use standard retry logic
	if job.CanRetry() {
		job.IncrementRetry()
		a.logger.Warn("job_failed_will_retry",
			zap.String("job_type", jobType),
			zap.String("job_id", job.ID.String()),
			zap.Int("attempt", job.RetryCount),
			zap.Int("max_retries", job.MaxRetries),
			zap.Error(err),
		)
		if nackErr := msg.Nack(true); nackErr != nil {
			a.logger.Warn("failed_to_nack_job",
				zap.String("job_id", job.ID.String()),
				zap.Error(nackErr),
			)
		}
		return fmt.Errorf("job failed (will retry): %w", err)
	}

	// Max retries exceeded, send to DLQ
	a.logger.Error("job_failed_max_retries_exceeded",
		zap.String("job_type", jobType),
		zap.String("job_id", job.ID.String()),
		zap.Int("max_retries", job.MaxRetries),
		zap.Error(err),
	)
	if nackErr := msg.Nack(false); nackErr != nil {
		a.logger.Warn("failed_to_nack_job_to_dlq",
			zap.String("job_id", job.ID.String()),
			zap.Error(nackErr),
		)
	}
	return fmt.Errorf("job failed (max retries): %w", err)
}
