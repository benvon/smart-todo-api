package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/benvon/smart-todo/internal/models"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

const (
	// DefaultOpenAIModel is the default model to use
	DefaultOpenAIModel = "gpt-4o-mini"
	// DefaultOpenAIBaseURL is the default OpenAI API base URL
	DefaultOpenAIBaseURL = "https://api.openai.com/v1"
	// DefaultTimeout is the default timeout for API calls
	DefaultTimeout = 30 * time.Second

	// ErrNoChoicesInResponse is returned when the API response has no choices
	ErrNoChoicesInResponse = "no choices in response"
)

// OpenAIProvider implements the AIProvider interface using OpenAI's API
type OpenAIProvider struct {
	client openai.Client
	model  string
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(apiKey string, model string) *OpenAIProvider {
	if model == "" {
		model = DefaultOpenAIModel
	}

	httpClient := &http.Client{
		Timeout: DefaultTimeout,
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(DefaultOpenAIBaseURL),
		option.WithHTTPClient(httpClient),
	)

	return &OpenAIProvider{
		client: client,
		model:  model,
	}
}

// NewOpenAIProviderWithConfig creates a new OpenAI provider with custom configuration
func NewOpenAIProviderWithConfig(apiKey string, baseURL string, model string) *OpenAIProvider {
	if model == "" {
		model = DefaultOpenAIModel
	}
	if baseURL == "" {
		baseURL = DefaultOpenAIBaseURL
	}

	httpClient := &http.Client{
		Timeout: DefaultTimeout,
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
		option.WithHTTPClient(httpClient),
	)

	return &OpenAIProvider{
		client: client,
		model:  model,
	}
}

// AnalyzeTask analyzes a task and returns suggested tags and time horizon
func (p *OpenAIProvider) AnalyzeTask(ctx context.Context, text string, userContext *models.AIContext) ([]string, models.TimeHorizon, error) {
	return p.AnalyzeTaskWithDueDate(ctx, text, nil, userContext)
}

// AnalyzeTaskWithDueDate analyzes a task with an optional due date and returns suggested tags and time horizon
func (p *OpenAIProvider) AnalyzeTaskWithDueDate(ctx context.Context, text string, dueDate *time.Time, userContext *models.AIContext) ([]string, models.TimeHorizon, error) {
	// Build prompt with user context and due date if available
	prompt := p.buildAnalysisPrompt(text, dueDate, userContext)

	// Build messages
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a helpful assistant that analyzes todo items and suggests tags and time horizons. Respond with valid JSON only."),
		openai.UserMessage(prompt),
	}

	// Create request with JSON response format
	// Note: Some models (like o1) don't support custom temperature values
	// We'll omit temperature to use the model's default (typically 1.0)
	req := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(p.model),
		Messages: messages,
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &shared.ResponseFormatJSONObjectParam{},
		},
		// Temperature omitted - use model default to avoid "unsupported_value" errors
		// Some models only support their default temperature value
	}

	resp, err := p.client.Chat.Completions.New(ctx, req)
	if err != nil {
		// Wrap error with API error details for better handling
		if apiErr := ExtractAPIError(err); apiErr != nil {
			return nil, models.TimeHorizonSoon, fmt.Errorf("failed to analyze task: %w", apiErr)
		}
		return nil, models.TimeHorizonSoon, fmt.Errorf("failed to analyze task: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, models.TimeHorizonSoon, errors.New(ErrNoChoicesInResponse)
	}

	// Get content from first choice (Content is a string directly in the SDK)
	content := resp.Choices[0].Message.Content

	// Parse response - OpenAI returns JSON in content field
	var analysis struct {
		Tags        []string `json:"tags"`
		TimeHorizon string   `json:"time_horizon"`
	}

	// The response content should already be JSON if we requested json_object format
	if err := json.Unmarshal([]byte(content), &analysis); err != nil {
		// Fallback: try to extract JSON from markdown code blocks if needed
		if len(content) > 0 && content[0] != '{' {
			// Try to find JSON in the response
			start := bytes.Index([]byte(content), []byte("{"))
			end := bytes.LastIndex([]byte(content), []byte("}"))
			if start != -1 && end != -1 && end > start {
				content = content[start : end+1]
			}
		}

		if err := json.Unmarshal([]byte(content), &analysis); err != nil {
			return nil, models.TimeHorizonSoon, fmt.Errorf("failed to parse analysis response: %w", err)
		}
	}

	// Validate time horizon
	timeHorizon := models.TimeHorizon(analysis.TimeHorizon)
	switch timeHorizon {
	case models.TimeHorizonNext, models.TimeHorizonSoon, models.TimeHorizonLater:
		// Valid
	default:
		// Default to soon if invalid
		timeHorizon = models.TimeHorizonSoon
	}

	return analysis.Tags, timeHorizon, nil
}

