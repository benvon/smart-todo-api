package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/queue"
)

// Pinger is implemented by dependencies that support a connectivity check (e.g. Redis).
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthChecker handles health check requests
type HealthChecker struct {
	db          *database.DB
	redisPinger Pinger
	jobQueue    queue.JobQueue
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(db *database.DB) *HealthChecker {
	return &HealthChecker{db: db}
}

// NewHealthCheckerWithDeps creates a new health checker with Redis and RabbitMQ dependencies
func NewHealthCheckerWithDeps(db *database.DB, redisPinger Pinger, jobQueue queue.JobQueue) *HealthChecker {
	return &HealthChecker{
		db:          db,
		redisPinger: redisPinger,
		jobQueue:    jobQueue,
	}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// runExtendedChecks runs database, Redis, and RabbitMQ checks and returns checks map and overall status.
func (h *HealthChecker) runExtendedChecks(ctx context.Context) (map[string]string, string) {
	checks := make(map[string]string)
	status := "healthy"

	if err := h.checkDatabase(ctx); err != nil {
		status = "unhealthy"
		checks["database"] = "unhealthy"
	} else {
		checks["database"] = "healthy"
	}

	if h.redisPinger != nil {
		if err := h.checkRedis(ctx); err != nil {
			status = "unhealthy"
			checks["redis"] = "unhealthy"
		} else {
			checks["redis"] = "healthy"
		}
	} else {
		checks["redis"] = "not configured"
	}

	if h.jobQueue != nil {
		if err := h.checkRabbitMQ(ctx); err != nil {
			status = "unhealthy"
			checks["rabbitmq"] = "unhealthy"
		} else {
			checks["rabbitmq"] = "healthy"
		}
	} else {
		checks["rabbitmq"] = "not configured"
	}

	return checks, status
}

// writeHealthResponse writes a HealthResponse with the given status and optional checks.
func (h *HealthChecker) writeHealthResponse(w http.ResponseWriter, status string, checks map[string]string) {
	code := http.StatusOK
	if status == "unhealthy" {
		code = http.StatusServiceUnavailable
	}
	resp := HealthResponse{
		Status:    status,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(resp)
}

// HealthCheck handles the /healthz endpoint
func (h *HealthChecker) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("mode") == "extended" {
		checks, status := h.runExtendedChecks(r.Context())
		h.writeHealthResponse(w, status, checks)
		return
	}
	h.writeHealthResponse(w, "healthy", nil)
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

// checkRedis verifies the Redis connection via Pinger
func (h *HealthChecker) checkRedis(ctx context.Context) error {
	if h.redisPinger == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return h.redisPinger.Ping(ctx)
}

// checkRabbitMQ verifies the RabbitMQ connection
func (h *HealthChecker) checkRabbitMQ(ctx context.Context) error {
	if h.jobQueue == nil {
		return nil // Not configured, skip check
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Try to check if the connection is still alive
	// RabbitMQQueue has a conn field, but it's private
	// We can try a lightweight operation or add a HealthCheck method
	// For now, we'll add a simple check by trying to get the connection state
	return h.jobQueue.HealthCheck(ctx)
}
