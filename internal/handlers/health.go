package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/benvon/smart-todo/internal/database"
)

// HealthChecker handles health check requests
type HealthChecker struct {
	db *database.DB
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(db *database.DB) *HealthChecker {
	return &HealthChecker{db: db}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// HealthCheck handles the /healthz endpoint
func (h *HealthChecker) HealthCheck(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if mode == "extended" {
		checks := make(map[string]string)
		
		// Check database connection
		if err := h.checkDatabase(r.Context()); err != nil {
			response.Status = "unhealthy"
			checks["database"] = "unhealthy: " + err.Error()
		} else {
			checks["database"] = "healthy"
		}
		
		// Future checks can be added here:
		// - checks["queue"] = h.checkQueue()
		// - checks["cache"] = h.checkCache()
		// etc.
		
		response.Checks = checks
		
		statusCode := http.StatusOK
		if response.Status == "unhealthy" {
			statusCode = http.StatusServiceUnavailable
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Basic mode - just return that the server is running
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// checkDatabase verifies the database connection
func (h *HealthChecker) checkDatabase(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		return err
	}

	return nil
}
