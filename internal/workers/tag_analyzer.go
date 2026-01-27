package workers

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/queue"
)

// TagAnalyzer processes tag analysis jobs to aggregate tag statistics
type TagAnalyzer struct {
	todoRepo        database.TodoRepositoryInterface
	tagStatsRepo    database.TagStatisticsRepositoryInterface
}

// NewTagAnalyzer creates a new tag analyzer
func NewTagAnalyzer(
	todoRepo database.TodoRepositoryInterface,
	tagStatsRepo database.TagStatisticsRepositoryInterface,
) *TagAnalyzer {
	return &TagAnalyzer{
		todoRepo:     todoRepo,
		tagStatsRepo: tagStatsRepo,
	}
}

// ProcessTagAnalysisJob processes a tag analysis job
func (a *TagAnalyzer) ProcessTagAnalysisJob(ctx context.Context, job *queue.Job) error {
	if job.UserID == (queue.Job{}.UserID) {
		return fmt.Errorf("user_id is required for tag analysis job")
	}

	log.Printf("Processing tag analysis job %s for user %s", job.ID, job.UserID)

	// Get or create tag statistics record
	stats, err := a.tagStatsRepo.GetByUserIDOrCreate(ctx, job.UserID)
	if err != nil {
		return fmt.Errorf("failed to get or create tag statistics: %w", err)
	}

	log.Printf("Tag statistics for user %s: tainted=%v, existing tags=%d", job.UserID, stats.Tainted, len(stats.TagStats))

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
		todos, total, err := a.todoRepo.GetByUserIDPaginated(ctx, job.UserID, nil, nil, page, pageSize)
		if err != nil {
			return fmt.Errorf("failed to get todos: %w", err)
		}
		
		allTodos = append(allTodos, todos...)
		
		// Check if we've loaded all todos
		// If this page returned fewer todos than pageSize, we're done
		// Or if we've loaded all todos according to total count
		if len(todos) < pageSize || len(allTodos) >= total {
			break
		}
		
		// Move to next page
		page++
	}

	log.Printf("Loaded %d todos for user %s (across %d pages)", len(allTodos), job.UserID, page)

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

	log.Printf("Found %d todos with tags (%d completed), aggregated %d unique tags for user %s", todosWithTags, completedTodosWithTags, len(tagStatsMap), job.UserID)

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
		log.Printf("Tag statistics for user %s was updated by another worker (version conflict), skipping save", job.UserID)
		return nil
	}

	log.Printf("Successfully analyzed tags for user %s: %d unique tags", job.UserID, len(tagStatsMap))
	if len(tagStatsMap) > 0 {
		log.Printf("Tag breakdown for user %s: %+v", job.UserID, tagStatsMap)
	}
	return nil
}

// ProcessJob processes a job based on its type
func (a *TagAnalyzer) ProcessJob(ctx context.Context, msg queue.MessageInterface) error {
	job := msg.GetJob()

	// Check if job should be processed now (respect NotBefore)
	if !job.ShouldProcess() {
		log.Printf("Tag analysis job %s not ready yet (NotBefore: %v), skipping", job.ID, job.NotBefore)
		// Re-ack to return to queue and wait
		if ackErr := msg.Ack(); ackErr != nil {
			log.Printf("Failed to ack job for later processing: %v", ackErr)
		}
		return nil
	}

	switch job.Type {
	case queue.JobTypeTagAnalysis:
		if err := a.ProcessTagAnalysisJob(ctx, job); err != nil {
			// For tag analysis errors, log and nack without requeue
			// Tag analysis can be retried later if needed
			log.Printf("Tag analysis job %s failed: %v", job.ID, err)
			if nackErr := msg.Nack(false); nackErr != nil {
				log.Printf("Failed to nack tag analysis job: %v", nackErr)
			}
			return fmt.Errorf("tag analysis failed: %w", err)
		}
		if ackErr := msg.Ack(); ackErr != nil {
			return fmt.Errorf("failed to ack tag analysis job: %w", ackErr)
		}
		return nil

	default:
		if nackErr := msg.Nack(false); nackErr != nil {
			log.Printf("Failed to nack unknown job type: %v", nackErr)
		}
		return fmt.Errorf("unknown job type: %s", job.Type)
	}
}
