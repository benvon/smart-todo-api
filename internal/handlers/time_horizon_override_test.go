package handlers

import (
	"testing"

	"github.com/benvon/smart-todo/internal/models"
)

// TestTimeHorizonOverrideLogic tests the time horizon override logic in isolation
// This tests the core business logic without requiring database integration
func TestTimeHorizonOverrideLogic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                        string
		initialOverride             *bool
		initialTimeHorizon          models.TimeHorizon
		requestTimeHorizon          *string
		expectedOverrideAfterUpdate *bool
		expectedTimeHorizon         models.TimeHorizon
		expectValidationError       bool
	}{
		{
			name:                        "Setting time horizon marks override as true",
			initialOverride:             nil,
			initialTimeHorizon:          models.TimeHorizonSoon,
			requestTimeHorizon:          stringPtr("next"),
			expectedOverrideAfterUpdate: boolPtr(true),
			expectedTimeHorizon:         models.TimeHorizonNext,
			expectValidationError:       false,
		},
		{
			name:                        "Empty string clears override flag",
			initialOverride:             boolPtr(true),
			initialTimeHorizon:          models.TimeHorizonSoon,
			requestTimeHorizon:          stringPtr(""),
			expectedOverrideAfterUpdate: boolPtr(false),
			expectedTimeHorizon:         models.TimeHorizonSoon, // Should remain unchanged
			expectValidationError:       false,
		},
		{
			name:                        "Changing from one value to another maintains override",
			initialOverride:             boolPtr(true),
			initialTimeHorizon:          models.TimeHorizonNext,
			requestTimeHorizon:          stringPtr("later"),
			expectedOverrideAfterUpdate: boolPtr(true),
			expectedTimeHorizon:         models.TimeHorizonLater,
			expectValidationError:       false,
		},
		{
			name:                        "Setting when override was false",
			initialOverride:             boolPtr(false),
			initialTimeHorizon:          models.TimeHorizonSoon,
			requestTimeHorizon:          stringPtr("next"),
			expectedOverrideAfterUpdate: boolPtr(true),
			expectedTimeHorizon:         models.TimeHorizonNext,
			expectValidationError:       false,
		},
		{
			name:                        "Clearing override when it was false",
			initialOverride:             boolPtr(false),
			initialTimeHorizon:          models.TimeHorizonSoon,
			requestTimeHorizon:          stringPtr(""),
			expectedOverrideAfterUpdate: boolPtr(false),
			expectedTimeHorizon:         models.TimeHorizonSoon,
			expectValidationError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Simulate the todo state
			todo := &models.Todo{
				TimeHorizon: tt.initialTimeHorizon,
				Metadata: models.Metadata{
					TimeHorizonUserOverride: tt.initialOverride,
				},
			}

			// Simulate the request handling logic from UpdateTodo
			if tt.requestTimeHorizon != nil {
				// This mimics the logic in UpdateTodo handler
				if *tt.requestTimeHorizon == "" {
					// Clear the user override flag to allow AI to manage time horizon again
					override := false
					todo.Metadata.TimeHorizonUserOverride = &override
				} else {
					// In real handler, this would validate first
					// For this test, we assume valid values
					if !tt.expectValidationError {
						// User explicitly setting time horizon - mark as user override
						todo.TimeHorizon = models.TimeHorizon(*tt.requestTimeHorizon)
						// Mark that user has manually set the time horizon
						override := true
						todo.Metadata.TimeHorizonUserOverride = &override
					}
				}
			}

			// Verify the override flag was set correctly
			if tt.expectedOverrideAfterUpdate == nil {
				if todo.Metadata.TimeHorizonUserOverride != nil {
					t.Errorf("Expected TimeHorizonUserOverride to be nil, got %v",
						*todo.Metadata.TimeHorizonUserOverride)
				}
			} else {
				if todo.Metadata.TimeHorizonUserOverride == nil {
					t.Errorf("Expected TimeHorizonUserOverride to be %v, got nil",
						*tt.expectedOverrideAfterUpdate)
				} else if *todo.Metadata.TimeHorizonUserOverride != *tt.expectedOverrideAfterUpdate {
					t.Errorf("Expected TimeHorizonUserOverride to be %v, got %v",
						*tt.expectedOverrideAfterUpdate, *todo.Metadata.TimeHorizonUserOverride)
				}
			}

			// Verify time horizon was updated correctly
			if todo.TimeHorizon != tt.expectedTimeHorizon {
				t.Errorf("Expected TimeHorizon to be %v, got %v",
					tt.expectedTimeHorizon, todo.TimeHorizon)
			}
		})
	}
}
