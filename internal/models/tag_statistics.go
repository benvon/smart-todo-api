package models

import (
	"time"

	"github.com/google/uuid"
)

// TagStats represents aggregated statistics for a single tag
type TagStats struct {
	Total int `json:"total"` // Total count of todos with this tag
	AI    int `json:"ai"`    // Count of todos where tag was AI-generated
	User  int `json:"user"`  // Count of todos where tag was user-defined
}

// TagStatistics represents tag statistics for a user
type TagStatistics struct {
	UserID          uuid.UUID           `json:"user_id"`
	TagStats        map[string]TagStats `json:"tag_stats"` // Maps tag name to statistics
	Tainted         bool                `json:"tainted"`
	LastAnalyzedAt  *time.Time          `json:"last_analyzed_at,omitempty"`
	AnalysisVersion int                 `json:"analysis_version"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
}
