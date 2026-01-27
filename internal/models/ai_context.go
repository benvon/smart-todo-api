package models

import (
	"time"

	"github.com/google/uuid"
)

// AIContext represents a user's AI context and preferences
type AIContext struct {
	ID             uuid.UUID      `json:"id"`
	UserID         uuid.UUID      `json:"user_id"`
	ContextSummary string         `json:"context_summary,omitempty"`
	Preferences    map[string]any `json:"preferences,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// UserActivity represents a user's activity tracking information
type UserActivity struct {
	UserID             uuid.UUID `json:"user_id"`
	LastAPIInteraction time.Time `json:"last_api_interaction"`
	ReprocessingPaused bool      `json:"reprocessing_paused"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
