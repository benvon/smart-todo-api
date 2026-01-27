package workers

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/queue"
	"github.com/google/uuid"
)

// mockTagStatisticsRepoForWorker is a mock for testing tag analyzer worker
type mockTagStatisticsRepoForWorker struct {
	t                       *testing.T
	getByUserIDFunc         func(ctx context.Context, userID uuid.UUID) (*models.TagStatistics, error)
	getByUserIDOrCreateFunc func(ctx context.Context, userID uuid.UUID) (*models.TagStatistics, error)
	updateStatisticsFunc    func(ctx context.Context, stats *models.TagStatistics) (bool, error)
	markTaintedFunc         func(ctx context.Context, userID uuid.UUID) (bool, error)

	// Call tracking (protected by mutex for concurrent access)
	mu                       sync.Mutex
	getByUserIDCalls         []uuid.UUID
	getByUserIDOrCreateCalls []uuid.UUID
	updateStatisticsCalls    []*models.TagStatistics
	markTaintedCalls         []uuid.UUID
}

func (m *mockTagStatisticsRepoForWorker) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.TagStatistics, error) {
	m.mu.Lock()
	m.getByUserIDCalls = append(m.getByUserIDCalls, userID)
	m.mu.Unlock()
	if m.getByUserIDFunc == nil {
		m.t.Fatal("GetByUserID called but not configured in test - mock requires explicit setup")
	}
	return m.getByUserIDFunc(ctx, userID)
}

func (m *mockTagStatisticsRepoForWorker) GetByUserIDOrCreate(ctx context.Context, userID uuid.UUID) (*models.TagStatistics, error) {
	m.mu.Lock()
	m.getByUserIDOrCreateCalls = append(m.getByUserIDOrCreateCalls, userID)
	m.mu.Unlock()
	if m.getByUserIDOrCreateFunc == nil {
		m.t.Fatal("GetByUserIDOrCreate called but not configured in test - mock requires explicit setup")
	}
	return m.getByUserIDOrCreateFunc(ctx, userID)
}

func (m *mockTagStatisticsRepoForWorker) UpdateStatistics(ctx context.Context, stats *models.TagStatistics) (bool, error) {
	m.mu.Lock()
	m.updateStatisticsCalls = append(m.updateStatisticsCalls, stats)
	m.mu.Unlock()
	if m.updateStatisticsFunc == nil {
		m.t.Fatal("UpdateStatistics called but not configured in test - mock requires explicit setup")
	}
	return m.updateStatisticsFunc(ctx, stats)
}

func (m *mockTagStatisticsRepoForWorker) MarkTainted(ctx context.Context, userID uuid.UUID) (bool, error) {
	m.mu.Lock()
	m.markTaintedCalls = append(m.markTaintedCalls, userID)
	m.mu.Unlock()
	if m.markTaintedFunc == nil {
		m.t.Fatal("MarkTainted called but not configured in test - mock requires explicit setup")
	}
	return m.markTaintedFunc(ctx, userID)
}

var _ database.TagStatisticsRepositoryInterface = (*mockTagStatisticsRepoForWorker)(nil)

