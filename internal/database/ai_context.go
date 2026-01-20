package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/benvon/smart-todo/internal/models"
)

// AIContextRepository handles AI context database operations
type AIContextRepository struct {
	db *DB
}

// NewAIContextRepository creates a new AI context repository
func NewAIContextRepository(db *DB) *AIContextRepository {
	return &AIContextRepository{db: db}
}

// GetByUserID retrieves AI context by user ID
func (r *AIContextRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.AIContext, error) {
	aiContext := &models.AIContext{}
	var preferencesJSON []byte
	
	query := `
		SELECT id, user_id, context_summary, preferences, created_at, updated_at
		FROM ai_context
		WHERE user_id = $1
	`
	
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&aiContext.ID,
		&aiContext.UserID,
		&aiContext.ContextSummary,
		&preferencesJSON,
		&aiContext.CreatedAt,
		&aiContext.UpdatedAt,
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get AI context: %w", err)
	}
	
	if len(preferencesJSON) > 0 {
		if err := json.Unmarshal(preferencesJSON, &aiContext.Preferences); err != nil {
			return nil, fmt.Errorf("failed to unmarshal preferences: %w", err)
		}
	}
	
	return aiContext, nil
}

// Create creates a new AI context
func (r *AIContextRepository) Create(ctx context.Context, aiContext *models.AIContext) error {
	query := `
		INSERT INTO ai_context (id, user_id, context_summary, preferences, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at
	`
	
	preferencesJSON, err := json.Marshal(aiContext.Preferences)
	if err != nil {
		return fmt.Errorf("failed to marshal preferences: %w", err)
	}
	
	now := time.Now()
	err = r.db.QueryRowContext(ctx, query,
		aiContext.ID,
		aiContext.UserID,
		aiContext.ContextSummary,
		preferencesJSON,
		now,
		now,
	).Scan(&aiContext.CreatedAt, &aiContext.UpdatedAt)
	
	if err != nil {
		return fmt.Errorf("failed to create AI context: %w", err)
	}
	
	return nil
}

// Update updates an existing AI context
func (r *AIContextRepository) Update(ctx context.Context, aiContext *models.AIContext) error {
	query := `
		UPDATE ai_context
		SET context_summary = $2, preferences = $3, updated_at = $4
		WHERE user_id = $1
		RETURNING id, created_at, updated_at
	`
	
	preferencesJSON, err := json.Marshal(aiContext.Preferences)
	if err != nil {
		return fmt.Errorf("failed to marshal preferences: %w", err)
	}
	
	now := time.Now()
	err = r.db.QueryRowContext(ctx, query,
		aiContext.UserID,
		aiContext.ContextSummary,
		preferencesJSON,
		now,
	).Scan(&aiContext.ID, &aiContext.CreatedAt, &aiContext.UpdatedAt)
	
	if err != nil {
		return fmt.Errorf("failed to update AI context: %w", err)
	}
	
	return nil
}

// Upsert creates or updates AI context
func (r *AIContextRepository) Upsert(ctx context.Context, aiContext *models.AIContext) error {
	if aiContext.ID == uuid.Nil {
		aiContext.ID = uuid.New()
	}
	
	query := `
		INSERT INTO ai_context (id, user_id, context_summary, preferences, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE
		SET context_summary = EXCLUDED.context_summary,
		    preferences = EXCLUDED.preferences,
		    updated_at = EXCLUDED.updated_at
		RETURNING created_at, updated_at
	`
	
	preferencesJSON, err := json.Marshal(aiContext.Preferences)
	if err != nil {
		return fmt.Errorf("failed to marshal preferences: %w", err)
	}
	
	now := time.Now()
	err = r.db.QueryRowContext(ctx, query,
		aiContext.ID,
		aiContext.UserID,
		aiContext.ContextSummary,
		preferencesJSON,
		now,
		now,
	).Scan(&aiContext.CreatedAt, &aiContext.UpdatedAt)
	
	if err != nil {
		return fmt.Errorf("failed to upsert AI context: %w", err)
	}
	
	return nil
}
