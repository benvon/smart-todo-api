package ai

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/benvon/smart-todo/internal/models"
	"github.com/google/uuid"
)

// ChatService manages chat sessions
type ChatService struct {
	provider AIProvider
	sessions map[uuid.UUID]*ChatSession
	mu       sync.RWMutex // Protects concurrent access to sessions map
}

// ChatSession represents an active chat session
type ChatSession struct {
	UserID             uuid.UUID
	Messages           []ChatMessage
	CreatedAt          time.Time
	LastActivity       time.Time
	ContextSummary     string
	NeedsSummaryUpdate bool
}

// NewChatService creates a new chat service
func NewChatService(provider AIProvider) *ChatService {
	return &ChatService{
		provider: provider,
		sessions: make(map[uuid.UUID]*ChatSession),
	}
}

// GetOrCreateSession gets or creates a chat session for a user
func (s *ChatService) GetOrCreateSession(userID uuid.UUID) *ChatSession {
	// Try read lock first for fast path
	s.mu.RLock()
	if session, exists := s.sessions[userID]; exists {
		s.mu.RUnlock()
		session.LastActivity = time.Now()
		return session
	}
	s.mu.RUnlock()

	// Need to create new session, acquire write lock
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have created it)
	if session, exists := s.sessions[userID]; exists {
		session.LastActivity = time.Now()
		return session
	}

	session := &ChatSession{
		UserID:       userID,
		Messages:     make([]ChatMessage, 0),
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	s.sessions[userID] = session
	return session
}

// AddMessage adds a message to the session
func (s *ChatService) AddMessage(session *ChatSession, role string, content string) {
	session.Messages = append(session.Messages, ChatMessage{
		Role:    role,
		Content: content,
	})
	session.LastActivity = time.Now()
	session.NeedsSummaryUpdate = true
}

// GetResponse gets a response from the AI for the session
func (s *ChatService) GetResponse(ctx context.Context, session *ChatSession, userContext *models.AIContext) (*ChatResponse, error) {
	response, err := s.provider.Chat(ctx, session.Messages, userContext)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat response: %w", err)
	}

	// Add AI response to session
	s.AddMessage(session, "assistant", response.Message)

	return response, nil
}

// SummarizeSession summarizes a chat session
func (s *ChatService) SummarizeSession(ctx context.Context, session *ChatSession) (string, error) {
	if len(session.Messages) == 0 {
		return "", nil
	}

	summary, err := s.provider.SummarizeContext(ctx, session.Messages)
	if err != nil {
		return "", fmt.Errorf("failed to summarize session: %w", err)
	}

	session.ContextSummary = summary
	session.NeedsSummaryUpdate = false

	return summary, nil
}

// CloseSession closes a chat session
func (s *ChatService) CloseSession(userID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, userID)
}
