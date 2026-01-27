package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
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

	// DefaultMaxTagsInPrompt is the default maximum number of tags to include in the prompt
	DefaultMaxTagsInPrompt = 50
	// DefaultMaxTagTokens is the default maximum number of tokens for the tag list (roughly 30% of typical context)
	DefaultMaxTagTokens = 500

	// TagScoreFrequencyWeight is the weight given to tag frequency in the scoring algorithm
	TagScoreFrequencyWeight = 0.7
	// TagScoreSimilarityWeight is the weight given to semantic similarity in the scoring algorithm
	TagScoreSimilarityWeight = 0.3
	// TagScoreSimilarityMultiplier scales similarity scores to be comparable with frequency scores
	TagScoreSimilarityMultiplier = 100

	// ErrNoChoicesInResponse is returned when the API response has no choices
	ErrNoChoicesInResponse = "no choices in response"
)

// OpenAIProvider implements the AIProvider interface using OpenAI's API
type OpenAIProvider struct {
	client          openai.Client
	model           string
	maxTagsInPrompt int
	maxTagTokens    int
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
		client:          client,
		model:           model,
		maxTagsInPrompt: DefaultMaxTagsInPrompt,
		maxTagTokens:    DefaultMaxTagTokens,
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
		client:          client,
		model:           model,
		maxTagsInPrompt: DefaultMaxTagsInPrompt,
		maxTagTokens:    DefaultMaxTagTokens,
	}
}

// AnalyzeTask analyzes a task and returns suggested tags and time horizon
func (p *OpenAIProvider) AnalyzeTask(ctx context.Context, text string, userContext *models.AIContext) ([]string, models.TimeHorizon, error) {
	// Use current time as creation time when not provided
	return p.AnalyzeTaskWithDueDate(ctx, text, nil, time.Now(), userContext, nil)
}

