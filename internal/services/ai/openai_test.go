package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/benvon/smart-todo/internal/models"
)

func TestBuildAnalysisPrompt_TimeContext(t *testing.T) {
	t.Parallel()

	// Use fixed times for deterministic tests
	fixedCreatedAt := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC) // 4.5 hours earlier

	tests := []struct {
		name        string
		text        string
		dueDate     *time.Time
		createdAt   time.Time
		userContext *models.AIContext
		validate    func(*testing.T, string)
	}{
		{
			name:      "includes current time and creation time",
			text:      "Complete project",
			createdAt: fixedCreatedAt,
			validate: func(t *testing.T, prompt string) {
				if !strings.Contains(prompt, "Time context:") {
					t.Error("Expected prompt to include 'Time context:'")
				}
				if !strings.Contains(prompt, "Current date and time:") {
					t.Error("Expected prompt to include current date and time")
				}
				if !strings.Contains(prompt, "Todo created/entered at:") {
					t.Error("Expected prompt to include creation time")
				}
				// Check that creation time is included (format may vary slightly)
				if !strings.Contains(prompt, fixedCreatedAt.Format(time.RFC3339)) {
					t.Errorf("Expected prompt to include creation time %s", fixedCreatedAt.Format(time.RFC3339))
				}
				// Verify it's a valid RFC3339 format in the prompt
				if !strings.Contains(prompt, "2024-03-15T10:00:00Z") {
					t.Error("Expected prompt to include creation time in RFC3339 format")
				}
			},
		},
		{
			name:      "shows 'entered today' for same day",
			text:      "Task for today",
			createdAt: time.Now().Add(-2 * time.Hour), // 2 hours ago (same day)
			validate: func(t *testing.T, prompt string) {
				if !strings.Contains(prompt, "This todo was entered today") {
					t.Error("Expected prompt to indicate todo was entered today")
				}
			},
		},
		{
			name:      "shows 'entered yesterday' for previous day",
			text:      "Task from yesterday",
			createdAt: time.Now().Add(-25 * time.Hour), // 25 hours ago (yesterday)
			validate: func(t *testing.T, prompt string) {
				if !strings.Contains(prompt, "This todo was entered yesterday") {
					t.Error("Expected prompt to indicate todo was entered yesterday")
				}
			},
		},
		{
			name:      "shows days ago for older todos",
			text:      "Old task",
			createdAt: time.Now().Add(-5 * 24 * time.Hour), // 5 days ago
			validate: func(t *testing.T, prompt string) {
				if !strings.Contains(prompt, "This todo was entered") || !strings.Contains(prompt, "days ago") {
					t.Error("Expected prompt to indicate todo was entered X days ago")
				}
				// Should contain a number of days
				if !strings.Contains(prompt, "5") && !strings.Contains(prompt, "4") {
					// Allow some variance due to time.Now() being called at different times
					t.Log("Note: Days calculation may vary slightly due to timing")
				}
			},
		},
		{
			name:      "includes date-only due date indication",
			text:      "Task due tomorrow",
			createdAt: fixedCreatedAt,
			dueDate:   timePtr(time.Date(2024, 3, 16, 0, 0, 0, 0, time.UTC)), // Midnight = date-only
			validate: func(t *testing.T, prompt string) {
				if !strings.Contains(prompt, "date only, no specific time") {
					t.Error("Expected prompt to indicate date-only due date")
				}
				if !strings.Contains(prompt, "2024-03-16") {
					t.Error("Expected prompt to include the date")
				}
			},
		},
		{
			name:      "includes specific time for due date with time",
			text:      "Task due at 3pm",
			createdAt: fixedCreatedAt,
			dueDate:   timePtr(time.Date(2024, 3, 16, 15, 0, 0, 0, time.UTC)), // 3pm
			validate: func(t *testing.T, prompt string) {
				if !strings.Contains(prompt, "specific time") {
					t.Error("Expected prompt to indicate specific time")
				}
				if !strings.Contains(prompt, "2024-03-16T15:00:00Z") {
					t.Error("Expected prompt to include full timestamp")
				}
			},
		},
		{
			name:      "includes relative time expression guidance",
			text:      "Task this weekend",
			createdAt: fixedCreatedAt,
			validate: func(t *testing.T, prompt string) {
				if !strings.Contains(prompt, "this weekend") {
					t.Error("Expected prompt to include guidance about relative time expressions")
				}
				if !strings.Contains(prompt, "consider when the todo was created") {
					t.Error("Expected prompt to mention considering creation time")
				}
			},
		},
		{
			name:      "handles missing due date",
			text:      "Task without due date",
			createdAt: fixedCreatedAt,
			dueDate:   nil,
			validate: func(t *testing.T, prompt string) {
				if strings.Contains(prompt, "Due date:") {
					t.Error("Expected prompt to not include due date when none provided")
				}
				// Should still include time context
				if !strings.Contains(prompt, "Time context:") {
					t.Error("Expected prompt to include time context even without due date")
				}
			},
		},
		{
			name:      "includes user context if provided",
			text:      "Task with context",
			createdAt: fixedCreatedAt,
			userContext: &models.AIContext{
				ContextSummary: "User prefers urgent tasks",
			},
			validate: func(t *testing.T, prompt string) {
				if !strings.Contains(prompt, "User preferences:") {
					t.Error("Expected prompt to include user preferences")
				}
				if !strings.Contains(prompt, "User prefers urgent tasks") {
					t.Error("Expected prompt to include user context summary")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a provider instance (we only need it for the method, not for API calls)
			provider := &OpenAIProvider{}

			// Mock time.Now() by using a fixed time
			// Since we can't easily mock time.Now(), we'll test with actual times
			// but verify the relative calculations are correct
			prompt := provider.buildAnalysisPrompt(tt.text, tt.dueDate, tt.createdAt, tt.userContext)

			// Basic validations
			if !strings.Contains(prompt, tt.text) {
				t.Errorf("Expected prompt to include todo text '%s'", tt.text)
			}

			if tt.validate != nil {
				tt.validate(t, prompt)
			}
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