// Chat handles a chat message and returns the AI response
func (p *OpenAIProvider) Chat(ctx context.Context, messages []ChatMessage, userContext *models.AIContext) (*ChatResponse, error) {
	// Build system message with user context if available
	systemContent := "You are a helpful assistant that helps users configure how their todos are analyzed and categorized. Be concise and helpful."
	if userContext != nil && userContext.ContextSummary != "" {
		systemContent += "\n\nUser context: " + userContext.ContextSummary
	}

	// Convert messages to OpenAI format
	openAIMessages := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages)+1)

	// Add system message
	openAIMessages = append(openAIMessages, openai.SystemMessage(systemContent))

	// Add conversation messages
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			openAIMessages = append(openAIMessages, openai.UserMessage(msg.Content))
		case "assistant":
			openAIMessages = append(openAIMessages, openai.AssistantMessage(msg.Content))
		default:
			openAIMessages = append(openAIMessages, openai.UserMessage(msg.Content))
		}
	}

	req := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(p.model),
		Messages: openAIMessages,
		// Temperature omitted - use model default to avoid "unsupported_value" errors
		// Some models only support their default temperature value
	}

	resp, err := p.client.Chat.Completions.New(ctx, req)
	if err != nil {
		// Wrap error with API error details for better handling
		if apiErr := ExtractAPIError(err); apiErr != nil {
			return nil, fmt.Errorf("failed to chat: %w", apiErr)
		}
		return nil, fmt.Errorf("failed to chat: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New(ErrNoChoicesInResponse)
	}

	// Get content from first choice (Content is a string directly in the SDK)
	content := resp.Choices[0].Message.Content

	return &ChatResponse{
		Message:     content,
		NeedsUpdate: true, // Always update context after chat
	}, nil
}

// SummarizeContext summarizes a conversation history into a context summary
func (p *OpenAIProvider) SummarizeContext(ctx context.Context, conversationHistory []ChatMessage) (string, error) {
	// Build summary prompt
	prompt := "Summarize the following conversation into a concise context that can be used to better understand the user's preferences for todo categorization. Focus on key preferences and patterns.\n\nConversation:\n"

	for _, msg := range conversationHistory {
		prompt += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a helpful assistant that creates concise summaries of conversations. Focus on extracting user preferences and patterns."),
		openai.UserMessage(prompt),
	}

	req := openai.ChatCompletionNewParams{
		Model:     shared.ChatModel(p.model),
		Messages:  messages,
		MaxTokens: openai.Int(500), // Limit summary length
		// Temperature omitted - use model default to avoid "unsupported_value" errors
		// Some models only support their default temperature value
	}

	resp, err := p.client.Chat.Completions.New(ctx, req)
	if err != nil {
		// Wrap error with API error details for better handling
		if apiErr := ExtractAPIError(err); apiErr != nil {
			return "", fmt.Errorf("failed to summarize context: %w", apiErr)
		}
		return "", fmt.Errorf("failed to summarize context: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", errors.New(ErrNoChoicesInResponse)
	}

	// Get content from first choice (Content is a string directly in the SDK)
	content := resp.Choices[0].Message.Content

	return content, nil
}

// buildAnalysisPrompt builds the prompt for task analysis
func (p *OpenAIProvider) buildAnalysisPrompt(text string, dueDate *time.Time, userContext *models.AIContext) string {
	prompt := fmt.Sprintf(`Analyze the following todo item and suggest:
1. Relevant tags (as a JSON array of strings)
2. Time horizon: "next", "soon", or "later"

Todo item: "%s"`, text)

	// Include due date information if available
	if dueDate != nil {
		now := time.Now()
		timeUntil := dueDate.Sub(now)
		daysUntil := int(timeUntil.Hours() / 24)
		
		prompt += fmt.Sprintf("\n\nDue date: %s (in %d days)", dueDate.Format(time.RFC3339), daysUntil)
		
		// Provide guidance based on due date
		if daysUntil < 0 {
			prompt += "\nNote: This item is overdue."
		} else if daysUntil == 0 {
			prompt += "\nNote: This item is due today."
		} else if daysUntil <= 1 {
			prompt += "\nNote: This item is due very soon (today or tomorrow)."
		} else if daysUntil <= 7 {
			prompt += fmt.Sprintf("\nNote: This item is due in %d days (within a week).", daysUntil)
		}
	}

	prompt += `

Respond with a JSON object in this format:
{
  "tags": ["tag1", "tag2"],
  "time_horizon": "next" | "soon" | "later"
}

Guidelines:
- "next": Items that need immediate attention or should be done very soon (including items due today or within 1-2 days)
- "soon": Items that should be done in the near future (typically within a week, or based on due date)
- "later": Items that can wait or are not urgent (typically more than a week away)

Use the due date as a strong signal for time horizon categorization. Items with earlier due dates should typically have higher priority time horizons.

Return only valid JSON.`

	if userContext != nil && userContext.ContextSummary != "" {
		prompt += "\n\nUser preferences: " + userContext.ContextSummary
	}

	return prompt
}

// RegisterOpenAI registers the OpenAI provider with the registry
func RegisterOpenAI(registry *ProviderRegistry) {
	registry.Register("openai", func(config map[string]string) (AIProvider, error) {
		apiKey, ok := config["api_key"]
		if !ok || apiKey == "" {
			return nil, fmt.Errorf("openai api_key is required")
		}

		model := config["model"]
		baseURL := config["base_url"]

		return NewOpenAIProviderWithConfig(apiKey, baseURL, model), nil
	})
}