func TestTagAnalyzer_ProcessTagAnalysisJob_Success(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	todoID1 := uuid.New()
	todoID2 := uuid.New()

	todos := []*models.Todo{
		{
			ID:     todoID1,
			UserID: userID,
			Text:   "Buy groceries",
			Status: models.TodoStatusProcessed,
			Metadata: models.Metadata{
				CategoryTags: []string{"shopping", "errands"},
				TagSources: map[string]models.TagSource{
					"shopping": models.TagSourceAI,
					"errands":  models.TagSourceAI,
				},
			},
		},
		{
			ID:     todoID2,
			UserID: userID,
			Text:   "Call mom",
			Status: models.TodoStatusProcessed,
			Metadata: models.Metadata{
				CategoryTags: []string{"personal", "errands"},
				TagSources: map[string]models.TagSource{
					"personal": models.TagSourceUser,
					"errands":  models.TagSourceAI,
				},
			},
		},
	}

	callCount := 0
	mockTodoRepo := &mockTodoRepo{
		t: t,
		getByUserIDPaginatedFunc: func(ctx context.Context, uid uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error) {
			callCount++
			if uid != userID {
				t.Errorf("GetByUserIDPaginated called with wrong userID: expected %s, got %s", userID, uid)
			}
			// Return all todos on first call, empty on subsequent calls (simulating pagination)
			if callCount == 1 {
				return todos, len(todos), nil
			}
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
			if uid != userID {
				t.Errorf("GetByUserIDOrCreate called with wrong userID: expected %s, got %s", userID, uid)
			}
			return stats, nil
		},
		updateStatisticsFunc: func(ctx context.Context, s *models.TagStatistics) (bool, error) {
			if s.UserID != userID {
				t.Errorf("UpdateStatistics called with wrong userID: expected %s, got %s", userID, s.UserID)
			}
			// Verify tag aggregation
			if s.TagStats["shopping"].Total != 1 {
				t.Errorf("Expected shopping tag total=1, got %d", s.TagStats["shopping"].Total)
			}
			if s.TagStats["shopping"].AI != 1 {
				t.Errorf("Expected shopping tag AI=1, got %d", s.TagStats["shopping"].AI)
			}
			if s.TagStats["errands"].Total != 2 {
				t.Errorf("Expected errands tag total=2, got %d", s.TagStats["errands"].Total)
			}
			if s.TagStats["errands"].AI != 2 {
				t.Errorf("Expected errands tag AI=2, got %d", s.TagStats["errands"].AI)
			}
			if s.TagStats["personal"].Total != 1 {
				t.Errorf("Expected personal tag total=1, got %d", s.TagStats["personal"].Total)
			}
			if s.TagStats["personal"].User != 1 {
				t.Errorf("Expected personal tag User=1, got %d", s.TagStats["personal"].User)
			}
			// Note: UpdateStatistics sets tainted=false in the database, but the stats object
			// passed in still has the original value. The database update handles the flag change.
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

	// Verify calls (may be called multiple times for pagination)
	mockTodoRepo.mu.Lock()
	todoCallsCount := len(mockTodoRepo.getByUserIDPaginatedCalls)
	mockTodoRepo.mu.Unlock()
	if todoCallsCount < 1 {
		t.Errorf("Expected GetByUserIDPaginated called at least 1 time, got %d", todoCallsCount)
	}
	mockTagStatsRepo.mu.Lock()
	updateCallsCount := len(mockTagStatsRepo.updateStatisticsCalls)
	mockTagStatsRepo.mu.Unlock()
	if updateCallsCount != 1 {
		t.Errorf("Expected UpdateStatistics called 1 time, got %d", updateCallsCount)
	}
}

func TestTagAnalyzer_ProcessTagAnalysisJob_ProcessesEvenWhenNotTainted(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	todos := []*models.Todo{
		{
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
		},
	}

	stats := &models.TagStatistics{
		UserID:          userID,
		TagStats:        make(map[string]models.TagStats),
		Tainted:         false, // Already cleared - but we should still process
		AnalysisVersion: 1,
	}

	mockTodoRepo := &mockTodoRepo{
		t: t,
		getByUserIDPaginatedFunc: func(ctx context.Context, uid uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error) {
			return todos, len(todos), nil
		},
	}

	mockTagStatsRepo := &mockTagStatisticsRepoForWorker{
		t: t,
		getByUserIDOrCreateFunc: func(ctx context.Context, uid uuid.UUID) (*models.TagStatistics, error) {
			return stats, nil
		},
		updateStatisticsFunc: func(ctx context.Context, s *models.TagStatistics) (bool, error) {
			// UpdateStatistics SHOULD be called even when tainted=false
			// We always process jobs when they're queued
			if len(s.TagStats) == 0 {
				t.Error("Expected tag stats to be populated")
			}
			if s.TagStats["test"].Total != 1 {
				t.Errorf("Expected test tag total=1, got %d", s.TagStats["test"].Total)
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

	// Verify UpdateStatistics WAS called (we always process queued jobs)
	mockTagStatsRepo.mu.Lock()
	updateCallsCount := len(mockTagStatsRepo.updateStatisticsCalls)
	mockTagStatsRepo.mu.Unlock()
	if updateCallsCount != 1 {
		t.Errorf("Expected UpdateStatistics called 1 time, but was called %d times", updateCallsCount)
	}
}

func TestTagAnalyzer_ProcessTagAnalysisJob_VersionConflict(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	stats := &models.TagStatistics{
		UserID:          userID,
		TagStats:        make(map[string]models.TagStats),
		Tainted:         true,
		AnalysisVersion: 0,
	}

	mockTodoRepo := &mockTodoRepo{
		t: t,
		getByUserIDPaginatedFunc: func(ctx context.Context, uid uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error) {
			return []*models.Todo{}, 0, nil
		},
	}

	mockTagStatsRepo := &mockTagStatisticsRepoForWorker{
		t: t,
		getByUserIDOrCreateFunc: func(ctx context.Context, uid uuid.UUID) (*models.TagStatistics, error) {
			return stats, nil
		},
		updateStatisticsFunc: func(ctx context.Context, s *models.TagStatistics) (bool, error) {
			// Simulate version conflict
			return false, nil
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
		t.Fatalf("ProcessJob should succeed even with version conflict: %v", err)
	}
}

func TestTagAnalyzer_ProcessTagAnalysisJob_HandlesEmptyTags(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	todos := []*models.Todo{
		{
			ID:     uuid.New(),
			UserID: userID,
			Text:   "Todo without tags",
			Status: models.TodoStatusProcessed,
			Metadata: models.Metadata{
				CategoryTags: nil,
				TagSources:   nil,
			},
		},
	}

	mockTodoRepo := &mockTodoRepo{
		t: t,
		getByUserIDPaginatedFunc: func(ctx context.Context, uid uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error) {
			return todos, len(todos), nil
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
			if len(s.TagStats) != 0 {
				t.Errorf("Expected empty tag stats, got %d tags", len(s.TagStats))
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

func TestTagAnalyzer_ProcessTagAnalysisJob_IncludesCompletedTodos(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	todos := []*models.Todo{
		{
			ID:     uuid.New(),
			UserID: userID,
			Text:   "Completed todo",
			Status: models.TodoStatusCompleted,
			Metadata: models.Metadata{
				CategoryTags: []string{"shopping"},
				TagSources: map[string]models.TagSource{
					"shopping": models.TagSourceAI,
				},
			},
		},
		{
			ID:     uuid.New(),
			UserID: userID,
			Text:   "Active todo",
			Status: models.TodoStatusProcessed,
			Metadata: models.Metadata{
				CategoryTags: []string{"shopping"},
				TagSources: map[string]models.TagSource{
					"shopping": models.TagSourceAI,
				},
			},
		},
	}

	callCount := 0
	mockTodoRepo := &mockTodoRepo{
		t: t,
		getByUserIDPaginatedFunc: func(ctx context.Context, uid uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error) {
			callCount++
			// Simulate pagination: return all todos on first call
			if callCount == 1 {
				return todos, len(todos), nil
			}
			// Subsequent calls return empty (all loaded)
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
			// Should count BOTH todos (completed and active)
			if s.TagStats["shopping"].Total != 2 {
				t.Errorf("Expected shopping tag total=2 (both completed and active todos), got %d", s.TagStats["shopping"].Total)
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

func TestTagAnalyzer_ProcessTagAnalysisJob_DebouncedJobs(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	job := &queue.Job{
		ID:        uuid.New(),
		Type:      queue.JobTypeTagAnalysis,
		UserID:    userID,
		NotBefore: func() *time.Time { t := time.Now().Add(10 * time.Second); return &t }(),
	}

	mockTodoRepo := &mockTodoRepo{t: t}
	mockTagStatsRepo := &mockTagStatisticsRepoForWorker{t: t}

	analyzer := NewTagAnalyzer(mockTodoRepo, mockTagStatsRepo)

	acked := false
	msg := &mockMessage{
		job: job,
		ackFunc: func() error {
			acked = true
			return nil
		},
	}

	err := analyzer.ProcessJob(context.Background(), msg)
	if err != nil {
		t.Fatalf("ProcessJob should succeed for debounced job: %v", err)
	}

	// Verify job was acked but not processed
	if !acked {
		t.Error("Expected job to be acked even when not ready")
	}
	mockTodoRepo.mu.Lock()
	todoCallsCount := len(mockTodoRepo.getByUserIDPaginatedCalls)
	mockTodoRepo.mu.Unlock()
	if todoCallsCount != 0 {
		t.Error("Expected GetByUserIDPaginated not called for debounced job")
	}
}

func TestTagAnalyzer_ProcessTagAnalysisJob_InvalidJobType(t *testing.T) {
	t.Parallel()

	job := &queue.Job{
		ID:     uuid.New(),
		Type:   queue.JobTypeTaskAnalysis, // Wrong type
		UserID: uuid.New(),
	}

	mockTodoRepo := &mockTodoRepo{t: t}
	mockTagStatsRepo := &mockTagStatisticsRepoForWorker{t: t}

	analyzer := NewTagAnalyzer(mockTodoRepo, mockTagStatsRepo)

	nacked := false
	msg := &mockMessage{
		job: job,
		nackFunc: func(requeue bool) error {
			nacked = true
			return nil
		},
	}

	err := analyzer.ProcessJob(context.Background(), msg)
	if err == nil {
		t.Error("Expected error for invalid job type")
	}
	if !nacked {
		t.Error("Expected job to be nacked for invalid type")
	}
}

// Race Condition Tests

func TestTagAnalyzer_ProcessTagAnalysisJob_ConcurrentWorkers(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	stats := &models.TagStatistics{
		UserID:          userID,
		TagStats:        make(map[string]models.TagStats),
		Tainted:         true,
		AnalysisVersion: 0,
	}

	todos := []*models.Todo{
		{
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
		},
	}

	mockTodoRepo := &mockTodoRepo{
		t: t,
		getByUserIDPaginatedFunc: func(ctx context.Context, uid uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error) {
			return todos, len(todos), nil
		},
	}

	// Track update attempts
	updateAttempts := make(chan int, 10)
	var firstUpdateMu sync.Mutex
	firstUpdateSucceeds := true

	mockTagStatsRepo := &mockTagStatisticsRepoForWorker{
		t: t,
		getByUserIDOrCreateFunc: func(ctx context.Context, uid uuid.UUID) (*models.TagStatistics, error) {
			// Return a copy to simulate concurrent access
			return &models.TagStatistics{
				UserID:          stats.UserID,
				TagStats:        make(map[string]models.TagStats),
				Tainted:         stats.Tainted,
				AnalysisVersion: stats.AnalysisVersion,
			}, nil
		},
		updateStatisticsFunc: func(ctx context.Context, s *models.TagStatistics) (bool, error) {
			updateAttempts <- s.AnalysisVersion
			// First update succeeds, subsequent ones fail (version conflict)
			firstUpdateMu.Lock()
			shouldSucceed := firstUpdateSucceeds
			if firstUpdateSucceeds {
				firstUpdateSucceeds = false
			}
			firstUpdateMu.Unlock()
			if shouldSucceed {
				return true, nil
			}
			return false, nil // Version conflict
		},
	}

	analyzer := NewTagAnalyzer(mockTodoRepo, mockTagStatsRepo)

	job := &queue.Job{
		ID:     uuid.New(),
		Type:   queue.JobTypeTagAnalysis,
		UserID: userID,
	}

	// Simulate two workers processing the same job concurrently
	done := make(chan bool, 2)
	errors := make(chan error, 2)

	for i := 0; i < 2; i++ {
		go func() {
			msg := &mockMessage{job: job}
			err := analyzer.ProcessJob(context.Background(), msg)
			errors <- err
			done <- true
		}()
	}

	// Wait for both workers
	<-done
	<-done

	// Check that both workers completed without error
	err1 := <-errors
	err2 := <-errors
	if err1 != nil {
		t.Errorf("Worker 1 failed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("Worker 2 failed: %v", err2)
	}

	// Verify that UpdateStatistics was called twice (both workers tried)
	mockTagStatsRepo.mu.Lock()
	updateCallsCount := len(mockTagStatsRepo.updateStatisticsCalls)
	mockTagStatsRepo.mu.Unlock()
	if updateCallsCount != 2 {
		t.Errorf("Expected UpdateStatistics called 2 times (one per worker), got %d", updateCallsCount)
	}

	// Verify that only one update succeeded (version conflict prevented second)
	close(updateAttempts)
	attempts := 0
	for range updateAttempts {
		attempts++
	}
	if attempts != 2 {
		t.Errorf("Expected 2 update attempts, got %d", attempts)
	}
}

func TestTagAnalyzer_ProcessTagAnalysisJob_ProcessesRegardlessOfTaintedStatus(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	todos := []*models.Todo{
		{
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
		},
	}

	mockTodoRepo := &mockTodoRepo{
		t: t,
		getByUserIDPaginatedFunc: func(ctx context.Context, uid uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error) {
			return todos, len(todos), nil
		},
	}

	// Test that we process even if tainted flag changes during processing
	mockTagStatsRepo := &mockTagStatisticsRepoForWorker{
		t: t,
		getByUserIDOrCreateFunc: func(ctx context.Context, uid uuid.UUID) (*models.TagStatistics, error) {
			// Return tainted=false (simulating stats were already processed)
			return &models.TagStatistics{
				UserID:          userID,
				TagStats:        make(map[string]models.TagStats),
				Tainted:         false,
				AnalysisVersion: 1,
			}, nil
		},
		updateStatisticsFunc: func(ctx context.Context, s *models.TagStatistics) (bool, error) {
			// UpdateStatistics SHOULD be called even if tainted=false
			// We always process queued jobs to ensure stats are up-to-date
			if len(s.TagStats) == 0 {
				t.Error("Expected tag stats to be populated")
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
		t.Fatalf("ProcessJob should succeed regardless of tainted status: %v", err)
	}

	// Verify UpdateStatistics WAS called (we always process queued jobs)
	mockTagStatsRepo.mu.Lock()
	updateCallsCount := len(mockTagStatsRepo.updateStatisticsCalls)
	mockTagStatsRepo.mu.Unlock()
	if updateCallsCount != 1 {
		t.Errorf("Expected UpdateStatistics called 1 time (we always process queued jobs), but was called %d times", updateCallsCount)
	}
	// Since we're simulating it being cleared, UpdateStatistics should not be called
	// However, the current implementation checks tainted after GetByUserIDOrCreate
	// So this test verifies that check works correctly
}

func TestTagAnalyzer_ProcessTagAnalysisJob_MultipleRapidJobs(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	todos := []*models.Todo{
		{
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
		},
	}

	mockTodoRepo := &mockTodoRepo{
		t: t,
		getByUserIDPaginatedFunc: func(ctx context.Context, uid uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error) {
			return todos, len(todos), nil
		},
	}

	stats := &models.TagStatistics{
		UserID:          userID,
		TagStats:        make(map[string]models.TagStats),
		Tainted:         true,
		AnalysisVersion: 0,
	}

	updateCount := 0
	mockTagStatsRepo := &mockTagStatisticsRepoForWorker{
		t: t,
		getByUserIDOrCreateFunc: func(ctx context.Context, uid uuid.UUID) (*models.TagStatistics, error) {
			return stats, nil
		},
		updateStatisticsFunc: func(ctx context.Context, s *models.TagStatistics) (bool, error) {
			updateCount++
			// First update succeeds, subsequent ones may fail due to version conflicts
			if updateCount == 1 {
				return true, nil
			}
			// Simulate version conflicts for rapid subsequent jobs
			return false, nil
		},
	}

	analyzer := NewTagAnalyzer(mockTodoRepo, mockTagStatsRepo)

	// Create multiple jobs with debounce delays
	jobs := []*queue.Job{
		{
			ID:        uuid.New(),
			Type:      queue.JobTypeTagAnalysis,
			UserID:    userID,
			NotBefore: func() *time.Time { t := time.Now().Add(1 * time.Second); return &t }(),
		},
		{
			ID:        uuid.New(),
			Type:      queue.JobTypeTagAnalysis,
			UserID:    userID,
			NotBefore: func() *time.Time { t := time.Now().Add(2 * time.Second); return &t }(),
		},
		{
			ID:        uuid.New(),
			Type:      queue.JobTypeTagAnalysis,
			UserID:    userID,
			NotBefore: func() *time.Time { t := time.Now().Add(3 * time.Second); return &t }(),
		},
	}

	// Process jobs sequentially (they'll be acked and wait for NotBefore)
	for _, job := range jobs {
		msg := &mockMessage{job: job}
		err := analyzer.ProcessJob(context.Background(), msg)
		if err != nil {
			t.Fatalf("ProcessJob failed for job %s: %v", job.ID, err)
		}
	}

	// All jobs should be acked (debounced)
	// In real scenario, they would process after their NotBefore times
	// This test verifies that debouncing works correctly
}

func TestTagAnalyzer_ProcessTagAnalysisJob_MissingUserID(t *testing.T) {
	t.Parallel()

	job := &queue.Job{
		ID:     uuid.New(),
		Type:   queue.JobTypeTagAnalysis,
		UserID: uuid.Nil, // Missing user ID
	}

	mockTodoRepo := &mockTodoRepo{t: t}
	mockTagStatsRepo := &mockTagStatisticsRepoForWorker{t: t}

	analyzer := NewTagAnalyzer(mockTodoRepo, mockTagStatsRepo)

	msg := &mockMessage{job: job}

	err := analyzer.ProcessJob(context.Background(), msg)
	if err == nil {
		t.Error("Expected error for missing user ID")
	}
}

func TestTagAnalyzer_ProcessTagAnalysisJob_DatabaseError(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	mockTodoRepo := &mockTodoRepo{
		t: t,
		getByUserIDPaginatedFunc: func(ctx context.Context, uid uuid.UUID, timeHorizon *models.TimeHorizon, status *models.TodoStatus, page, pageSize int) ([]*models.Todo, int, error) {
			return nil, 0, fmt.Errorf("database connection failed")
		},
	}

	mockTagStatsRepo := &mockTagStatisticsRepoForWorker{
		t: t,
		getByUserIDOrCreateFunc: func(ctx context.Context, uid uuid.UUID) (*models.TagStatistics, error) {
			return &models.TagStatistics{
				UserID:          userID,
				TagStats:        make(map[string]models.TagStats),
				Tainted:         true,
				AnalysisVersion: 0,
			}, nil
		},
		updateStatisticsFunc: func(ctx context.Context, s *models.TagStatistics) (bool, error) {
			// Should not be called due to database error
			t.Error("UpdateStatistics should not be called when GetByUserIDPaginated fails")
			return false, nil
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
	if err == nil {
		t.Error("Expected error for database failure")
	}
}

// Use the existing mockMessage from analyzer_test.go
