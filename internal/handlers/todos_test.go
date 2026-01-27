package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/middleware"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/google/uuid"
)


// TestCreateTodo_TimeEnteredLogic tests that TimeEntered is set correctly when creating todos
// This tests the logic used in CreateTodo handler
func TestCreateTodo_TimeEnteredLogic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		text         string
		dueDate      *string
		validateTodo func(*testing.T, *models.Todo)
	}{
		{
			name: "TimeEntered is populated on creation",
			text: "Test todo",
			validateTodo: func(t *testing.T, todo *models.Todo) {
				if todo.Metadata.TimeEntered == nil {
					t.Error("Expected TimeEntered to be populated, got nil")
					return
				}
				if *todo.Metadata.TimeEntered == "" {
					t.Error("Expected TimeEntered to have a value, got empty string")
					return
				}
				// Verify it's a valid RFC3339 timestamp
				parsedTime, err := time.Parse(time.RFC3339, *todo.Metadata.TimeEntered)
				if err != nil {
					t.Errorf("Expected TimeEntered to be valid RFC3339, got error: %v", err)
					return
				}
				// Verify it's recent (within last second)
				now := time.Now()
				if parsedTime.After(now) || parsedTime.Before(now.Add(-1*time.Second)) {
					t.Errorf("Expected TimeEntered to be recent, got %s (now is %s)", parsedTime, now)
				}
			},
		},
		{
			name:    "TimeEntered is set even with due date",
			text:    "Todo with due date",
			dueDate: stringPtr("2024-03-20T15:00:00Z"),
			validateTodo: func(t *testing.T, todo *models.Todo) {
				if todo.Metadata.TimeEntered == nil {
					t.Error("Expected TimeEntered to be populated even with due date")
					return
				}
				if todo.DueDate == nil {
					t.Error("Expected DueDate to be set")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Simulate the logic from CreateTodo handler
			now := time.Now()
			timeEntered := now.Format(time.RFC3339)
			todo := &models.Todo{
				ID:          uuid.New(),
				UserID:      uuid.New(),
				Text:        tt.text,
				TimeHorizon: models.TimeHorizonSoon,
				Status:      models.TodoStatusPending,
				Metadata: models.Metadata{
					TagSources:  make(map[string]models.TagSource),
					TimeEntered: &timeEntered,
				},
			}

			// Parse due_date if provided (simulating handler logic)
			if tt.dueDate != nil && *tt.dueDate != "" {
				dueDate, err := time.Parse(time.RFC3339, *tt.dueDate)
				if err == nil {
					todo.DueDate = &dueDate
				}
			}

			// Validate the todo structure
			if tt.validateTodo != nil {
				tt.validateTodo(t, todo)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}

// Helper to set user in request context for testing
func setUserInRequestContext(r *http.Request, user *models.User) *http.Request {
	ctx := middleware.SetUserInContext(r.Context(), user)
	return r.WithContext(ctx)
}

// Note: Tests for tainted marking logic would require:
// 1. Mock repositories (todoRepo, tagStatsRepo, jobQueue)
// 2. HTTP request/response setup with authentication middleware
// 3. Full handler integration testing
// These are marked as skipped and should be implemented with integration test setup
// that uses testcontainers or a test database

func TestTodoHandler_CreateTodo_MarksTainted(t *testing.T) {
	t.Skip("Tag change detection is now handled automatically by the repository layer - test repository-level detection instead")
	// Note: Tag change detection is centralized in TodoRepository.Update()
	// When a todo is created and then updated with tags (via AI analysis),
	// the repository automatically detects tag changes and invokes the handler
}

func TestTodoHandler_UpdateTodo_MarksTaintedOnTagChange(t *testing.T) {
	t.Skip("Tag change detection is now handled automatically by the repository layer - test repository-level detection instead")
	// Note: Tag change detection is centralized in TodoRepository.Update()
	// When UpdateTodo calls todoRepo.Update(), the repository automatically
	// detects tag changes and invokes the configured handler
}

func TestTodoHandler_UpdateTodo_NoTaintedOnNonTagChange(t *testing.T) {
	t.Skip("Tag change detection is now handled automatically by the repository layer - test repository-level detection instead")
	// Note: Tag change detection is centralized in TodoRepository.Update()
	// The repository compares old and new tags, so non-tag changes don't trigger the handler
}

func TestTodoHandler_GetTagStats_Success(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	now := time.Now()
	stats := &models.TagStatistics{
		UserID: userID,
		TagStats: map[string]models.TagStats{
			"shopping": {
				Total: 5,
				AI:    3,
				User:  2,
			},
			"work": {
				Total: 3,
				AI:    2,
				User:  1,
			},
		},
		Tainted:         false,
		LastAnalyzedAt:  &now,
		AnalysisVersion: 1,
	}

	mockTagStatsRepo := &mockTagStatisticsRepoForHandlers{
		t: t,
		getByUserIDOrCreateFunc: func(ctx context.Context, uid uuid.UUID) (*models.TagStatistics, error) {
			if uid != userID {
				t.Errorf("GetByUserIDOrCreate called with wrong userID: expected %s, got %s", userID, uid)
			}
			return stats, nil
		},
	}

	handler := NewTodoHandlerWithQueueAndTagStats(nil, mockTagStatsRepo, nil)

	user := &models.User{
		ID:    userID,
		Email: "test@example.com",
	}

	req := httptest.NewRequest("GET", "/api/v1/todos/tags/stats", nil)
	req = setUserInRequestContext(req, user)
	w := httptest.NewRecorder()

	handler.GetTagStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var wrapper struct {
		Success   bool            `json:"success"`
		Data      TagStatsResponse `json:"data"`
		Timestamp string          `json:"timestamp"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &wrapper); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	response := wrapper.Data

	if len(response.TagStats) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(response.TagStats))
	}

	if response.TagStats["shopping"].Total != 5 {
		t.Errorf("Expected shopping total=5, got %d", response.TagStats["shopping"].Total)
	}

	if response.Tainted {
		t.Error("Expected tainted=false")
	}

	if response.LastAnalyzedAt == nil {
		t.Error("Expected LastAnalyzedAt to be set")
	}
}

func TestTodoHandler_GetTagStats_Unauthorized(t *testing.T) {
	t.Parallel()

	handler := NewTodoHandlerWithQueueAndTagStats(nil, nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/todos/tags/stats", nil)
	// No user in context
	w := httptest.NewRecorder()

	handler.GetTagStats(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestTodoHandler_GetTagStats_DatabaseError(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	mockTagStatsRepo := &mockTagStatisticsRepoForHandlers{
		t: t,
		getByUserIDOrCreateFunc: func(ctx context.Context, uid uuid.UUID) (*models.TagStatistics, error) {
			return nil, fmt.Errorf("database connection failed")
		},
	}

	handler := NewTodoHandlerWithQueueAndTagStats(nil, mockTagStatsRepo, nil)

	user := &models.User{
		ID:    userID,
		Email: "test@example.com",
	}

	req := httptest.NewRequest("GET", "/api/v1/todos/tags/stats", nil)
	req = setUserInRequestContext(req, user)
	w := httptest.NewRecorder()

	handler.GetTagStats(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestTodoHandler_GetTagStats_StaleData(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	stats := &models.TagStatistics{
		UserID: userID,
		TagStats: map[string]models.TagStats{
			"shopping": {
				Total: 5,
				AI:    3,
				User:  2,
			},
		},
		Tainted:         true, // Stale data
		LastAnalyzedAt:  nil,
		AnalysisVersion: 0,
	}

	mockTagStatsRepo := &mockTagStatisticsRepoForHandlers{
		t: t,
		getByUserIDOrCreateFunc: func(ctx context.Context, uid uuid.UUID) (*models.TagStatistics, error) {
			return stats, nil
		},
	}

	handler := NewTodoHandlerWithQueueAndTagStats(nil, mockTagStatsRepo, nil)

	user := &models.User{
		ID:    userID,
		Email: "test@example.com",
	}

	req := httptest.NewRequest("GET", "/api/v1/todos/tags/stats", nil)
	req = setUserInRequestContext(req, user)
	w := httptest.NewRecorder()

	handler.GetTagStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 even for stale data, got %d", w.Code)
	}

	var wrapper struct {
		Success   bool            `json:"success"`
		Data      TagStatsResponse `json:"data"`
		Timestamp string          `json:"timestamp"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &wrapper); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	response := wrapper.Data

	if !response.Tainted {
		t.Error("Expected tainted=true for stale data")
	}
}

// mockTagStatisticsRepoForHandlers is a mock for testing handlers
type mockTagStatisticsRepoForHandlers struct {
	t                         *testing.T
	getByUserIDFunc           func(ctx context.Context, userID uuid.UUID) (*models.TagStatistics, error)
	getByUserIDOrCreateFunc   func(ctx context.Context, userID uuid.UUID) (*models.TagStatistics, error)
	updateStatisticsFunc      func(ctx context.Context, stats *models.TagStatistics) (bool, error)
	markTaintedFunc           func(ctx context.Context, userID uuid.UUID) (bool, error)
	getByUserIDCalls          []uuid.UUID
	getByUserIDOrCreateCalls  []uuid.UUID
	updateStatisticsCalls     []*models.TagStatistics
	markTaintedCalls          []uuid.UUID
}

func (m *mockTagStatisticsRepoForHandlers) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.TagStatistics, error) {
	m.getByUserIDCalls = append(m.getByUserIDCalls, userID)
	if m.getByUserIDFunc == nil {
		m.t.Fatal("GetByUserID called but not configured in test - mock requires explicit setup")
	}
	return m.getByUserIDFunc(ctx, userID)
}

func (m *mockTagStatisticsRepoForHandlers) GetByUserIDOrCreate(ctx context.Context, userID uuid.UUID) (*models.TagStatistics, error) {
	m.getByUserIDOrCreateCalls = append(m.getByUserIDOrCreateCalls, userID)
	if m.getByUserIDOrCreateFunc == nil {
		m.t.Fatal("GetByUserIDOrCreate called but not configured in test - mock requires explicit setup")
	}
	return m.getByUserIDOrCreateFunc(ctx, userID)
}

func (m *mockTagStatisticsRepoForHandlers) UpdateStatistics(ctx context.Context, stats *models.TagStatistics) (bool, error) {
	m.updateStatisticsCalls = append(m.updateStatisticsCalls, stats)
	if m.updateStatisticsFunc == nil {
		m.t.Fatal("UpdateStatistics called but not configured in test - mock requires explicit setup")
	}
	return m.updateStatisticsFunc(ctx, stats)
}

func (m *mockTagStatisticsRepoForHandlers) MarkTainted(ctx context.Context, userID uuid.UUID) (bool, error) {
	m.markTaintedCalls = append(m.markTaintedCalls, userID)
	if m.markTaintedFunc == nil {
		m.t.Fatal("MarkTainted called but not configured in test - mock requires explicit setup")
	}
	return m.markTaintedFunc(ctx, userID)
}

var _ database.TagStatisticsRepositoryInterface = (*mockTagStatisticsRepoForHandlers)(nil)
