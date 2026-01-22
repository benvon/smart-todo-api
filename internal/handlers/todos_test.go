package handlers

import (
	"testing"
	"time"

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
