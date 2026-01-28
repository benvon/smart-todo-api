package workers

import (
	"context"
	"fmt"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/queue"
	"go.uber.org/zap"
)

// TagAnalyzer processes tag analysis jobs to aggregate tag statistics
type TagAnalyzer struct {
	todoRepo     database.TodoRepositoryInterface
	tagStatsRepo database.TagStatisticsRepositoryInterface
	logger       *zap.Logger
}

// NewTagAnalyzer creates a new tag analyzer
func NewTagAnalyzer(
	todoRepo database.TodoRepositoryInterface,
	tagStatsRepo database.TagStatisticsRepositoryInterface,
	logger *zap.Logger,
) *TagAnalyzer {
	return &TagAnalyzer{
		todoRepo:     todoRepo,
		tagStatsRepo: tagStatsRepo,
		logger:       logger,
	}
}

// ProcessTagAnalysisJob processes a tag analysis job
func (a *TagAnalyzer) ProcessTagAnalysisJob(ctx context.Context, job *queue.Job) error {
	if job.UserID == (queue.Job{}.UserID) {
		return fmt.Errorf("user_id is required for tag analysis job")
	}

	a.logger.Info("processing_tag_analysis_job",
		zap.String("job_id", job.ID.String()),
		zap.String("user_id", job.UserID.String()),
	)

	// Get or create tag statistics record
	stats, err := a.tagStatsRepo.GetByUserIDOrCreate(ctx, job.UserID)
	if err != nil {
		return fmt.Errorf("failed to get or create tag statistics: %w", err)
	}

	a.logger.Debug("tag_statistics_status",
		zap.String("user_id", job.UserID.String()),
		zap.Bool("tainted", stats.Tainted),
		zap.Int("existing_tags", len(stats.TagStats)),
	)

	// Always process tag analysis jobs when they're queued
	// We re-analyze all todos to ensure tag statistics are up-to-date
	// The tainted flag tracks whether stats need updating, but we process regardless
	// Multiple jobs are fine - they'll be debounced and the last one will have the final state

	// Load ALL todos for user (including completed ones for statistics)
	// We need to paginate through all pages to get every todo
	var allTodos []*models.Todo
	page := 1
	pageSize := 500

	for {
		todos, _, err := a.todoRepo.GetByUserIDPaginated(ctx, job.UserID, nil, nil, page, pageSize)
		if err != nil {
			return fmt.Errorf("failed to get todos: %w", err)
		}

		allTodos = append(allTodos, todos...)

		// Check if we've loaded all todos
		// If this page returned no todos or fewer than pageSize, we're done
		if len(todos) == 0 || len(todos) < pageSize {
			break
		}

		// Move to next page
		page++
	}

	a.logger.Debug("loaded_todos_for_tag_analysis",
		zap.String("user_id", job.UserID.String()),
		zap.Int("total_todos", len(allTodos)),
		zap.Int("pages", page),
	)

	// Aggregate tags from all todos (including completed ones)
	tagStatsMap := make(map[string]models.TagStats)

	todosWithTags := 0
	completedTodosWithTags := 0
	for _, todo := range allTodos {
		// Process each tag in the todo's metadata
		// Include ALL todos (both open and completed) in tag statistics
		if len(todo.Metadata.CategoryTags) > 0 {
			todosWithTags++
			if todo.Status == models.TodoStatusCompleted {
				completedTodosWithTags++
			}

			for _, tag := range todo.Metadata.CategoryTags {
				// Initialize tag stats if not exists
				if _, exists := tagStatsMap[tag]; !exists {
					tagStatsMap[tag] = models.TagStats{}
				}

				// Get current stats
				currentStats := tagStatsMap[tag]
				currentStats.Total++

				// Determine source (default to AI if not specified)
				source := models.TagSourceAI
				if todo.Metadata.TagSources != nil {
					if tagSource, ok := todo.Metadata.TagSources[tag]; ok {
						source = tagSource
					}
				}

				// Increment appropriate counter
				switch source {
				case models.TagSourceAI:
					currentStats.AI++
				case models.TagSourceUser:
					currentStats.User++
				}

				tagStatsMap[tag] = currentStats
			}
		}
	}

	a.logger.Info("aggregated_tag_statistics",
		zap.String("user_id", job.UserID.String()),
		zap.Int("todos_with_tags", todosWithTags),
		zap.Int("completed_todos_with_tags", completedTodosWithTags),
		zap.Int("unique_tags", len(tagStatsMap)),
	)

	// Update statistics with aggregated data
	stats.TagStats = tagStatsMap
	now := time.Now()
	stats.LastAnalyzedAt = &now

	// Atomically update statistics with version check
	updated, err := a.tagStatsRepo.UpdateStatistics(ctx, stats)
	if err != nil {
		return fmt.Errorf("failed to update tag statistics: %w", err)
	}

	if !updated {
		// Version conflict - another worker updated the statistics
		// This is expected in concurrent scenarios, log and return success
		a.logger.Debug("tag_statistics_version_conflict",
			zap.String("user_id", job.UserID.String()),
		)
		return nil
	}

	a.logger.Info("successfully_analyzed_tags",
		zap.String("user_id", job.UserID.String()),
		zap.Int("unique_tags", len(tagStatsMap)),
	)
	if len(tagStatsMap) > 0 && a.logger.Core().Enabled(zap.DebugLevel) {
		// Only log tag breakdown in debug mode (can be verbose)
		tagList := make([]string, 0, len(tagStatsMap))
		for tag := range tagStatsMap {
			tagList = append(tagList, tag)
		}
		a.logger.Debug("tag_breakdown",
			zap.String("user_id", job.UserID.String()),
			zap.Strings("tags", tagList),
		)
	}
	return nil
}

// ProcessJob processes a job based on its type
func (a *TagAnalyzer) ProcessJob(ctx context.Context, msg queue.MessageInterface) error {
	job := msg.GetJob()

	// Check if job should be processed now (respect NotBefore)
	if !job.ShouldProcess() {
		a.logger.Debug("tag_analysis_job_not_ready",
			zap.String("job_id", job.ID.String()),
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
	case queue.JobTypeTagAnalysis:
		if err := a.ProcessTagAnalysisJob(ctx, job); err != nil {
			// For tag analysis errors, log and nack without requeue
			// Tag analysis can be retried later if needed
			a.logger.Error("tag_analysis_job_failed",
				zap.String("job_id", job.ID.String()),
				zap.String("user_id", job.UserID.String()),
				zap.Error(err),
			)
			if nackErr := msg.Nack(false); nackErr != nil {
				a.logger.Warn("failed_to_nack_tag_analysis_job",
					zap.String("job_id", job.ID.String()),
					zap.Error(nackErr),
				)
			}
			return fmt.Errorf("tag analysis failed: %w", err)
		}
		if ackErr := msg.Ack(); ackErr != nil {
			return fmt.Errorf("failed to ack tag analysis job: %w", ackErr)
		}
		return nil

	default:
		if nackErr := msg.Nack(false); nackErr != nil {
			a.logger.Error("failed_to_nack_unknown_job_type",
				zap.String("job_id", job.ID.String()),
				zap.String("job_type", string(job.Type)),
				zap.Error(nackErr),
			)
		}
		return fmt.Errorf("unknown job type: %s", job.Type)
	}
}
