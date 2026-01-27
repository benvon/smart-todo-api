package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/middleware"
	"github.com/benvon/smart-todo/internal/queue"
)

// HealthChecker handles health check requests
type HealthChecker struct {
	db           *database.DB
	redisLimiter *middleware.RedisRateLimiter
	jobQueue     queue.JobQueue
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(db *database.DB) *HealthChecker {
	return &HealthChecker{db: db}
}

// NewHealthCheckerWithDeps creates a new health checker with Redis and RabbitMQ dependencies
func NewHealthCheckerWithDeps(db *database.DB, redisLimiter *middleware.RedisRateLimiter, jobQueue queue.JobQueue) *HealthChecker {
	return &HealthChecker{
		db:           db,
		redisLimiter: redisLimiter,
		jobQueue:     jobQueue,
	}
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
			// Don't expose detailed error messages - just indicate unhealthy
			checks["database"] = "unhealthy"
		} else {
			checks["database"] = "healthy"
		}

		// Check Redis connection
		if h.redisLimiter != nil {
			if err := h.checkRedis(r.Context()); err != nil {
				response.Status = "unhealthy"
				checks["redis"] = "unhealthy"
			} else {
				checks["redis"] = "healthy"
			}
		} else {
			checks["redis"] = "not configured"
		}

		// Check RabbitMQ connection
		if h.jobQueue != nil {
			if err := h.checkRabbitMQ(r.Context()); err != nil {
				response.Status = "unhealthy"
				checks["rabbitmq"] = "unhealthy"
			} else {
				checks["rabbitmq"] = "healthy"
			}
		} else {
			checks["rabbitmq"] = "not configured"
		}

		response.Checks = checks

		statusCode := http.StatusOK
		if response.Status == "unhealthy" {
			statusCode = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			// Log error but response already started, can't send error response
			return
		}
		return
	}

	// Basic mode - just return that the server is running
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Log error but response already started, can't send error response
		return
	}
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

// checkRedis verifies the Redis connection
func (h *HealthChecker) checkRedis(ctx context.Context) error {
	if h.redisLimiter == nil {
		return nil // Not configured, skip check
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Use the Redis client's Ping method
	// The RedisRateLimiter wraps a redis.Client, but we need to access it
	// For now, we'll use a simple approach: try to get a key (which will ping)
	// Actually, we need to add a Ping method to RedisRateLimiter or access the client
	// Let's add a simple health check method to RedisRateLimiter
	return h.redisLimiter.Ping(ctx)
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