// AnalyzeTaskWithDueDate analyzes a task with an optional due date and creation time, returns suggested tags and time horizon
// tagStats is optional tag statistics to guide tag selection (prefer existing tags)
func (p *OpenAIProvider) AnalyzeTaskWithDueDate(ctx context.Context, text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext, tagStats *models.TagStatistics) ([]string, models.TimeHorizon, error) {
	// Build prompt with user context, due date, creation time, and tag statistics
	prompt := p.buildAnalysisPrompt(text, dueDate, createdAt, userContext, tagStats)

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

// estimateTokenCount provides a rough estimate of token count for a string
// This uses a simple heuristic: ~4 characters per token (common for English text)
// For more accurate counting, consider using a tokenizer library
func estimateTokenCount(text string) int {
	// Handle empty text explicitly
	if len(text) == 0 {
		return 0
	}
	return len(text) / 4
}

// calculateStringSimilarity calculates a simple similarity score between two strings
// Returns a score between 0 and 1, where 1 means identical
// This uses a basic approach: counts common words (case-insensitive)
func calculateStringSimilarity(s1, s2 string) float64 {
	// Convert to lowercase and split into words
	words1 := strings.Fields(strings.ToLower(s1))
	words2 := strings.Fields(strings.ToLower(s2))

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	// Create a set of words from s2 for faster lookup
	word2Set := make(map[string]bool)
	for _, word := range words2 {
		word2Set[word] = true
	}

	// Count common words
	commonCount := 0
	for _, word := range words1 {
		if word2Set[word] {
			commonCount++
		}
	}

	// Use Jaccard similarity: intersection / union
	union := len(words1) + len(words2) - commonCount
	if union == 0 {
		return 0.0
	}

	return float64(commonCount) / float64(union)
}

// selectTagsForPrompt selects tags to include in the prompt using a smart algorithm
// It combines frequently used tags with tags semantically similar to the todo text
func (p *OpenAIProvider) selectTagsForPrompt(tagStats map[string]models.TagStats, todoText string) []string {
	if len(tagStats) == 0 {
		return nil
	}

	// Use defaults if not configured
	maxTags := p.maxTagsInPrompt
	if maxTags == 0 {
		maxTags = DefaultMaxTagsInPrompt
	}
	maxTokens := p.maxTagTokens
	if maxTokens == 0 {
		maxTokens = DefaultMaxTagTokens
	}

	// Create tag list with scores
	type tagScore struct {
		tag        string
		total      int
		similarity float64
		score      float64
	}

	tagList := make([]tagScore, 0, len(tagStats))
	for tag, stats := range tagStats {
		// Calculate similarity between tag and todo text
		similarity := calculateStringSimilarity(tag, todoText)

		// Combined score: frequency weight + similarity weight
		// Similarity is multiplied to make it comparable with frequency scores
		score := float64(stats.Total)*TagScoreFrequencyWeight + similarity*TagScoreSimilarityMultiplier*TagScoreSimilarityWeight

		tagList = append(tagList, tagScore{
			tag:        tag,
			total:      stats.Total,
			similarity: similarity,
			score:      score,
		})
	}

	// Sort by combined score (descending)
	sort.Slice(tagList, func(i, j int) bool {
		return tagList[i].score > tagList[j].score
	})

	// Select tags up to limits
	selectedTags := make([]string, 0, maxTags)
	estimatedTokens := 0

	for _, entry := range tagList {
		// Build tag entry text to estimate its token count
		tagText := fmt.Sprintf("- %s (used %d times)\n", entry.tag, entry.total)
		entryTokens := estimateTokenCount(tagText)

		// Check if adding this tag would exceed token limit
		if estimatedTokens+entryTokens > maxTokens {
			break
		}

		// Check if we've reached max tags limit
		if len(selectedTags) >= maxTags {
			break
		}

		selectedTags = append(selectedTags, entry.tag)
		estimatedTokens += entryTokens
	}

	return selectedTags
}

// buildAnalysisPrompt builds the prompt for task analysis with time context and tag statistics
func (p *OpenAIProvider) buildAnalysisPrompt(text string, dueDate *time.Time, createdAt time.Time, userContext *models.AIContext, tagStats *models.TagStatistics) string {
	now := time.Now()

	prompt := fmt.Sprintf(`Analyze the following todo item and suggest:
1. Relevant tags (as a JSON array of strings)
2. Time horizon: "next", "soon", or "later"

Todo item: "%s"`, text)

	// Include time context for better understanding of relative time expressions
	prompt += "\n\nTime context:"
	prompt += fmt.Sprintf("\n- Current date and time: %s", now.Format(time.RFC3339))
	prompt += fmt.Sprintf("\n- Todo created/entered at: %s", createdAt.Format(time.RFC3339))

	// Calculate time since creation
	timeSinceCreation := now.Sub(createdAt)
	daysSinceCreation := int(timeSinceCreation.Hours() / 24)
	switch daysSinceCreation {
	case 0:
		prompt += "\n- This todo was entered today."
	case 1:
		prompt += "\n- This todo was entered yesterday."
	default:
		prompt += fmt.Sprintf("\n- This todo was entered %d days ago.", daysSinceCreation)
	}

	// Include due date information if available
	if dueDate != nil {
		timeUntil := dueDate.Sub(now)
		daysUntil := int(timeUntil.Hours() / 24)

		// Check if due date is date-only (midnight)
		isDateOnly := dueDate.Hour() == 0 && dueDate.Minute() == 0 && dueDate.Second() == 0 && dueDate.Nanosecond() == 0

		if isDateOnly {
			prompt += fmt.Sprintf("\n\nDue date: %s (date only, no specific time)", dueDate.Format("2006-01-02"))
		} else {
			prompt += fmt.Sprintf("\n\nDue date: %s (specific time)", dueDate.Format(time.RFC3339))
		}

		prompt += fmt.Sprintf(" (in %d days)", daysUntil)

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

When interpreting relative time expressions in the todo text (like "this weekend", "soon", "next week"), consider when the todo was created. For example, if a todo says "this weekend" and it was created on Monday, "this weekend" refers to the upcoming weekend. If it was created on Saturday, it might refer to today or tomorrow.

Return only valid JSON.`

	// Include tag statistics to guide tag selection
	if tagStats != nil && len(tagStats.TagStats) > 0 {
		prompt += "\n\nExisting tags (prefer reusing these when semantically similar):"

		// Use smart tag selection algorithm
		selectedTags := p.selectTagsForPrompt(tagStats.TagStats, text)

		for _, tag := range selectedTags {
			stats := tagStats.TagStats[tag]
			prompt += fmt.Sprintf("\n- %s (used %d times", tag, stats.Total)
			if stats.AI > 0 || stats.User > 0 {
				prompt += fmt.Sprintf(", %d AI-generated, %d user-defined", stats.AI, stats.User)
			}
			prompt += ")"
		}

		prompt += "\n\nTag selection guidance:"
		prompt += "\n- Prefer reusing existing tags when they are semantically similar or closely related to the todo item"
		prompt += "\n- Only create new tags if no existing tag is a good match (consider synonyms, related concepts, and variations)"
		prompt += "\n- When an existing tag is close enough, use it rather than creating a new one"
		prompt += "\n- This helps maintain consistency and reduces tag proliferation"
	}

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
