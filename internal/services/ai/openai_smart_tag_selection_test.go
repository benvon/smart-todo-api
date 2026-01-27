package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/benvon/smart-todo/internal/models"
	"github.com/google/uuid"
)

func TestSelectTagsForPrompt_PrioritizesSimilarTags(t *testing.T) {
	t.Parallel()

	provider := NewOpenAIProvider("test-key", "")

	tagStats := map[string]models.TagStats{
		"work": {
			Total: 10,
			AI:    8,
			User:  2,
		},
		"shopping": {
			Total: 5,
			AI:    3,
			User:  2,
		},
		"groceries": {
			Total: 2,
			AI:    1,
			User:  1,
		},
		"personal": {
			Total: 8,
			AI:    5,
			User:  3,
		},
	}

	// Test with "buy groceries" - should prioritize "shopping" and "groceries" due to similarity
	selectedTags := provider.selectTagsForPrompt(tagStats, "buy groceries")

	if len(selectedTags) == 0 {
		t.Error("Expected to select at least some tags")
	}

	// Check that "groceries" is included despite lower usage count due to similarity
	foundGroceries := false
	for _, tag := range selectedTags {
		if tag == "groceries" {
			foundGroceries = true
			break
		}
	}

	if !foundGroceries {
		t.Errorf("Expected 'groceries' tag to be selected due to semantic similarity, got tags: %v", selectedTags)
	}
}

func TestSelectTagsForPrompt_RespectsMaxTagsLimit(t *testing.T) {
	t.Parallel()

	provider := NewOpenAIProvider("test-key", "")
	provider.maxTagsInPrompt = 2 // Limit to 2 tags

	tagStats := map[string]models.TagStats{
		"work":     {Total: 10},
		"shopping": {Total: 5},
		"personal": {Total: 8},
		"urgent":   {Total: 3},
	}

	selectedTags := provider.selectTagsForPrompt(tagStats, "finish report")

	if len(selectedTags) > 2 {
		t.Errorf("Expected at most 2 tags, got %d: %v", len(selectedTags), selectedTags)
	}
}

func TestSelectTagsForPrompt_RespectsTokenLimit(t *testing.T) {
	t.Parallel()

	provider := NewOpenAIProvider("test-key", "")
	provider.maxTagTokens = 20 // Very small token limit

	tagStats := map[string]models.TagStats{
		"work":     {Total: 10},
		"shopping": {Total: 5},
		"personal": {Total: 8},
		"urgent":   {Total: 3},
	}

	selectedTags := provider.selectTagsForPrompt(tagStats, "finish report")

	// Should select fewer tags due to token limit
	if len(selectedTags) >= len(tagStats) {
		t.Errorf("Expected fewer tags due to token limit, got %d tags", len(selectedTags))
	}
}

func TestBuildAnalysisPrompt_UsesSmartTagSelection(t *testing.T) {
	t.Parallel()

	provider := NewOpenAIProvider("test-key", "")

	userID := uuid.New()
	tagStats := &models.TagStatistics{
		UserID: userID,
		TagStats: map[string]models.TagStats{
			"work": {
				Total: 10,
				AI:    8,
				User:  2,
			},
			"meeting": {
				Total: 5,
				AI:    3,
				User:  2,
			},
			"urgent": {
				Total: 3,
				AI:    1,
				User:  2,
			},
		},
	}

	prompt := provider.buildAnalysisPrompt(
		"schedule team meeting",
		nil,
		time.Now(),
		nil,
		tagStats,
	)

	// Verify that "meeting" tag is included due to similarity
	if !strings.Contains(prompt, "meeting") {
		t.Error("Expected prompt to include 'meeting' tag due to semantic similarity to 'schedule team meeting'")
	}
}

func TestEstimateTokenCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "short text",
			text:     "test",
			expected: 1, // 4 chars / 4 = 1
		},
		{
			name:     "medium text",
			text:     "this is a test message",
			expected: 5, // 22 chars / 4 = 5
		},
		{
			name:     "empty text",
			text:     "",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimateTokenCount(tt.text)
			if result != tt.expected {
				t.Errorf("estimateTokenCount(%q) = %d, expected %d", tt.text, result, tt.expected)
			}
		})
	}
}

func TestCalculateStringSimilarity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		s1       string
		s2       string
		minScore float64 // Minimum expected score
	}{
		{
			name:     "identical strings",
			s1:       "shopping",
			s2:       "shopping",
			minScore: 0.9, // Should be very high
		},
		{
			name:     "similar strings",
			s1:       "buy groceries",
			s2:       "groceries",
			minScore: 0.3, // Should have some similarity
		},
		{
			name:     "unrelated strings",
			s1:       "work",
			s2:       "shopping",
			minScore: 0.0,
		},
		{
			name:     "empty strings",
			s1:       "",
			s2:       "test",
			minScore: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateStringSimilarity(tt.s1, tt.s2)
			if result < 0 || result > 1 {
				t.Errorf("calculateStringSimilarity(%q, %q) = %f, expected value between 0 and 1", tt.s1, tt.s2, result)
			}
			if result < tt.minScore {
				t.Errorf("calculateStringSimilarity(%q, %q) = %f, expected at least %f", tt.s1, tt.s2, result, tt.minScore)
			}
		})
	}
}
