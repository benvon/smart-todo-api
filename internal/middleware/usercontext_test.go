package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/benvon/smart-todo/internal/models"
)

func TestUserFromContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(*http.Request) *http.Request
		validate func(*testing.T, *models.User)
	}{
		{
			name: "user in context",
			setup: func(r *http.Request) *http.Request {
				userID := uuid.New()
				user := &models.User{
					ID:    userID,
					Email: "test@example.com",
				}
				ctx := r.Context()
				ctx = SetUserInContext(ctx, user)
				return r.WithContext(ctx)
			},
			validate: func(t *testing.T, user *models.User) {
				if user == nil {
					t.Fatal("Expected user to be present")
				}
				if user.Email != "test@example.com" {
					t.Errorf("Expected email 'test@example.com', got '%s'", user.Email)
				}
			},
		},
		{
			name: "no user in context",
			setup: func(r *http.Request) *http.Request {
				return r
			},
			validate: func(t *testing.T, user *models.User) {
				if user != nil {
					t.Errorf("Expected user to be nil, got %+v", user)
				}
			},
		},
		{
			name: "wrong type in context",
			setup: func(r *http.Request) *http.Request {
				ctx := r.Context()
				ctx = context.WithValue(ctx, userContextKey, "not a user")
				return r.WithContext(ctx)
			},
			validate: func(t *testing.T, user *models.User) {
				if user != nil {
					t.Errorf("Expected user to be nil when wrong type in context, got %+v", user)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/test", nil)
			req = tt.setup(req)

			user := UserFromContext(req)

			if tt.validate != nil {
				tt.validate(t, user)
			}
		})
	}
}

// Helper function for testing - sets user in context
func SetUserInContext(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}
