package middleware

import (
	"net/http"
	"net/url"
	"strings"

	logpkg "github.com/benvon/smart-todo/internal/logger"
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
			origin := normalizeCORSOrigin(r.Header.Get("Origin"), logger, debugMode)
			if debugMode {
				logger.Debug("cors_request",
					zap.String("method", r.Method),
					zap.String("path", logpkg.SanitizePath(r.URL.Path)),
					zap.String("origin", logpkg.SanitizeString(origin, logpkg.MaxGeneralStringLength)),
				)
			}
			allowed := originAllowed(origin, allowedOrigins)
			if r.Method == http.MethodOptions {
				corsHandleOptions(w, r, origin, allowed, logger, debugMode)
				return
			}
			if allowed && origin != "" {
				setCORSHeaders(w, origin)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func normalizeCORSOrigin(origin string, logger *zap.Logger, debugMode bool) string {
	if origin == "" {
		return ""
	}
	if !isValidOrigin(origin) {
		if debugMode {
			logger.Debug("cors_invalid_origin_format",
				zap.String("origin", logpkg.SanitizeString(origin, logpkg.MaxGeneralStringLength)),
			)
		}
		return ""
	}
	if origin == "null" {
		return ""
	}
	return origin
}

func originAllowed(origin string, allowedOrigins []string) bool {
	if origin == "" {
		return false
	}
	for _, o := range allowedOrigins {
		if origin == o {
			return true
		}
	}
	return false
}

func setCORSHeaders(w http.ResponseWriter, origin string) {
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
}

func corsHandleOptions(w http.ResponseWriter, r *http.Request, origin string, allowed bool, logger *zap.Logger, debugMode bool) {
	if debugMode {
		logger.Debug("cors_options_preflight",
			zap.String("path", logpkg.SanitizePath(r.URL.Path)),
			zap.Bool("allowed", allowed),
		)
	}
	if allowed && origin != "" {
		setCORSHeaders(w, origin)
		w.Header().Set("Access-Control-Max-Age", "86400")
	}
	w.WriteHeader(http.StatusNoContent)
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
