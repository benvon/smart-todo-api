package request

import (
	"context"
	"net/http"
	"strings"

	"github.com/benvon/smart-todo/internal/models"
)

type contextKey string

const userContextKey contextKey = "user"

// UserContextKey returns the context key used for the user. Exposed for tests that inject non-user values.
func UserContextKey() contextKey { return userContextKey }

// ClientIP extracts the client IP from the request, respecting X-Forwarded-For and X-Real-IP.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	return r.RemoteAddr
}

// WithUser returns a context with the user attached.
func WithUser(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext returns the user from the request context, or nil if missing or wrong type.
func UserFromContext(r *http.Request) *models.User {
	u, _ := r.Context().Value(userContextKey).(*models.User)
	return u
}
