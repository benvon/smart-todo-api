package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrRateLimited indicates the API rate limit was exceeded
	ErrRateLimited = errors.New("rate limited")
	// ErrQuotaExceeded indicates the API quota was exceeded
	ErrQuotaExceeded = errors.New("quota exceeded")
)

// APIError represents an error from the AI provider API
type APIError struct {
	Message     string
	Type        string
	Code        string
	StatusCode  int
	RetryAfter  *time.Duration
	IsPermanent bool // true for quota errors, false for rate limits
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (status %d, type %s): %s", e.StatusCode, e.Type, e.Message)
}

// IsRateLimitError checks if an error is a rate limit error
func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 429 && !apiErr.IsPermanent
	}

	// Check error message for rate limit indicators
	errStr := err.Error()
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "too many requests")
}

// IsQuotaError checks if an error is a quota exhaustion error
func IsQuotaError(err error) bool {
	if err == nil {
		return false
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsPermanent || apiErr.Code == "insufficient_quota"
	}

	// Check error message for quota indicators
	errStr := err.Error()
	return strings.Contains(errStr, "insufficient_quota") ||
		strings.Contains(errStr, "quota") ||
		strings.Contains(errStr, "billing")
}

// ExtractAPIError extracts API error details from an error
func ExtractAPIError(err error) *APIError {
	if err == nil {
		return nil
	}

	// Try to unwrap the error to find the underlying error
	errStr := err.Error()

	// Check if it's an OpenAI API error
	// OpenAI SDK errors often include JSON in the error message
	if strings.Contains(errStr, "429") {
		apiErr := &APIError{
			StatusCode: 429,
			Message:    errStr,
			Type:       "rate_limit_error",
		}

		// Try to parse JSON error details if present
		if jsonStart := strings.Index(errStr, "{"); jsonStart != -1 {
			jsonStr := errStr[jsonStart:]
			if jsonEnd := strings.LastIndex(jsonStr, "}"); jsonEnd != -1 {
				jsonStr = jsonStr[:jsonEnd+1]
				var errorData struct {
					Message string `json:"message"`
					Type    string `json:"type"`
					Code    string `json:"code"`
				}
				if json.Unmarshal([]byte(jsonStr), &errorData) == nil {
					apiErr.Message = errorData.Message
					apiErr.Type = errorData.Type
					apiErr.Code = errorData.Code

					// Check if it's quota exhaustion
					if errorData.Code == "insufficient_quota" {
						apiErr.IsPermanent = true
					}
				}
			}
		}

		// Estimate retry after time
		// Rate limits typically reset after 60 seconds
		retryAfter := 60 * time.Second
		apiErr.RetryAfter = &retryAfter

		// If quota exceeded, use much longer retry time (1 hour)
		if apiErr.IsPermanent {
			retryAfter = 1 * time.Hour
			apiErr.RetryAfter = &retryAfter
		}

		return apiErr
	}

	return nil
}

// GetRetryDelay calculates the delay before retrying based on error type
func GetRetryDelay(err error, attempt int) time.Duration {
	shift := retryShiftAmount(attempt)
	if IsQuotaError(err) {
		return capDuration(time.Hour*time.Duration(1<<shift), 24*time.Hour)
	}
	if IsRateLimitError(err) {
		delay := capDuration(60*time.Second*time.Duration(1<<shift), 15*time.Minute)
		if apiErr := ExtractAPIError(err); apiErr != nil && apiErr.RetryAfter != nil && *apiErr.RetryAfter > delay {
			delay = *apiErr.RetryAfter
		}
		return delay
	}
	return capDuration(5*time.Second*time.Duration(1<<shift), 5*time.Minute)
}

// retryShiftAmount caps attempt to [0, 20] and returns a shift in [0, 10] for exponential backoff.
func retryShiftAmount(attempt int) uint {
	if attempt < 0 {
		attempt = 0
	}
	if attempt > 20 {
		attempt = 20
	}
	if attempt > 10 {
		return 10
	}
	return uint(attempt)
}

func capDuration(d, max time.Duration) time.Duration {
	if d > max {
		return max
	}
	return d
}
