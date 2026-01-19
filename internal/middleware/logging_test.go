package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogging(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		method        string
		path          string
		handlerStatus int
		validate      func(*testing.T, *http.Response)
	}{
		{
			name:          "GET request",
			method:        "GET",
			path:          "/test",
			handlerStatus: http.StatusOK,
			validate: func(t *testing.T, resp *http.Response) {
				if resp.StatusCode != http.StatusOK {
					t.Errorf("Expected status 200, got %d", resp.StatusCode)
				}
			},
		},
		{
			name:          "POST request",
			method:        "POST",
			path:          "/api/v1/todos",
			handlerStatus: http.StatusCreated,
			validate: func(t *testing.T, resp *http.Response) {
				if resp.StatusCode != http.StatusCreated {
					t.Errorf("Expected status 201, got %d", resp.StatusCode)
				}
			},
		},
		{
			name:          "404 request",
			method:        "GET",
			path:          "/notfound",
			handlerStatus: http.StatusNotFound,
			validate: func(t *testing.T, resp *http.Response) {
				if resp.StatusCode != http.StatusNotFound {
					t.Errorf("Expected status 404, got %d", resp.StatusCode)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.handlerStatus)
			})

			middleware := Logging(handler)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			middleware.ServeHTTP(w, req)

			resp := w.Result()
			defer func() {
				_ = resp.Body.Close() // Ignore error in test
			}()

			if tt.validate != nil {
				tt.validate(t, resp)
			}
		})
	}
}

func TestLoggingResponseWriter(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("test")) // Ignore error in test
	})

	middleware := Logging(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	resp := w.Result()
	defer func() {
		_ = resp.Body.Close() // Ignore error in test
	}()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}
}
