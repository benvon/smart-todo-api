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
	// Cap attempt to prevent integer overflow (max 30 for uint32, max 63 for uint64)
	// Using 20 as safe maximum for reasonable delays
	maxAttempt := attempt
	if maxAttempt > 20 {
		maxAttempt = 20
	}
	if maxAttempt < 0 {
		maxAttempt = 0
	}

	// Calculate safe shift amount in range [0, 10] and convert to uint safely
	// This prevents integer overflow when converting to uint
	// Explicitly check range before conversion so static analysis tools can verify safety
	var shiftAmountUint uint
	if maxAttempt < 0 {
		shiftAmountUint = 0
	} else if maxAttempt > 10 {
		shiftAmountUint = 10
	} else {
		// maxAttempt is in range [0, 10], safe to convert
		shiftAmountUint = uint(maxAttempt)
	}

	if IsQuotaError(err) {
		// Quota errors: exponential backoff starting at 1 hour
		// shiftAmountUint is already in safe range [0, 10]
		delay := time.Hour * time.Duration(1<<shiftAmountUint)
		if delay > 24*time.Hour {
			delay = 24 * time.Hour
		}
		return delay
	}

	if IsRateLimitError(err) {
		// Rate limit errors: exponential backoff starting at 60 seconds
		// shiftAmountUint is already in safe range [0, 10]
		delay := 60 * time.Second * time.Duration(1<<shiftAmountUint)
		if delay > 15*time.Minute {
			delay = 15 * time.Minute
		}

		// Try to extract retry-after from error
		if apiErr := ExtractAPIError(err); apiErr != nil && apiErr.RetryAfter != nil {
			if *apiErr.RetryAfter > delay {
				delay = *apiErr.RetryAfter
			}
		}

		return delay
	}

	// Default: exponential backoff starting at 5 seconds
	// shiftAmountUint is already in safe range [0, 10]
	delay := 5 * time.Second * time.Duration(1<<shiftAmountUint)
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	return delay
}
