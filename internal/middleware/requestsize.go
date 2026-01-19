package middleware

import (
	"net/http"
)

const (
	// DefaultMaxRequestSize is the default maximum request body size (1MB)
	DefaultMaxRequestSize int64 = 1 << 20 // 1MB
)

// MaxRequestSize limits the size of request bodies to prevent DoS attacks
func MaxRequestSize(maxBytes int64) func(http.Handler) http.Handler {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxRequestSize
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check Content-Length header early if present
			if r.ContentLength > maxBytes {
				http.Error(w, "Request Entity Too Large", http.StatusRequestEntityTooLarge)
				return
			}

			// Wrap the request body with MaxBytesReader
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			defer r.Body.Close()

			next.ServeHTTP(w, r)
		})
	}
}