package middleware

import (
	"encoding/json"
	"net/http"
	"time"

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
					// Log panic details server-side but don't expose to client
					logger.Error("panic_recovered",
						zap.Any("error", err),
						zap.String("path", r.URL.Path),
						zap.String("method", r.Method),
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
		logger.Error("failed_to_encode_error_response",
			zap.Error(err),
			zap.Int("status_code", status),
			zap.String("path", r.URL.Path),
		)
	}
}
