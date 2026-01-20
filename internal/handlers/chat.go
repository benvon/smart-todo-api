package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/middleware"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/services/ai"
	"github.com/gorilla/mux"
)

// ChatHandler handles AI chat requests
type ChatHandler struct {
	chatService    *ai.ChatService
	contextService *ai.ContextService
	contextRepo    *database.AIContextRepository
}

// NewChatHandler creates a new chat handler
func NewChatHandler(chatService *ai.ChatService, contextService *ai.ContextService, contextRepo *database.AIContextRepository) *ChatHandler {
	return &ChatHandler{
		chatService:    chatService,
		contextService: contextService,
		contextRepo:    contextRepo,
	}
}

// RegisterRoutes registers chat routes
func (h *ChatHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/chat", h.StartChat).Methods("POST")
	r.HandleFunc("/chat/message", h.SendMessage).Methods("POST")
}

// ChatMessageRequest represents a chat message request
type ChatMessageRequest struct {
	Message string `json:"message" validate:"required"`
}

// StartChat starts a chat session and returns SSE stream
func (h *ChatHandler) StartChat(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r)
	if user == nil {
		respondJSONError(w, http.StatusUnauthorized, "Unauthorized", "User not found in context")
		return
	}

	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Get or create chat session
	session := h.chatService.GetOrCreateSession(user.ID)

	// Send initial connection message
	if _, err := fmt.Fprintf(w, "data: %s\n\n", h.formatSSEMessage("connected", map[string]any{
		"message":    "Chat session started",
		"session_id": session.UserID.String(),
	})); err != nil {
		log.Printf("Failed to write SSE message: %v", err)
		return
	}

	flusher, ok := w.(http.Flusher)
	if ok {
		flusher.Flush()
	}

	// Keep connection alive with ping every 30 seconds
	ctx := r.Context()
	// Create independent context for cleanup work before request context is cancelled
	cleanupCtx := context.WithoutCancel(ctx)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := fmt.Fprintf(w, ": ping\n\n"); err != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
		}
	}()

	// Wait for context cancellation (client disconnect)
	<-ctx.Done()

	// Summarize and save context before closing
	// Extract data before closing session, then run in background goroutine
	// with independent context since request context is already cancelled
	if session.NeedsSummaryUpdate && len(session.Messages) > 0 {
		userID := user.ID
		messages := make([]ai.ChatMessage, len(session.Messages))
		copy(messages, session.Messages)

		go func(ctx context.Context) {
			updateCtx, updateCancel := context.WithTimeout(ctx, 5*time.Second)
			defer updateCancel()

			if err := h.contextService.UpdateContextSummary(updateCtx, userID, messages); err != nil {
				log.Printf("Failed to save chat summary: %v", err)
			}
		}(cleanupCtx)
	}

	h.chatService.CloseSession(user.ID)
}

// SendMessage sends a message in the chat session
func (h *ChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r)
	if user == nil {
		respondJSONError(w, http.StatusUnauthorized, "Unauthorized", "User not found in context")
		return
	}

	var req ChatMessageRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "Bad Request", "Invalid request body")
		return
	}

	// Get or create session
	session := h.chatService.GetOrCreateSession(user.ID)

	// Add user message
	h.chatService.AddMessage(session, "user", req.Message)

	// Load user context
	ctx := r.Context()
	userContext, err := h.contextRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		// Create context if it doesn't exist
		userContext = &models.AIContext{
			UserID:      user.ID,
			Preferences: make(map[string]any),
		}
	}

	// Get AI response
	response, err := h.chatService.GetResponse(ctx, session, userContext)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to get AI response")
		return
	}

	// Periodically summarize conversation (every 10 messages)
	if len(session.Messages) > 0 && len(session.Messages)%10 == 0 {
		go func(ctx context.Context) {
			summaryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			if err := h.contextService.UpdateContextSummary(summaryCtx, user.ID, session.Messages); err != nil {
				log.Printf("Failed to summarize conversation: %v", err)
			}
		}(ctx)
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"message":      response.Message,
		"summary":      response.Summary,
		"needs_update": response.NeedsUpdate,
	})
}

// formatSSEMessage formats a message for SSE
func (h *ChatHandler) formatSSEMessage(event string, data any) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(`{"event":"%s","data":%s}`, event, string(jsonData))
}
