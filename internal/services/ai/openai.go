package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	// DefaultMaxPromptTags is the default maximum number of tags to include in prompts
	DefaultMaxPromptTags = 30
	// DefaultTagsTokenPercent is the default percentage of available tokens to allocate for tags
	DefaultTagsTokenPercent = 20
	
	// Token estimation constants
	// CharsPerToken is a rough estimate for token counting (1 token ≈ 4 characters for English text)
	CharsPerToken = 4
	// MaxContextTokens is a conservative estimate of model context limits
	// Set to 8000 to work safely with most models (GPT-3.5/4 have 8192, but we leave room for response)
	MaxContextTokens = 8000
	
	// Tag scoring constants
	// RelevanceWeight is the weight given to tag relevance in scoring (0.0-1.0)
	RelevanceWeight = 0.7
	// FrequencyWeight is the weight given to tag frequency in scoring (0.0-1.0)
	FrequencyWeight = 0.3
	// FrequencyNormalizer is used to normalize frequency scores to a 0-1 range
	FrequencyNormalizer = 100.0

	// ErrNoChoicesInResponse is returned when the API response has no choices
	ErrNoChoicesInResponse = "no choices in response"
)

// OpenAIProvider implements the AIProvider interface using OpenAI's API
type OpenAIProvider struct {
	client           openai.Client
	model            string
	maxPromptTags    int
	tagsTokenPercent int
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
		client:           client,
		model:            model,
		maxPromptTags:    DefaultMaxPromptTags,
		tagsTokenPercent: DefaultTagsTokenPercent,
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
		client:           client,
		model:            model,
		maxPromptTags:    DefaultMaxPromptTags,
		tagsTokenPercent: DefaultTagsTokenPercent,
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

		// Select tags using smart algorithm that considers both frequency and relevance
		selectedTags := p.selectTagsForPrompt(text, tagStats, prompt)

		for _, entry := range selectedTags {
			stats := tagStats.TagStats[entry.tag]
			prompt += fmt.Sprintf("\n- %s (used %d times", entry.tag, stats.Total)
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

// tagEntry holds tag information for sorting and selection
type tagEntry struct {
	tag   string
	total int
	score float64 // Combined score for ranking
}

// selectTagsForPrompt selects tags to include in the prompt using a smart algorithm
// that considers both frequency and relevance to the todo text, with token counting
func (p *OpenAIProvider) selectTagsForPrompt(todoText string, tagStats *models.TagStatistics, currentPrompt string) []tagEntry {
	// Use defaults if not set (for backward compatibility and tests)
	maxPromptTags := p.maxPromptTags
	if maxPromptTags == 0 {
		maxPromptTags = DefaultMaxPromptTags
	}
	tagsTokenPercent := p.tagsTokenPercent
	if tagsTokenPercent == 0 {
		tagsTokenPercent = DefaultTagsTokenPercent
	}

	// Calculate available tokens for tags based on configured percentage
	// Rough estimate: 1 token ~= CharsPerToken characters for English text
	currentTokens := len(currentPrompt) / CharsPerToken
	
	// Use conservative context limit to work safely with most models
	// Ensure we don't have negative available tokens if prompt is already large
	availableTokens := MaxContextTokens - currentTokens
	if availableTokens < 0 {
		availableTokens = 0
	}
	maxTagTokens := availableTokens * tagsTokenPercent / 100

	// Build tag list with relevance scores
	tagList := make([]tagEntry, 0, len(tagStats.TagStats))
	todoLower := strings.ToLower(todoText)

	for tag, stats := range tagStats.TagStats {
		// Calculate relevance score
		relevance := p.calculateTagRelevance(tag, todoLower)

		// Calculate frequency score (normalize by max possible)
		// Higher usage count means higher frequency score
		frequencyScore := float64(stats.Total)

		// Combined score: RelevanceWeight * relevance + FrequencyWeight * frequency
		// Prioritize relevance but still favor frequently used tags
		combinedScore := (RelevanceWeight * relevance) + (FrequencyWeight * frequencyScore/FrequencyNormalizer)

		tagList = append(tagList, tagEntry{
			tag:   tag,
			total: stats.Total,
			score: combinedScore,
		})
	}

	// Sort by combined score (descending) using Go's built-in sort
	sort.Slice(tagList, func(i, j int) bool {
		return tagList[i].score > tagList[j].score
	})

	// Select tags within token budget and count limits
	selectedTags := make([]tagEntry, 0, maxPromptTags)
	usedTokens := 0

	for _, entry := range tagList {
		// Get stats once to avoid repeated map lookups
		stats := tagStats.TagStats[entry.tag]
		
		// Estimate tokens for this tag entry
		// Format: "- tagname (used N times, X AI-generated, Y user-defined)\n"
		tagEntryText := fmt.Sprintf("- %s (used %d times, %d AI-generated, %d user-defined)\n",
			entry.tag, entry.total, stats.AI, stats.User)
		tagTokens := len(tagEntryText) / CharsPerToken

		// Check if adding this tag would exceed limits
		if len(selectedTags) >= maxPromptTags {
			break
		}
		if usedTokens+tagTokens > maxTagTokens {
			break
		}

		selectedTags = append(selectedTags, entry)
		usedTokens += tagTokens
	}

	return selectedTags
}

// calculateTagRelevance calculates how relevant a tag is to the todo text
// Returns a score between 0 and 1
func (p *OpenAIProvider) calculateTagRelevance(tag string, todoTextLower string) float64 {
	tagLower := strings.ToLower(tag)

	// Exact match (case-insensitive)
	if strings.Contains(todoTextLower, tagLower) {
		return 1.0
	}

	// Check for partial matches (word-based)
	tagWords := strings.Fields(tagLower)
	todoWords := strings.Fields(todoTextLower)

	matchCount := 0
	for _, tagWord := range tagWords {
		for _, todoWord := range todoWords {
			// Exact word match
			if tagWord == todoWord {
				matchCount++
				break
			}
			// Prefix match (e.g., "shop" matches "shopping")
			if strings.HasPrefix(todoWord, tagWord) || strings.HasPrefix(tagWord, todoWord) {
				matchCount++
				break
			}
		}
	}

	if len(tagWords) > 0 {
		wordMatchScore := float64(matchCount) / float64(len(tagWords))
		if wordMatchScore > 0 {
			return 0.5 + (wordMatchScore * 0.5) // Score between 0.5 and 1.0 for partial matches
		}
	}

	// No match - return base score (still might be selected based on frequency)
	return 0.1
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

		provider := NewOpenAIProviderWithConfig(apiKey, baseURL, model)

		// Apply optional configuration
		if maxTags := config["max_prompt_tags"]; maxTags != "" {
			if val, err := parseIntOrDefault(maxTags, DefaultMaxPromptTags); err == nil {
				provider.maxPromptTags = val
			} else {
				log.Printf("Warning: invalid AI_MAX_PROMPT_TAGS value '%s': %v. Using default: %d", maxTags, err, DefaultMaxPromptTags)
			}
		}

		if tokenPercent := config["tags_token_percent"]; tokenPercent != "" {
			if val, err := parseIntOrDefault(tokenPercent, DefaultTagsTokenPercent); err == nil {
				provider.tagsTokenPercent = val
			} else {
				log.Printf("Warning: invalid AI_TAGS_TOKEN_PERCENT value '%s': %v. Using default: %d", tokenPercent, err, DefaultTagsTokenPercent)
			}
		}

		return provider, nil
	})
}

// parseIntOrDefault parses a string to an int, returning the default if parsing fails
func parseIntOrDefault(s string, defaultVal int) (int, error) {
	if s == "" {
		return defaultVal, nil
	}
	var result int
	n, err := fmt.Sscanf(s, "%d", &result)
	if err != nil || n != 1 {
		return defaultVal, fmt.Errorf("invalid integer: %s", s)
	}
	return result, nil
}
