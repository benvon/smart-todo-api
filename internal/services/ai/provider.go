package ai

import (
	"context"
	"time"

	"github.com/benvon/smart-todo/internal/models"
)

// AIProvider is the interface for AI providers
type AIProvider interface {
	// AnalyzeTask analyzes a task and returns suggested tags and time horizon
	AnalyzeTask(ctx context.Context, text string, userContext *models.AIContext) ([]string, models.TimeHorizon, error)

	// Chat handles a chat message and returns the AI response
	Chat(ctx context.Context, messages []ChatMessage, userContext *models.AIContext) (*ChatResponse, error)

	// SummarizeContext summarizes a conversation history into a context summary
	SummarizeContext(ctx context.Context, conversationHistory []ChatMessage) (string, error)
}

// AIProviderWithDueDate is an optional interface for providers that support due date analysis
type AIProviderWithDueDate interface {
	AIProvider
	// AnalyzeTaskWithDueDate analyzes a task with an optional due date and creation time, returns suggested tags and time horizon
	// createdAt is when the todo was created/entered, used for understanding relative time expressions
	AnalyzeTaskWithDueDate(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext) ([]string, models.TimeHorizon, error)
}

// ChatMessage represents a message in a chat conversation
type ChatMessage struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// ChatResponse represents a response from the AI chat
type ChatResponse struct {
	Message     string `json:"message"`
	Summary     string `json:"summary,omitempty"`      // Optional summary of the conversation
	NeedsUpdate bool   `json:"needs_update,omitempty"` // Whether context needs updating
}

// ProviderFactory creates an AI provider based on the provider type
type ProviderFactory func(config map[string]string) (AIProvider, error)

// ProviderRegistry stores available AI providers
type ProviderRegistry struct {
	providers map[string]ProviderFactory
}

// NewProviderRegistry creates a new provider registry
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]ProviderFactory),
	}
}

// Register registers a provider factory
func (r *ProviderRegistry) Register(name string, factory ProviderFactory) {
	r.providers[name] = factory
}

// GetProvider gets a provider by name
func (r *ProviderRegistry) GetProvider(name string, config map[string]string) (AIProvider, error) {
	factory, ok := r.providers[name]
	if !ok {
		return nil, &ErrProviderNotFound{Name: name}
	}

	return factory(config)
}

// ErrProviderNotFound is returned when a provider is not found
type ErrProviderNotFound struct {
	Name string
}

func (e *ErrProviderNotFound) Error() string {
	return "AI provider not found: " + e.Name
}
