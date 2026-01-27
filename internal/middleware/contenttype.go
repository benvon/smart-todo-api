package middleware

import (
	"net/http"
	"strings"
)

// ContentType validates Content-Type headers for requests with bodies
func ContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only validate Content-Type for methods that typically have bodies
		if r.Method == "POST" || r.Method == "PATCH" || r.Method == "PUT" {
			contentType := r.Header.Get("Content-Type")

			// Check if Content-Type is present
			if contentType == "" {
				http.Error(w, "Content-Type header is required", http.StatusBadRequest)
				return
			}

			// For JSON APIs, require application/json (or application/json with charset)
			// Allow common variations
			contentTypeLower := strings.ToLower(contentType)
			isJSON := strings.HasPrefix(contentTypeLower, "application/json")

			if !isJSON {
				http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
