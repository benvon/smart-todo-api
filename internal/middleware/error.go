package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	logpkg "github.com/benvon/smart-todo/internal/logger"
	"go.uber.org/zap"
)

// ErrorResponse represents an error response
type ErrorResponse struct {
	Success   bool   `json:"success"`
	Error     string `json:"error"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Path      string `json:"path"`
}

// ErrorHandler creates error handling middleware
func ErrorHandler(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Convert panic value to string safely
					var errStr string
					if err != nil {
						errStr = fmt.Sprintf("%v", err)
					}

					// Log panic details server-side but don't expose to client
					// Include stack trace for debugging
					logger.Error("panic_recovered",
						zap.String("operation", "http_request_handler"),
						zap.String("error", logpkg.SanitizeErrorString(errStr)),
						zap.String("path", logpkg.SanitizePath(r.URL.Path)),
						zap.String("method", r.Method),
						zap.String("stacktrace", string(debug.Stack())),
					)
					respondErrorJSON(w, r, http.StatusInternalServerError, "Internal Server Error", "An unexpected error occurred", logger)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// respondErrorJSON sends an error JSON response
func respondErrorJSON(w http.ResponseWriter, r *http.Request, status int, errorType, message string, logger *zap.Logger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := ErrorResponse{
		Success:   false,
		Error:     errorType,
		Message:   message,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Path:      r.URL.Path,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Use fallback logging to avoid recursion if logger fails
		// Write directly to response writer as last resort
		if _, writeErr := w.Write([]byte(`{"success":false,"error":"Failed to encode error response"}`)); writeErr != nil {
			// If even writing fails, there's nothing more we can do
			_ = writeErr
		}
		logger.Error("failed_to_encode_error_response",
			zap.String("operation", "respond_error_json"),
			zap.String("error", logpkg.SanitizeError(err)),
			zap.Int("status_code", status),
			zap.String("path", logpkg.SanitizePath(r.URL.Path)),
			zap.String("error_type", errorType),
		)
	}
}
