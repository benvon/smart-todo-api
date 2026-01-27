package middleware

import (
	"context"

	"github.com/benvon/smart-todo/internal/models"
)

// SetUserInContext is a helper function for testing - sets user in context
// This is exported so other test packages can use it
func SetUserInContext(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}
