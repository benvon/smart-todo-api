package ai

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/models"
)

// ContextService manages AI context for users
type ContextService struct {
	provider  AIProvider
	contextRepo *database.AIContextRepository
}

// NewContextService creates a new context service
func NewContextService(provider AIProvider, contextRepo *database.AIContextRepository) *ContextService {
	return &ContextService{
		provider:    provider,
		contextRepo: contextRepo,
	}
}

// GetOrCreateContext gets or creates AI context for a user
func (s *ContextService) GetOrCreateContext(ctx context.Context, userID uuid.UUID) (*models.AIContext, error) {
	aiContext, err := s.contextRepo.GetByUserID(ctx, userID)
	if err == nil {
		return aiContext, nil
	}
	
	// Create new context if not found
	aiContext = &models.AIContext{
		UserID:      userID,
		Preferences: make(map[string]any),
	}
	
	if err := s.contextRepo.Create(ctx, aiContext); err != nil {
		return nil, fmt.Errorf("failed to create AI context: %w", err)
	}
	
	return aiContext, nil
}

// UpdateContextSummary updates the context summary from a conversation
func (s *ContextService) UpdateContextSummary(ctx context.Context, userID uuid.UUID, conversationHistory []ChatMessage) error {
	// Summarize conversation
	summary, err := s.provider.SummarizeContext(ctx, conversationHistory)
	if err != nil {
		return fmt.Errorf("failed to summarize context: %w", err)
	}
	
	// Get or create context
	aiContext, err := s.GetOrCreateContext(ctx, userID)
	if err != nil {
		return err
	}
	
	// Update summary
	aiContext.ContextSummary = summary
	
	// Update in database
	if err := s.contextRepo.Update(ctx, aiContext); err != nil {
		return fmt.Errorf("failed to update context: %w", err)
	}
	
	return nil
}

// MergeContextSummary merges a new summary with existing context
func (s *ContextService) MergeContextSummary(ctx context.Context, userID uuid.UUID, newSummary string) error {
	aiContext, err := s.GetOrCreateContext(ctx, userID)
	if err != nil {
		return err
	}
	
	// Simple merge: append new summary to existing
	if aiContext.ContextSummary != "" {
		aiContext.ContextSummary = aiContext.ContextSummary + "\n\n" + newSummary
	} else {
		aiContext.ContextSummary = newSummary
	}
	
	// Update in database
	if err := s.contextRepo.Update(ctx, aiContext); err != nil {
		return fmt.Errorf("failed to update context: %w", err)
	}
	
	return nil
}

// LoadContextForAnalysis loads user context for task analysis
func (s *ContextService) LoadContextForAnalysis(ctx context.Context, userID uuid.UUID) (*models.AIContext, error) {
	return s.GetOrCreateContext(ctx, userID)
}
