package middleware

import (
	"log"
	"net/http"
)

// Audit logs security-related events for monitoring and compliance
func Audit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wrap ResponseWriter to capture status code for audit logging
		wrapped := &auditResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		// Log security-relevant events
		statusCode := wrapped.statusCode
		if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
			// Log failed authentication/authorization attempts
			ip := getClientIP(r)
			log.Printf("[AUDIT] Security event: status=%d method=%s path=%s ip=%s",
				statusCode, r.Method, r.URL.Path, ip)
		}

		// Log rate limit violations (429 Too Many Requests)
		if statusCode == http.StatusTooManyRequests {
			ip := getClientIP(r)
			log.Printf("[AUDIT] Rate limit violation: method=%s path=%s ip=%s",
				r.Method, r.URL.Path, ip)
		}
	})
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
