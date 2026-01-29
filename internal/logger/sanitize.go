package logger

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxPathLength is the maximum length for URL paths in logs
	MaxPathLength = 500
	// MaxUserIDLength is the maximum length for user IDs in logs (UUIDs are 36 chars)
	MaxUserIDLength = 128
	// MaxErrorMessageLength is the maximum length for error messages in logs
	MaxErrorMessageLength = 1000
	// MaxGeneralStringLength is the maximum length for general strings in logs
	MaxGeneralStringLength = 2000
	// MaxDebugContentLength is the maximum length for debug content (prompts/responses)
	MaxDebugContentLength = 10000
)

// SanitizePath sanitizes a URL path for safe logging
// Removes control characters, truncates to MaxPathLength, and validates UTF-8
func SanitizePath(path string) string {
	if path == "" {
		return ""
	}

	// Validate and fix UTF-8 encoding
	if !utf8.ValidString(path) {
		path = strings.ToValidUTF8(path, "")
	}

	// Remove control characters (except space, tab, newline, carriage return)
	var builder strings.Builder
	builder.Grow(len(path))
	for _, r := range path {
		// Allow printable characters, space, tab, newline, carriage return
		if unicode.IsPrint(r) || r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			builder.WriteRune(r)
		}
	}
	path = builder.String()

	// Truncate to max length
	if len(path) > MaxPathLength {
		path = path[:MaxPathLength] + "..."
	}

	return path
}

// SanitizeString sanitizes a general string for safe logging
// Removes control characters, truncates to maxLength, and validates UTF-8
func SanitizeString(s string, maxLength int) string {
	if s == "" {
		return ""
	}
	if maxLength <= 0 {
		maxLength = MaxGeneralStringLength
	}
	s = sanitizeFilterRunes(s)
	if len(s) > maxLength {
		s = s[:maxLength] + "..."
	}
	return s
}

// sanitizeFilterRunes validates UTF-8 and removes control characters (keeps printable, space, tab, newline, CR).
func sanitizeFilterRunes(s string) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
	}
	var builder strings.Builder
	builder.Grow(len(s))
	for _, r := range s {
		if unicode.IsPrint(r) || r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

// SanitizeError sanitizes an error message for safe logging
func SanitizeError(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()
	return SanitizeString(errStr, MaxErrorMessageLength)
}

// SanitizeErrorString sanitizes an error string for safe logging
func SanitizeErrorString(errStr string) string {
	return SanitizeString(errStr, MaxErrorMessageLength)
}

// SanitizeUserID sanitizes a user ID for safe logging
func SanitizeUserID(userID string) string {
	return SanitizeString(userID, MaxUserIDLength)
}

// SanitizeDebugContent sanitizes debug content (prompts/responses) for safe logging
// Even in debug mode, we should sanitize to prevent log injection and limit size
func SanitizeDebugContent(content string) string {
	return SanitizeString(content, MaxDebugContentLength)
}
