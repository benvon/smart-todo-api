package database

import (
	"context"
	"testing"

	"github.com/benvon/smart-todo/internal/models"
	"github.com/google/uuid"
)

// TestTodoRepository_TagChangeDetection tests that tag changes are detected correctly
// Note: Full integration testing of Update() with tag change detection requires a database
// This test focuses on the tag comparison logic
func TestTodoRepository_TagChangeDetection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		oldTags       []string
		newTags       []string
		expectChanged bool
	}{
		{
			name:          "tags changed - should detect change",
			oldTags:       []string{"work"},
			newTags:       []string{"work", "urgent"},
			expectChanged: true,
		},
		{
			name:          "tags removed - should detect change",
			oldTags:       []string{"work", "urgent"},
			newTags:       []string{"work"},
			expectChanged: true,
		},
		{
			name:          "tags unchanged - should not detect change",
			oldTags:       []string{"work"},
			newTags:       []string{"work"},
			expectChanged: false,
		},
		{
			name:          "no tags to no tags - should not detect change",
			oldTags:       []string{},
			newTags:       []string{},
			expectChanged: false,
		},
		{
			name:          "tags added from empty - should detect change",
			oldTags:       []string{},
			newTags:       []string{"work"},
			expectChanged: true,
		},
		{
			name:          "all tags removed - should detect change",
			oldTags:       []string{"work"},
			newTags:       []string{},
			expectChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Test tag comparison logic directly
			tagsChanged := !tagsEqual(tt.oldTags, tt.newTags)

			if tagsChanged != tt.expectChanged {
				t.Errorf("Expected tagsChanged=%v, got %v (oldTags=%v, newTags=%v)",
					tt.expectChanged, tagsChanged, tt.oldTags, tt.newTags)
			}
		})
	}
}

// TestTagsEqual tests the tagsEqual helper function
func TestTagsEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tagsA    []string
		tagsB    []string
		expected bool
	}{
		{
			name:     "identical tags",
			tagsA:    []string{"work", "urgent"},
			tagsB:    []string{"work", "urgent"},
			expected: true,
		},
		{
			name:     "same tags different order",
			tagsA:    []string{"work", "urgent"},
			tagsB:    []string{"urgent", "work"},
			expected: true,
		},
		{
			name:     "different tags",
			tagsA:    []string{"work"},
			tagsB:    []string{"urgent"},
			expected: false,
		},
		{
			name:     "one empty",
			tagsA:    []string{"work"},
			tagsB:    []string{},
			expected: false,
		},
		{
			name:     "both empty",
			tagsA:    []string{},
			tagsB:    []string{},
			expected: true,
		},
		{
			name:     "nil and empty",
			tagsA:    nil,
			tagsB:    []string{},
			expected: true,
		},
		{
			name:     "duplicate tags",
			tagsA:    []string{"work", "work"},
			tagsB:    []string{"work"},
			expected: false,
		},
		{
			name:     "subset",
			tagsA:    []string{"work", "urgent"},
			tagsB:    []string{"work"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tagsEqual(tt.tagsA, tt.tagsB)
			if result != tt.expected {
				t.Errorf("tagsEqual(%v, %v) = %v, expected %v", tt.tagsA, tt.tagsB, result, tt.expected)
			}
		})
	}
}

// mockTagStatsRepoForTodosTest is a minimal mock for testing
type mockTagStatsRepoForTodosTest struct{}

func (m *mockTagStatsRepoForTodosTest) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.TagStatistics, error) {
	return nil, nil
}

func (m *mockTagStatsRepoForTodosTest) GetByUserIDOrCreate(ctx context.Context, userID uuid.UUID) (*models.TagStatistics, error) {
	return nil, nil
}

func (m *mockTagStatsRepoForTodosTest) UpdateStatistics(ctx context.Context, stats *models.TagStatistics) (bool, error) {
	return false, nil
}

func (m *mockTagStatsRepoForTodosTest) MarkTainted(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}

var _ TagStatisticsRepositoryInterface = (*mockTagStatsRepoForTodosTest)(nil)
