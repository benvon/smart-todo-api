package workers

import (
	"context"
	"testing"

	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/queue"
	"github.com/google/uuid"
)

// TestTagAnalyzer_ProcessTagAnalysisJob_LoadsAllPages tests that the analyzer
// loads all todos across multiple pages
func TestTagAnalyzer_ProcessTagAnalysisJob_LoadsAllPages(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	
	// Create 750 todos (more than one page of 500)
	allTodos := make([]*models.Todo, 750)
	for i := 0; i < 750; i++ {
		allTodos[i] = &models.Todo{
			ID:     uuid.New(),
			UserID: userID,
			Text:   "Test todo",
			Status: models.TodoStatusProcessed,
			Metadata: models.Metadata{
				CategoryTags: []string{"test"},
				TagSources: map[string]models.TagSource{
					"test": models.TagSourceAI,
				},
			},
		}
	}

	callCount := 0
	mockTodoRepo := &mockTodoRepo{
		t: t,
		getByUserIDPaginatedFunc: func(ctx context.Context, uid uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error) {
			callCount++
			offset := (page - 1) * pageSize
			end := offset + pageSize
			if end > len(allTodos) {
				end = len(allTodos)
			}
			
			// Return the appropriate page
			pageTodos := allTodos[offset:end]
			return pageTodos, len(allTodos), nil
		},
	}

	stats := &models.TagStatistics{
		UserID:          userID,
		TagStats:        make(map[string]models.TagStats),
		Tainted:         true,
		AnalysisVersion: 0,
	}

	mockTagStatsRepo := &mockTagStatisticsRepoForWorker{
		t: t,
		getByUserIDOrCreateFunc: func(ctx context.Context, uid uuid.UUID) (*models.TagStatistics, error) {
			return stats, nil
		},
		updateStatisticsFunc: func(ctx context.Context, s *models.TagStatistics) (bool, error) {
			// Should have counted ALL 750 todos
			if s.TagStats["test"].Total != 750 {
				t.Errorf("Expected test tag total=750 (all todos), got %d", s.TagStats["test"].Total)
			}
			return true, nil
		},
	}

	analyzer := NewTagAnalyzer(mockTodoRepo, mockTagStatsRepo)

	job := &queue.Job{
		ID:     uuid.New(),
		Type:   queue.JobTypeTagAnalysis,
		UserID: userID,
	}

	msg := &mockMessage{job: job}

	err := analyzer.ProcessJob(context.Background(), msg)
	if err != nil {
		t.Fatalf("ProcessJob failed: %v", err)
	}

	// Verify we made multiple pagination calls
	if callCount < 2 {
		t.Errorf("Expected at least 2 pagination calls (750 todos / 500 per page = 2 pages), got %d", callCount)
	}
}

// TestTagAnalyzer_ProcessTagAnalysisJob_IncludesCompletedTodosInStats tests that
// completed todos are included in tag statistics
func TestTagAnalyzer_ProcessTagAnalysisJob_IncludesCompletedTodosInStats(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	todos := []*models.Todo{
		{
			ID:     uuid.New(),
			UserID: userID,
			Text:   "Completed todo 1",
			Status: models.TodoStatusCompleted,
			Metadata: models.Metadata{
				CategoryTags: []string{"work"},
				TagSources: map[string]models.TagSource{
					"work": models.TagSourceAI,
				},
			},
		},
		{
			ID:     uuid.New(),
			UserID: userID,
			Text:   "Completed todo 2",
			Status: models.TodoStatusCompleted,
			Metadata: models.Metadata{
				CategoryTags: []string{"work", "urgent"},
				TagSources: map[string]models.TagSource{
					"work":   models.TagSourceAI,
					"urgent": models.TagSourceUser,
				},
			},
		},
		{
			ID:     uuid.New(),
			UserID: userID,
			Text:   "Active todo",
			Status: models.TodoStatusProcessed,
			Metadata: models.Metadata{
				CategoryTags: []string{"work"},
				TagSources: map[string]models.TagSource{
					"work": models.TagSourceAI,
				},
			},
		},
	}

	callCount := 0
	mockTodoRepo := &mockTodoRepo{
		t: t,
		getByUserIDPaginatedFunc: func(ctx context.Context, uid uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error) {
			callCount++
			// Return all todos on first call
			if callCount == 1 {
				return todos, len(todos), nil
			}
			// Subsequent calls return empty
			return []*models.Todo{}, len(todos), nil
		},
	}

	stats := &models.TagStatistics{
		UserID:          userID,
		TagStats:        make(map[string]models.TagStats),
		Tainted:         true,
		AnalysisVersion: 0,
	}

	mockTagStatsRepo := &mockTagStatisticsRepoForWorker{
		t: t,
		getByUserIDOrCreateFunc: func(ctx context.Context, uid uuid.UUID) (*models.TagStatistics, error) {
			return stats, nil
		},
		updateStatisticsFunc: func(ctx context.Context, s *models.TagStatistics) (bool, error) {
			// Should count ALL todos including completed ones
			// "work" appears in all 3 todos (2 completed + 1 active)
			if s.TagStats["work"].Total != 3 {
				t.Errorf("Expected work tag total=3 (all todos including completed), got %d", s.TagStats["work"].Total)
			}
			// "urgent" appears in 1 completed todo
			if s.TagStats["urgent"].Total != 1 {
				t.Errorf("Expected urgent tag total=1 (from completed todo), got %d", s.TagStats["urgent"].Total)
			}
			if s.TagStats["urgent"].User != 1 {
				t.Errorf("Expected urgent tag User=1, got %d", s.TagStats["urgent"].User)
			}
			return true, nil
		},
	}

	analyzer := NewTagAnalyzer(mockTodoRepo, mockTagStatsRepo)

	job := &queue.Job{
		ID:     uuid.New(),
		Type:   queue.JobTypeTagAnalysis,
		UserID: userID,
	}

	msg := &mockMessage{job: job}

	err := analyzer.ProcessJob(context.Background(), msg)
	if err != nil {
		t.Fatalf("ProcessJob failed: %v", err)
	}
}
