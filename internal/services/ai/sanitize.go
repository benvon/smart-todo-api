package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
)

// Context key types for logging (to avoid collisions with string keys)
type contextKey string

const (
	userIDContextKey    contextKey = "user_id"
	todoIDContextKey    contextKey = "todo_id"
	requestIDContextKey contextKey = "request_id"
)

// UserIDContextKey returns the context key for user ID
func UserIDContextKey() contextKey {
	return userIDContextKey
}

// TodoIDContextKey returns the context key for todo ID
func TodoIDContextKey() contextKey {
	return todoIDContextKey
}

// RequestIDContextKey returns the context key for request ID
func RequestIDContextKey() contextKey {
	return requestIDContextKey
}

const (
	// MaxPreviewLength is the maximum length for preview strings in logs
	MaxPreviewLength = 200
	// RedactedValue is the value used to replace sensitive data
	RedactedValue = "[REDACTED]"
)

// SanitizeAPIKey sanitizes an API key for logging
func SanitizeAPIKey(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	if len(apiKey) <= 8 {
		return RedactedValue
	}
	// Show first 4 and last 4 characters, redact the middle
	return apiKey[:4] + RedactedValue + apiKey[len(apiKey)-4:]
}

// SanitizePrompt creates a safe preview of a prompt for logging
func SanitizePrompt(prompt string, fullLog bool) string {
	if prompt == "" {
		return ""
	}
	if fullLog {
		return prompt
	}
	// Return preview with truncation indicator
	if len(prompt) <= MaxPreviewLength {
		return prompt
	}
	return prompt[:MaxPreviewLength] + "..."
}

// SanitizeResponse creates a safe preview of a response for logging
func SanitizeResponse(response string, fullLog bool) string {
	if response == "" {
		return ""
	}
	if fullLog {
		return response
	}
	// Return preview with truncation indicator
	if len(response) <= MaxPreviewLength {
		return response
	}
	return response[:MaxPreviewLength] + "..."
}

// HashUserID creates a hash of a user ID for logging (optional, for additional privacy)
func HashUserID(userID string) string {
	if userID == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(userID))
	return hex.EncodeToString(hash[:])[:16] // Return first 16 chars of hash
}

// TruncateString safely truncates a string to max length
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// SanitizeMessages creates sanitized previews of messages for logging
func SanitizeMessages(messages []string, fullLog bool) []string {
	if fullLog {
		return messages
	}
	sanitized := make([]string, 0, len(messages))
	for _, msg := range messages {
		sanitized = append(sanitized, SanitizePrompt(msg, false))
	}
	return sanitized
}

// ExtractRequestID extracts a request ID from context if available
func ExtractRequestID(ctx context.Context) string {
	// Check if context has request ID (could be added via middleware)
	if reqID := ctx.Value(requestIDContextKey); reqID != nil {
		if id, ok := reqID.(string); ok {
			return id
		}
	}
	return ""
}

// ExtractUserID extracts a user ID from context if available (handles UUID)
func ExtractUserID(ctx context.Context) string {
	if userID := ctx.Value(userIDContextKey); userID != nil {
		// Handle UUID type
		if id, ok := userID.(interface{ String() string }); ok {
			return id.String()
		}
		// Handle string type
		if id, ok := userID.(string); ok {
			return id
		}
	}
	return ""
}

// ExtractTodoID extracts a todo ID from context if available (handles UUID)
func ExtractTodoID(ctx context.Context) string {
	if todoID := ctx.Value(todoIDContextKey); todoID != nil {
		// Handle UUID type
		if id, ok := todoID.(interface{ String() string }); ok {
			return id.String()
		}
		// Handle string type
		if id, ok := todoID.(string); ok {
			return id
		}
	}
	return ""
}
