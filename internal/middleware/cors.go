package middleware

import (
	"log"
	"net/http"
	"strings"
)

// CORS creates CORS middleware that handles CORS headers and OPTIONS preflight requests
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	log.Printf("CORS middleware initialized with allowed origins: %v", allowedOrigins)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Log every request to verify middleware is being called
			origin := r.Header.Get("Origin")
			log.Printf("[CORS] REQUEST: %s %s | Origin: '%s' | Allowed origins: %v", 
				r.Method, r.URL.Path, origin, allowedOrigins)
			
			// Check if origin is allowed
			allowed := false
			for _, allowedOrigin := range allowedOrigins {
				if origin == allowedOrigin {
					allowed = true
					break
				}
			}

			// Handle preflight OPTIONS requests
			if r.Method == http.MethodOptions {
				log.Printf("[CORS] OPTIONS preflight for %s - allowed: %v", r.URL.Path, allowed)
				if allowed && origin != "" {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					w.Header().Set("Access-Control-Max-Age", "86400") // Cache preflight for 24 hours
					log.Printf("[CORS] Set CORS headers for OPTIONS preflight")
				} else {
					log.Printf("[CORS] OPTIONS request from disallowed origin: %s", origin)
				}
				// Return 204 for OPTIONS regardless of origin (browser will reject if headers missing)
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// For actual requests, set CORS headers if origin is allowed
			if allowed && origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORSFromEnv creates CORS middleware from environment variable
// Parses FRONTEND_URL (comma-separated origins) and defaults to http://localhost:3000
func CORSFromEnv(frontendURL string) func(http.Handler) http.Handler {
	origins := []string{"http://localhost:3000"}
	if frontendURL != "" {
		// Parse comma-separated origins and trim whitespace
		envOrigins := strings.Split(frontendURL, ",")
		for _, origin := range envOrigins {
			trimmed := strings.TrimSpace(origin)
			if trimmed != "" {
				// Avoid duplicates
				exists := false
				for _, existing := range origins {
					if existing == trimmed {
						exists = true
						break
					}
				}
				if !exists {
					origins = append(origins, trimmed)
				}
			}
		}
	}
	return CORS(origins)
}
