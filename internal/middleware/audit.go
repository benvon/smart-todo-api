package middleware

import (
	"log"
	"net/http"
	"strings"
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

// getClientIP extracts the client IP from the request, respecting X-Forwarded-For
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies/load balancers)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs (comma-separated)
		// The first one is typically the original client IP
		ips := splitIPs(xff)
		if len(ips) > 0 {
			return ips[0]
		}
	}

	// Check X-Real-IP header (alternative header used by some proxies)
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// splitIPs splits a comma-separated list of IP addresses
func splitIPs(ips string) []string {
	var result []string
	parts := splitCommaSeparated(ips)
	for _, part := range parts {
		trimmed := trimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// splitCommaSeparated splits a string by commas
func splitCommaSeparated(s string) []string {
	return strings.Split(s, ",")
}

// trimSpace trims whitespace from a string
func trimSpace(s string) string {
	return strings.TrimSpace(s)
}