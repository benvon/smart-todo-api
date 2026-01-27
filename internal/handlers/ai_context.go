package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/middleware"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/gorilla/mux"
)

// AIContextHandler handles AI context-related requests
type AIContextHandler struct {
	contextRepo *database.AIContextRepository
}

// NewAIContextHandler creates a new AI context handler
func NewAIContextHandler(contextRepo *database.AIContextRepository) *AIContextHandler {
	return &AIContextHandler{
		contextRepo: contextRepo,
	}
}

// RegisterRoutes registers AI context routes on the given router
// The router should already have the /ai/context prefix
func (h *AIContextHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("", h.GetContext).Methods("GET")
	r.HandleFunc("", h.UpdateContext).Methods("PUT")
}

// GetContextRequest represents a get context request (empty for now, but could be extended)
type GetContextResponse struct {
	ContextSummary string         `json:"context_summary,omitempty"`
	Preferences    map[string]any `json:"preferences,omitempty"`
}

// GetContext returns the current user's AI context
func (h *AIContextHandler) GetContext(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r)
	if user == nil {
		respondJSONError(w, http.StatusUnauthorized, "Unauthorized", "User not found in context")
		return
	}

	ctx := r.Context()

	// Get or create context
	aiContext, err := h.contextRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		// Create new context if not found
		aiContext = &models.AIContext{
			UserID:      user.ID,
			Preferences: make(map[string]any),
		}
		if err := h.contextRepo.Create(ctx, aiContext); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to create AI context")
			return
		}
	}

	response := GetContextResponse{
		ContextSummary: aiContext.ContextSummary,
		Preferences:    aiContext.Preferences,
	}

	respondJSON(w, http.StatusOK, response)
}

// UpdateContextRequest represents an update context request
type UpdateContextRequest struct {
	ContextSummary *string        `json:"context_summary,omitempty"`
	Preferences    map[string]any `json:"preferences,omitempty"`
}

// UpdateContext updates the current user's AI context
func (h *AIContextHandler) UpdateContext(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r)
	if user == nil {
		respondJSONError(w, http.StatusUnauthorized, "Unauthorized", "User not found in context")
		return
	}

	var req UpdateContextRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		// Check if error is due to request size limit
		if maxBytesErr, ok := err.(*http.MaxBytesError); ok {
			respondJSONError(w, http.StatusRequestEntityTooLarge, "Request Entity Too Large", fmt.Sprintf("Request body exceeds maximum size of %d bytes", maxBytesErr.Limit))
			return
		}
		respondJSONError(w, http.StatusBadRequest, "Bad Request", "Invalid request body")
		return
	}

	ctx := r.Context()

	// Get or create context
	aiContext, err := h.contextRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		// Create new context if not found
		aiContext = &models.AIContext{
			UserID:      user.ID,
			Preferences: make(map[string]any),
		}
	}

	// Update context summary if provided
	if req.ContextSummary != nil {
		aiContext.ContextSummary = *req.ContextSummary
	}

	// Update preferences if provided (merge with existing)
	if req.Preferences != nil {
		if aiContext.Preferences == nil {
			aiContext.Preferences = make(map[string]any)
		}
		// Merge preferences (new values override existing)
		for k, v := range req.Preferences {
			aiContext.Preferences[k] = v
		}
	}

	// Upsert context
	if err := h.contextRepo.Upsert(ctx, aiContext); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to update AI context")
		return
	}

	response := GetContextResponse{
		ContextSummary: aiContext.ContextSummary,
		Preferences:    aiContext.Preferences,
	}

	respondJSON(w, http.StatusOK, response)
}
