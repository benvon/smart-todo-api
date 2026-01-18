package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
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
func ErrorHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				respondErrorJSON(w, r, http.StatusInternalServerError, "Internal Server Error", "An unexpected error occurred")
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// respondErrorJSON sends an error JSON response
func respondErrorJSON(w http.ResponseWriter, r *http.Request, status int, errorType, message string) {
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
		log.Printf("Failed to encode error response: %v", err)
	}
}
