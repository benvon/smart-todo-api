package middleware

import (
	"net/http"

	logpkg "github.com/benvon/smart-todo/internal/logger"
	"go.uber.org/zap"
)

// Audit logs security-related events for monitoring and compliance
func Audit(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Wrap ResponseWriter to capture status code for audit logging
			wrapped := &auditResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			// Log security-relevant events
			statusCode := wrapped.statusCode
			if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
				// Log failed authentication/authorization attempts
				ip := getClientIP(r)
				logger.Warn("security_event",
					zap.Int("status_code", statusCode),
					zap.String("method", r.Method),
					zap.String("path", logpkg.SanitizePath(r.URL.Path)),
					zap.String("ip", logpkg.SanitizeString(ip, logpkg.MaxGeneralStringLength)),
				)
			}

			// Log rate limit violations (429 Too Many Requests)
			if statusCode == http.StatusTooManyRequests {
				ip := getClientIP(r)
				logger.Warn("rate_limit_violation",
					zap.String("method", r.Method),
					zap.String("path", logpkg.SanitizePath(r.URL.Path)),
					zap.String("ip", logpkg.SanitizeString(ip, logpkg.MaxGeneralStringLength)),
				)
			}
		})
	}
}

// auditResponseWriter wraps http.ResponseWriter to capture status code
type auditResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (aw *auditResponseWriter) WriteHeader(code int) {
	aw.statusCode = code
	aw.ResponseWriter.WriteHeader(code)
}
