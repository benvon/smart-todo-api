package middleware

import (
	"context"
	"net/http"
	"time"
)

const (
	// DefaultRequestTimeout is the default request timeout (30 seconds)
	DefaultRequestTimeout = 30 * time.Second
)

// Timeout creates a middleware that enforces a timeout on request handlers
func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	if timeout <= 0 {
		timeout = DefaultRequestTimeout
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			// Replace the request context with timeout context
			r = r.WithContext(ctx)

			// Use TimeoutHandler for automatic timeout handling
			handler := http.TimeoutHandler(next, timeout, "Request Timeout")
			handler.ServeHTTP(w, r)
		})
	}
}
