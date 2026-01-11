package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

// Response represents a standard API response
type Response struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Path      string      `json:"path"`
}

// ErrorResponse represents an error API response
type ErrorResponse struct {
	Success   bool      `json:"success"`
	Error     string    `json:"error"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Path      string    `json:"path"`
}

// RegisterRoutes registers all HTTP routes
func RegisterRoutes(mux *http.ServeMux) {
	// API routes
	mux.HandleFunc("/api/v1/hello", handleHello)
	mux.HandleFunc("/api/v1/status", handleStatus)
	
	// Catch-all handler for undefined routes
	mux.HandleFunc("/", handleNotFound)
}

// handleHello handles the hello endpoint
func handleHello(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", r.URL.Path)
		return
	}

	response := Response{
		Success:   true,
		Message:   "Hello from AI Code Template Go!",
		Data: map[string]interface{}{
			"service": "ai-code-template-go",
			"time":    time.Now().Format(time.RFC3339),
		},
		Timestamp: time.Now(),
		Path:      r.URL.Path,
	}

	sendJSONResponse(w, http.StatusOK, response)
}

// handleStatus handles the status endpoint
func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", r.URL.Path)
		return
	}

	response := Response{
		Success: true,
		Message: "Service is running",
		Data: map[string]interface{}{
			"status":    "healthy",
			"uptime":    "running",
			"timestamp": time.Now().Format(time.RFC3339),
		},
		Timestamp: time.Now(),
		Path:      r.URL.Path,
	}

	sendJSONResponse(w, http.StatusOK, response)
}

// handleNotFound handles undefined routes
func handleNotFound(w http.ResponseWriter, r *http.Request) {
	sendErrorResponse(w, http.StatusNotFound, "Endpoint not found", r.URL.Path)
}

// sendJSONResponse sends a JSON response with the given status code and data
func sendJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// sendErrorResponse sends an error response with the given status code and message
func sendErrorResponse(w http.ResponseWriter, statusCode int, message, path string) {
	response := ErrorResponse{
		Success:   false,
		Error:     http.StatusText(statusCode),
		Message:   message,
		Timestamp: time.Now(),
		Path:      path,
	}

	sendJSONResponse(w, statusCode, response)
}
