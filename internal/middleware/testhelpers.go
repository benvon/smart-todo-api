package middleware

import (
	"context"

	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/request"
)

// SetUserInContext is a test helper that sets the user in context.
func SetUserInContext(ctx context.Context, user *models.User) context.Context {
	return request.WithUser(ctx, user)
}
