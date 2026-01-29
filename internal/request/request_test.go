package request

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/benvon/smart-todo/internal/models"
	"github.com/google/uuid"
)

func TestClientIP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		headers map[string]string
		remote  string
		wantIP  string
	}{
		{"x-forwarded-for", map[string]string{"X-Forwarded-For": "1.2.3.4"}, "", "1.2.3.4"},
		{"x-forwarded-for first", map[string]string{"X-Forwarded-For": " 1.2.3.4 , 5.6.7.8 "}, "", "1.2.3.4"},
		{"x-real-ip", map[string]string{"X-Real-IP": "9.9.9.9"}, "", "9.9.9.9"},
		{"remote addr", nil, "10.0.0.1:12345", "10.0.0.1:12345"},
		{"xff over xri", map[string]string{"X-Forwarded-For": "1.2.3.4", "X-Real-IP": "9.9.9.9"}, "", "1.2.3.4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest("GET", "/", nil)
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}
			if tt.remote != "" {
				r.RemoteAddr = tt.remote
			}
			got := ClientIP(r)
			if got != tt.wantIP {
				t.Errorf("ClientIP() = %q, want %q", got, tt.wantIP)
			}
		})
	}
}

func TestUserFromContext(t *testing.T) {
	t.Parallel()
	u := &models.User{ID: uuid.New(), Email: "a@b.c"}
	ctx := WithUser(context.Background(), u)
	r := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	got := UserFromContext(r)
	if got != u {
		t.Errorf("UserFromContext() = %p, want %p", got, u)
	}
	if got != nil && got.Email != "a@b.c" {
		t.Errorf("UserFromContext().Email = %q, want a@b.c", got.Email)
	}
}

func TestUserFromContext_NoUser(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest("GET", "/", nil)
	got := UserFromContext(r)
	if got != nil {
		t.Errorf("UserFromContext() = %+v, want nil", got)
	}
}

func TestUserFromContext_WrongType(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), UserContextKey(), "not a user")
	r := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	got := UserFromContext(r)
	if got != nil {
		t.Errorf("UserFromContext() = %+v, want nil when wrong type", got)
	}
}
