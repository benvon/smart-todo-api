package models

import (
	"testing"
)

func TestTimeHorizon_Values(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value TimeHorizon
		valid bool
	}{
		{"next", TimeHorizonNext, true},
		{"soon", TimeHorizonSoon, true},
		{"later", TimeHorizonLater, true},
		{"invalid", TimeHorizon("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			switch tt.value {
			case TimeHorizonNext, TimeHorizonSoon, TimeHorizonLater:
				if !tt.valid {
					t.Errorf("Expected %s to be invalid", tt.value)
				}
			default:
				if tt.valid {
					t.Errorf("Expected %s to be valid", tt.value)
				}
			}
		})
	}
}

func TestTodoStatus_Values(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value TodoStatus
		valid bool
	}{
		{"pending", TodoStatusPending, true},
		{"processing", TodoStatusProcessing, true},
		{"processed", TodoStatusProcessed, true},
		{"completed", TodoStatusCompleted, true},
		{"invalid", TodoStatus("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			switch tt.value {
			case TodoStatusPending, TodoStatusProcessing, TodoStatusProcessed, TodoStatusCompleted:
				if !tt.valid {
					t.Errorf("Expected %s to be invalid", tt.value)
				}
			default:
				if tt.valid {
					t.Errorf("Expected %s to be valid", tt.value)
				}
			}
		})
	}
}
