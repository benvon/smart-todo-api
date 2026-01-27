package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// getClientIPForRateLimit extracts the client IP for rate limiting
func getClientIPForRateLimit(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies/load balancers)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs (comma-separated)
		// The first one is typically the original client IP
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header (alternative header used by some proxies)
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

const (
	// DefaultUnauthenticatedRateLimit is the default rate limit for unauthenticated requests (100 req/min)
	DefaultUnauthenticatedRateLimit = 100
	// DefaultAuthenticatedRateLimit is the default rate limit for authenticated requests (1000 req/min)
	DefaultAuthenticatedRateLimit = 1000
)

// RedisRateLimiter wraps Redis client for rate limiting
type RedisRateLimiter struct {
	client *redis.Client
}

// NewRedisRateLimiter creates a new Redis-backed rate limiter
func NewRedisRateLimiter(redisURL string) (*RedisRateLimiter, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisRateLimiter{client: client}, nil
}

// Close closes the Redis connection
func (r *RedisRateLimiter) Close() error {
	return r.client.Close()
}

// Ping checks if Redis is reachable
func (r *RedisRateLimiter) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// LimitCounter implements httprate.LimitCounter interface for Redis
type redisLimitCounter struct {
	client *redis.Client
	key    string
	limit  int
	window time.Duration
}

// Increment increments the counter and returns the new count
func (c *redisLimitCounter) Increment(ctx context.Context) (int, error) {
	now := time.Now()
	windowStart := now.Truncate(c.window)

	// Use a sliding window key based on the window start time
	key := fmt.Sprintf("%s:%d", c.key, windowStart.Unix())

	// Increment and set expiration
	pipe := c.client.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, c.window+time.Second) // Add 1 second buffer
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}

	count := int(incr.Val())

	// Also check previous window for sliding window behavior
	prevWindowStart := windowStart.Add(-c.window)
	prevKey := fmt.Sprintf("%s:%d", c.key, prevWindowStart.Unix())
	prevCount := c.client.Get(ctx, prevKey).Val()
		if prevCount != "" {
			// Calculate sliding window count (proportional to remaining time in previous window)
			elapsed := now.Sub(windowStart)
			var prevWindowCount int
			if _, err := fmt.Sscanf(prevCount, "%d", &prevWindowCount); err == nil && prevWindowCount > 0 {
				// Weight the previous window count by how much time is left
				remainingRatio := float64(c.window-elapsed) / float64(c.window)
				count += int(float64(prevWindowCount) * remainingRatio)
			}
		}

	return count, nil
}

// RateLimit creates rate limiting middleware using Redis
func RateLimit(redisLimiter *RedisRateLimiter, requestsPerMinute int) func(http.Handler) http.Handler {
	if requestsPerMinute <= 0 {
		requestsPerMinute = DefaultUnauthenticatedRateLimit
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := getClientIPForRateLimit(r)
			counter := &redisLimitCounter{
				client: redisLimiter.client,
				key:    fmt.Sprintf("ratelimit:%s", key),
				limit:  requestsPerMinute,
				window: time.Minute,
			}

			ctx := r.Context()
			count, err := counter.Increment(ctx)
			if err != nil {
				// On Redis error, log but allow request (fail open for availability)
				// In production, you might want to fail closed instead
				next.ServeHTTP(w, r)
				return
			}

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerMinute))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, requestsPerMinute-count)))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Minute).Unix()))

			if count > requestsPerMinute {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitAuthenticated creates rate limiting for authenticated endpoints using Redis
func RateLimitAuthenticated(redisLimiter *RedisRateLimiter) func(http.Handler) http.Handler {
	return RateLimit(redisLimiter, DefaultAuthenticatedRateLimit)
}

// RateLimitUnauthenticated creates rate limiting for unauthenticated endpoints using Redis
func RateLimitUnauthenticated(redisLimiter *RedisRateLimiter) func(http.Handler) http.Handler {
	return RateLimit(redisLimiter, DefaultUnauthenticatedRateLimit)
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
