package workers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	logpkg "github.com/benvon/smart-todo/internal/logger"
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

// JobProcessor processes a single job. Returns an error to trigger retry/DLQ handling.
type JobProcessor func(ctx context.Context, job *queue.Job) error

type processorEntry struct {
	proc              JobProcessor
	useHandleJobError bool
}

// TaskAnalyzer processes task analysis jobs
type TaskAnalyzer struct {
	aiProvider    ai.AIProvider
	todoRepo      database.TodoRepositoryInterface
	contextRepo   database.AIContextRepositoryInterface
	activityRepo  database.UserActivityRepositoryInterface
	tagStatsRepo  database.TagStatisticsRepositoryInterface
	jobQueue      queue.JobQueue
	tagStatsCache map[uuid.UUID]*TagStatsCache
	cacheMu       sync.RWMutex
	cacheTTL      time.Duration
	logger        *zap.Logger
	registry      map[queue.JobType]processorEntry
}

// NewTaskAnalyzer creates a new task analyzer and registers task_analysis and reprocess_user processors.
func NewTaskAnalyzer(
	aiProvider ai.AIProvider,
	todoRepo database.TodoRepositoryInterface,
	contextRepo database.AIContextRepositoryInterface,
	activityRepo database.UserActivityRepositoryInterface,
	tagStatsRepo database.TagStatisticsRepositoryInterface,
	jobQueue queue.JobQueue,
	logger *zap.Logger,
) *TaskAnalyzer {
	a := &TaskAnalyzer{
		aiProvider:    aiProvider,
		todoRepo:      todoRepo,
		contextRepo:   contextRepo,
		activityRepo:  activityRepo,
		tagStatsRepo:  tagStatsRepo,
		jobQueue:      jobQueue,
		tagStatsCache: make(map[uuid.UUID]*TagStatsCache),
		cacheTTL:      3 * time.Minute,
		logger:        logger,
		registry:      make(map[queue.JobType]processorEntry),
	}
	a.RegisterProcessor(queue.JobTypeTaskAnalysis, a.ProcessTaskAnalysisJob, true)
	a.RegisterProcessor(queue.JobTypeReprocessUser, a.ProcessReprocessUserJob, false)
	return a
}

