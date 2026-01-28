package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"go.uber.org/zap"
)

// isValidOrigin validates that an origin string has a valid format (scheme://host[:port])
func isValidOrigin(origin string) bool {
	if origin == "" || origin == "null" {
		return false
	}

	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}

	// Must have scheme and host
	if parsed.Scheme == "" || parsed.Host == "" {
		return false
	}

	// Only allow http or https schemes
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	return true
}

// CORS creates CORS middleware that handles CORS headers and OPTIONS preflight requests
func CORS(allowedOrigins []string, logger *zap.Logger, debugMode bool) func(http.Handler) http.Handler {
	if debugMode {
		logger.Info("cors_middleware_initialized",
			zap.Strings("allowed_origins", allowedOrigins),
		)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Validate origin format
			if origin != "" && !isValidOrigin(origin) {
				if debugMode {
					logger.Debug("cors_invalid_origin_format",
						zap.String("origin", origin),
					)
				}
				// Continue but don't set CORS headers for invalid origins
				origin = ""
			}

			if debugMode {
				logger.Debug("cors_request",
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.String("origin", origin),
				)
			}

			// Check if origin is allowed
			allowed := false
			if origin != "" && origin != "null" {
				for _, allowedOrigin := range allowedOrigins {
					if origin == allowedOrigin {
						allowed = true
						break
					}
				}
			}

			// Handle preflight OPTIONS requests
			if r.Method == http.MethodOptions {
				if debugMode {
					logger.Debug("cors_options_preflight",
						zap.String("path", r.URL.Path),
						zap.Bool("allowed", allowed),
					)
				}
				if allowed && origin != "" && origin != "null" {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					w.Header().Set("Access-Control-Max-Age", "86400") // Cache preflight for 24 hours
				}
				// Return 204 for OPTIONS (browser will reject if headers missing for disallowed origin)
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// For actual requests, set CORS headers if origin is allowed
			if allowed && origin != "" && origin != "null" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORSFromEnv creates CORS middleware from environment variable
// Parses FRONTEND_URL (comma-separated origins) and defaults to http://localhost:3000
func CORSFromEnv(frontendURL string, logger *zap.Logger, debugMode bool) func(http.Handler) http.Handler {
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
	return CORS(origins, logger, debugMode)
}
