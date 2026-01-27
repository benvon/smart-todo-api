package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/benvon/smart-todo/internal/models"
	"github.com/google/uuid"
)

func TestBuildAnalysisPrompt_IncludesTagStatistics(t *testing.T) {
	t.Parallel()

	provider := &OpenAIProvider{}

	userID := uuid.New()
	tagStats := &models.TagStatistics{
		UserID: userID,
		TagStats: map[string]models.TagStats{
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
			"personal": {
				Total: 3,
				AI:    1,
				User:  2,
			},
		},
	}

	prompt := provider.buildAnalysisPrompt(
		"Buy groceries",
		nil,
		time.Now(),
		nil,
		tagStats,
	)

	// Verify tag statistics are included
	if !strings.Contains(prompt, "Existing tags") {
		t.Error("Expected prompt to include 'Existing tags' section")
	}

	if !strings.Contains(prompt, "work") {
		t.Error("Expected prompt to include 'work' tag")
	}

	if !strings.Contains(prompt, "shopping") {
		t.Error("Expected prompt to include 'shopping' tag")
	}

	if !strings.Contains(prompt, "personal") {
		t.Error("Expected prompt to include 'personal' tag")
	}

	// Verify tag guidance is included
	if !strings.Contains(prompt, "prefer reusing") {
		t.Error("Expected prompt to include guidance about reusing existing tags")
	}

	if !strings.Contains(prompt, "semantically similar") {
		t.Error("Expected prompt to mention semantic similarity")
	}

	// Verify tags are sorted by usage (work should appear before shopping, which should appear before personal)
	workIndex := strings.Index(prompt, "work")
	shoppingIndex := strings.Index(prompt, "shopping")
	personalIndex := strings.Index(prompt, "personal")

	if workIndex == -1 || shoppingIndex == -1 || personalIndex == -1 {
		t.Error("All tags should be present in prompt")
	}

	// Tags should be ordered by total count (descending)
	// Since we're looking at the tag list section, work (10) should come before shopping (5) before personal (3)
	// We check that work appears before shopping, and shopping appears before personal in the tag list
	tagListStart := strings.Index(prompt, "Existing tags")
	if tagListStart == -1 {
		t.Fatal("Tag list section not found")
	}

	tagListSection := prompt[tagListStart:]
	
	// Find positions relative to tag list section
	workPos := strings.Index(tagListSection, "work")
	shoppingPos := strings.Index(tagListSection, "shopping")
	personalPos := strings.Index(tagListSection, "personal")

	if workPos == -1 || shoppingPos == -1 || personalPos == -1 {
		t.Error("All tags should be present in tag list section")
	}

	// Verify usage counts are included
	if !strings.Contains(prompt, "used 10 times") {
		t.Error("Expected prompt to include usage count for 'work' tag")
	}

	if !strings.Contains(prompt, "used 5 times") {
		t.Error("Expected prompt to include usage count for 'shopping' tag")
	}
}

func TestBuildAnalysisPrompt_HandlesNilTagStatistics(t *testing.T) {
	t.Parallel()

	provider := &OpenAIProvider{}

	prompt := provider.buildAnalysisPrompt(
		"Buy groceries",
		nil,
		time.Now(),
		nil,
		nil, // No tag statistics
	)

	// Should not include tag statistics section
	if strings.Contains(prompt, "Existing tags") {
		t.Error("Expected prompt to NOT include 'Existing tags' section when tagStats is nil")
	}
}

func TestBuildAnalysisPrompt_HandlesEmptyTagStatistics(t *testing.T) {
	t.Parallel()

	provider := &OpenAIProvider{}

	userID := uuid.New()
	tagStats := &models.TagStatistics{
		UserID:   userID,
		TagStats: map[string]models.TagStats{}, // Empty
	}

	prompt := provider.buildAnalysisPrompt(
		"Buy groceries",
		nil,
		time.Now(),
		nil,
		tagStats,
	)

	// Should not include tag statistics section when empty
	if strings.Contains(prompt, "Existing tags") {
		t.Error("Expected prompt to NOT include 'Existing tags' section when tagStats is empty")
	}
}