// RegisterProcessor registers a processor for a job type. useHandleJobError enables handleJobError on failure.
func (a *TaskAnalyzer) RegisterProcessor(typ queue.JobType, proc JobProcessor, useHandleJobError bool) {
	a.registry[typ] = processorEntry{proc: proc, useHandleJobError: useHandleJobError}
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

// todoCreatedAt returns CreatedAt, or TimeEntered from metadata if present and parseable.
func todoCreatedAt(todo *models.Todo) time.Time {
	if todo.Metadata.TimeEntered != nil && *todo.Metadata.TimeEntered != "" {
		if t, err := time.Parse(time.RFC3339, *todo.Metadata.TimeEntered); err == nil {
			return t
		}
	}
	return todo.CreatedAt
}

// analyzeTodoWithProvider runs AI analysis for a todo. It uses AnalyzeTaskWithDueDate when
// the provider supports it, otherwise falls back to AnalyzeTask.
func (a *TaskAnalyzer) analyzeTodoWithProvider(ctx context.Context, job *queue.Job, todo *models.Todo, userContext *models.AIContext, tagStats *models.TagStatistics) ([]string, models.TimeHorizon, error) {
	createdAt := todoCreatedAt(todo)
	ctxWithIDs := context.WithValue(ctx, ai.UserIDContextKey(), job.UserID)
	ctxWithIDs = context.WithValue(ctxWithIDs, ai.TodoIDContextKey(), todo.ID)

	providerWithDueDate, ok := a.aiProvider.(ai.AIProviderWithDueDate)
	if ok {
		return providerWithDueDate.AnalyzeTaskWithDueDate(ctxWithIDs, todo.Text, todo.DueDate, createdAt, userContext, tagStats)
	}
	return a.aiProvider.AnalyzeTask(ctxWithIDs, todo.Text, userContext)
}

// ProcessTaskAnalysisJob processes a task analysis job
func (a *TaskAnalyzer) ProcessTaskAnalysisJob(ctx context.Context, job *queue.Job) error {
	if job.TodoID == nil {
		return fmt.Errorf("todo_id is required for task analysis job")
	}
	todo, err := a.todoRepo.GetByUserIDAndID(ctx, job.UserID, *job.TodoID)
	if err != nil {
		return fmt.Errorf("failed to get todo: %w", err)
	}
	originalTags := todo.Metadata.CategoryTags
	userContext, _ := a.contextRepo.GetByUserID(ctx, job.UserID)
	tagStats, _ := a.getTagStatistics(ctx, job.UserID)
	if a.shouldSkipAnalysisForPausedUser(ctx, job.UserID) {
		return nil
	}
	a.setTodoProcessingIfPending(ctx, todo, originalTags)
	tags, timeHorizon, err := a.analyzeTodoWithProvider(ctx, job, todo, userContext, tagStats)
	if err != nil {
		a.resetTodoToPendingOnError(ctx, todo, originalTags)
		return fmt.Errorf("failed to analyze task: %w", err)
	}
	a.applyAnalysisResultToTodo(todo, tags, timeHorizon)
	if err := a.todoRepo.Update(ctx, todo, originalTags); err != nil {
		return fmt.Errorf("failed to update todo: %w", err)
	}
	a.logAnalyzedTodo(todo, tags, timeHorizon, job.UserID)
	return nil
}

func (a *TaskAnalyzer) shouldSkipAnalysisForPausedUser(ctx context.Context, userID uuid.UUID) bool {
	activity, err := a.activityRepo.GetByUserID(ctx, userID)
	if err != nil || activity == nil || !activity.ReprocessingPaused {
		return false
	}
	a.logger.Debug("skipping_analysis_reprocessing_paused",
		zap.String("user_id", logpkg.SanitizeUserID(userID.String())),
	)
	return true
}

func (a *TaskAnalyzer) setTodoProcessingIfPending(ctx context.Context, todo *models.Todo, originalTags []string) {
	if todo.Status != models.TodoStatusPending {
		return
	}
	todo.Status = models.TodoStatusProcessing
	if err := a.todoRepo.Update(ctx, todo, originalTags); err != nil {
		a.logger.Warn("failed_to_update_todo_status_to_processing",
			zap.String("operation", "process_task_analysis_job"),
			zap.String("todo_id", logpkg.SanitizeUserID(todo.ID.String())),
			zap.String("error", logpkg.SanitizeError(err)),
		)
		return
	}
	a.logger.Debug("set_todo_status_to_processing",
		zap.String("todo_id", logpkg.SanitizeUserID(todo.ID.String())),
	)
}

func (a *TaskAnalyzer) resetTodoToPendingOnError(ctx context.Context, todo *models.Todo, originalTags []string) {
	if todo.Status != models.TodoStatusProcessing {
		return
	}
	todo.Status = models.TodoStatusPending
	if err := a.todoRepo.Update(ctx, todo, originalTags); err != nil {
		a.logger.Warn("failed_to_reset_todo_status_to_pending",
			zap.String("todo_id", logpkg.SanitizeUserID(todo.ID.String())),
			zap.String("error", logpkg.SanitizeError(err)),
		)
	}
}

func (a *TaskAnalyzer) applyAnalysisResultToTodo(todo *models.Todo, tags []string, timeHorizon models.TimeHorizon) {
	existingUserTags := todo.Metadata.GetUserTags()
	todo.Metadata.MergeTags(tags, existingUserTags)
	if todo.Metadata.TimeHorizonUserOverride == nil || !*todo.Metadata.TimeHorizonUserOverride {
		todo.TimeHorizon = timeHorizon
	}
	if todo.Status == models.TodoStatusProcessing {
		todo.Status = models.TodoStatusProcessed
	}
}

func (a *TaskAnalyzer) logAnalyzedTodo(todo *models.Todo, tags []string, timeHorizon models.TimeHorizon, userID uuid.UUID) {
	a.logger.Info("analyzed_todo",
		zap.String("todo_id", logpkg.SanitizeUserID(todo.ID.String())),
		zap.Strings("tags", tags),
		zap.String("time_horizon", string(timeHorizon)),
		zap.String("status", string(todo.Status)),
		zap.String("user_id", logpkg.SanitizeUserID(userID.String())),
	)
}

// ProcessReprocessUserJob processes a reprocess user job
func (a *TaskAnalyzer) ProcessReprocessUserJob(ctx context.Context, job *queue.Job) error {
	if a.shouldSkipReprocessingForPausedUser(ctx, job.UserID) {
		return nil
	}
	todos, _, err := a.todoRepo.GetByUserIDPaginated(ctx, job.UserID, nil, nil, 1, 500)
	if err != nil {
		return fmt.Errorf("failed to get todos: %w", err)
	}
	todosToProcess := filterPendingOrProcessingTodos(todos)
	userContext, _ := a.contextRepo.GetByUserID(ctx, job.UserID)
	updated := a.reprocessTodos(ctx, job, todosToProcess, userContext)
	a.logger.Info("reprocessed_todos",
		zap.Int("todos_processed", len(todosToProcess)),
		zap.Int("time_horizons_updated", updated),
		zap.String("user_id", logpkg.SanitizeUserID(job.UserID.String())),
	)
	return nil
}

func (a *TaskAnalyzer) shouldSkipReprocessingForPausedUser(ctx context.Context, userID uuid.UUID) bool {
	activity, err := a.activityRepo.GetByUserID(ctx, userID)
	if err != nil || activity == nil || !activity.ReprocessingPaused {
		return false
	}
	a.logger.Debug("skipping_reprocessing_paused",
		zap.String("user_id", logpkg.SanitizeUserID(userID.String())),
	)
	return true
}

func filterPendingOrProcessingTodos(todos []*models.Todo) []*models.Todo {
	var out []*models.Todo
	for _, t := range todos {
		if t.Status == models.TodoStatusPending || t.Status == models.TodoStatusProcessing {
			out = append(out, t)
		}
	}
	return out
}

func (a *TaskAnalyzer) reprocessTodos(ctx context.Context, job *queue.Job, todos []*models.Todo, userContext *models.AIContext) int {
	tagStats, _ := a.getTagStatistics(ctx, job.UserID)
	updated := 0
	for _, todo := range todos {
		originalTags := todo.Metadata.CategoryTags
		existingUserTags := todo.Metadata.GetUserTags()
		originalTimeHorizon := todo.TimeHorizon
		tags, timeHorizon, err := a.analyzeTodoWithProvider(ctx, job, todo, userContext, tagStats)
		if err != nil {
			a.logger.Error("failed_to_analyze_todo",
				zap.String("operation", "reprocess_user_job"),
				zap.String("todo_id", logpkg.SanitizeUserID(todo.ID.String())),
				zap.String("user_id", logpkg.SanitizeUserID(job.UserID.String())),
				zap.String("error", logpkg.SanitizeError(err)),
			)
			continue
		}
		todo.Metadata.MergeTags(tags, existingUserTags)
		if (todo.Metadata.TimeHorizonUserOverride == nil || !*todo.Metadata.TimeHorizonUserOverride) && timeHorizon != originalTimeHorizon {
			todo.TimeHorizon = timeHorizon
			updated++
		}
		if err := a.todoRepo.Update(ctx, todo, originalTags); err != nil {
			a.logger.Error("failed_to_update_todo",
				zap.String("operation", "reprocess_user_job"),
				zap.String("todo_id", logpkg.SanitizeUserID(todo.ID.String())),
				zap.String("user_id", logpkg.SanitizeUserID(job.UserID.String())),
				zap.String("error", logpkg.SanitizeError(err)),
			)
		}
	}
	return updated
}

// ProcessJob processes a job based on its type using the processor registry.
func (a *TaskAnalyzer) ProcessJob(ctx context.Context, msg queue.MessageInterface) error {
	job := msg.GetJob()
	if !job.ShouldProcess() {
		fields := []zap.Field{
			zap.String("job_id", logpkg.SanitizeUserID(job.ID.String())),
			zap.String("job_type", string(job.Type)),
		}
		if job.NotBefore != nil {
			fields = append(fields, zap.Time("not_before", *job.NotBefore))
		}
		a.logger.Debug("job_not_ready_yet", fields...)
		if ackErr := msg.Ack(); ackErr != nil {
			a.logger.Warn("failed_to_ack_job_for_later_processing",
				zap.String("job_id", logpkg.SanitizeUserID(job.ID.String())),
				zap.String("error", logpkg.SanitizeError(ackErr)),
			)
		}
		return nil
	}
	ent, ok := a.registry[job.Type]
	if !ok {
		a.nackOrLog(msg, false, job.ID.String())
		a.logger.Error("unknown_job_type",
			zap.String("operation", "process_job"),
			zap.String("job_id", logpkg.SanitizeUserID(job.ID.String())),
			zap.String("job_type", string(job.Type)),
		)
		return fmt.Errorf("unknown job type: %s", job.Type)
	}
	jobTypeLabel := string(job.Type)
	if err := ent.proc(ctx, job); err != nil {
		if ent.useHandleJobError {
			return a.handleJobError(ctx, msg, job, err, jobTypeLabel)
		}
		a.nackOrLog(msg, false, job.ID.String())
		return fmt.Errorf("job failed: %w", err)
	}
	if ackErr := msg.Ack(); ackErr != nil {
		return fmt.Errorf("failed to ack job: %w", ackErr)
	}
	return nil
}

func buildDelayedJob(job *queue.Job, notBefore time.Time) *queue.Job {
	return &queue.Job{
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
}

func (a *TaskAnalyzer) ackOrLog(msg queue.MessageInterface, jobID string) {
	if err := msg.Ack(); err != nil {
		a.logger.Warn("failed_to_ack_job",
			zap.String("job_id", logpkg.SanitizeUserID(jobID)),
			zap.String("error", logpkg.SanitizeError(err)),
		)
	}
}

func (a *TaskAnalyzer) nackOrLog(msg queue.MessageInterface, requeue bool, jobID string) {
	if err := msg.Nack(requeue); err != nil {
		a.logger.Warn("failed_to_nack_job",
			zap.String("job_id", logpkg.SanitizeUserID(jobID)),
			zap.String("error", logpkg.SanitizeError(err)),
		)
	}
}

func (a *TaskAnalyzer) handleQuotaError(ctx context.Context, msg queue.MessageInterface, job *queue.Job, err error, jobType string) error {
	a.logger.Warn("quota_exceeded",
		zap.String("operation", "handle_job_error"),
		zap.String("job_type", jobType),
		zap.String("job_id", logpkg.SanitizeUserID(job.ID.String())),
		zap.String("error", logpkg.SanitizeError(err)),
	)
	retryDelay := ai.GetRetryDelay(err, job.RetryCount)
	notBefore := time.Now().Add(retryDelay)
	a.logger.Info("re_enqueueing_job_quota_exhausted",
		zap.String("job_type", jobType),
		zap.String("job_id", logpkg.SanitizeUserID(job.ID.String())),
		zap.Time("not_before", notBefore),
		zap.Duration("retry_delay", retryDelay),
	)
	if a.jobQueue == nil {
		a.logger.Warn("no_queue_access_cannot_re_enqueue", zap.String("job_id", logpkg.SanitizeUserID(job.ID.String())))
		a.nackOrLog(msg, false, job.ID.String())
		return fmt.Errorf("quota exhausted (job %s): %w", job.ID, err)
	}
	a.ackOrLog(msg, job.ID.String())
	delayedJob := buildDelayedJob(job, notBefore)
	if enqErr := a.jobQueue.Enqueue(ctx, delayedJob); enqErr != nil {
		a.logger.Error("failed_to_re_enqueue_job_with_delay",
			zap.String("job_id", logpkg.SanitizeUserID(job.ID.String())),
			zap.String("error", logpkg.SanitizeError(enqErr)),
		)
		return fmt.Errorf("quota exhausted, failed to re-enqueue: %w", enqErr)
	}
	a.logger.Info("successfully_re_enqueued_job",
		zap.String("job_type", jobType),
		zap.String("job_id", logpkg.SanitizeUserID(job.ID.String())),
		zap.Time("retry_at", notBefore),
	)
	return nil
}

func (a *TaskAnalyzer) handleRateLimitError(ctx context.Context, msg queue.MessageInterface, job *queue.Job, err error, jobType string) error {
	a.logger.Warn("rate_limited",
		zap.String("operation", "handle_job_error"),
		zap.String("job_type", jobType),
		zap.String("job_id", logpkg.SanitizeUserID(job.ID.String())),
		zap.String("error", logpkg.SanitizeError(err)),
	)
	retryDelay := ai.GetRetryDelay(err, job.RetryCount)
	if job.CanRetry() && a.jobQueue != nil {
		notBefore := time.Now().Add(retryDelay)
		delayedJob := buildDelayedJob(job, notBefore)
		a.ackOrLog(msg, job.ID.String())
		if enqErr := a.jobQueue.Enqueue(ctx, delayedJob); enqErr != nil {
			a.logger.Error("failed_to_re_enqueue_rate_limited_job",
				zap.String("job_id", logpkg.SanitizeUserID(job.ID.String())),
				zap.String("error", logpkg.SanitizeError(enqErr)),
			)
			return fmt.Errorf("rate limited, failed to re-enqueue: %w", enqErr)
		}
		a.logger.Info("rate_limited_re_enqueued",
			zap.String("job_type", jobType),
			zap.String("job_id", logpkg.SanitizeUserID(job.ID.String())),
			zap.Time("retry_at", notBefore),
			zap.Duration("retry_delay", retryDelay),
		)
		return nil
	}
	if job.CanRetry() {
		job.IncrementRetry()
		a.logger.Info("rate_limit_will_retry_immediately",
			zap.String("job_id", logpkg.SanitizeUserID(job.ID.String())),
			zap.Int("attempt", job.RetryCount),
			zap.Int("max_retries", job.MaxRetries),
		)
		a.nackOrLog(msg, true, job.ID.String())
		return fmt.Errorf("rate limited (will retry): %w", err)
	}
	return a.sendToDLQ(msg, job, err, jobType)
}

func (a *TaskAnalyzer) handleGenericRetry(msg queue.MessageInterface, job *queue.Job, err error, jobType string) error {
	job.IncrementRetry()
	a.logger.Warn("job_failed_will_retry",
		zap.String("job_type", jobType),
		zap.String("job_id", logpkg.SanitizeUserID(job.ID.String())),
		zap.Int("attempt", job.RetryCount),
		zap.Int("max_retries", job.MaxRetries),
		zap.String("error", logpkg.SanitizeError(err)),
	)
	a.nackOrLog(msg, true, job.ID.String())
	return fmt.Errorf("job failed (will retry): %w", err)
}

func (a *TaskAnalyzer) sendToDLQ(msg queue.MessageInterface, job *queue.Job, err error, jobType string) error {
	a.logger.Error("job_failed_max_retries_exceeded",
		zap.String("operation", "handle_job_error"),
		zap.String("job_type", jobType),
		zap.String("job_id", logpkg.SanitizeUserID(job.ID.String())),
		zap.Int("max_retries", job.MaxRetries),
		zap.Int("retry_count", job.RetryCount),
		zap.String("error", logpkg.SanitizeError(err)),
	)
	a.nackOrLog(msg, false, job.ID.String())
	return fmt.Errorf("job failed (max retries): %w", err)
}

// handleJobError handles errors from job processing with intelligent retry logic.
func (a *TaskAnalyzer) handleJobError(ctx context.Context, msg queue.MessageInterface, job *queue.Job, err error, jobType string) error {
	if ai.IsQuotaError(err) {
		return a.handleQuotaError(ctx, msg, job, err, jobType)
	}
	if ai.IsRateLimitError(err) {
		return a.handleRateLimitError(ctx, msg, job, err, jobType)
	}
	if job.CanRetry() {
		return a.handleGenericRetry(msg, job, err, jobType)
	}
	return a.sendToDLQ(msg, job, err, jobType)
}
