package middleware

import (
	"net/http"
)

// SecurityHeaders sets security headers on all responses
func SecurityHeaders(enableHSTS bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// X-Content-Type-Options: Prevent MIME type sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// X-Frame-Options: Prevent clickjacking
			w.Header().Set("X-Frame-Options", "DENY")

			// X-XSS-Protection: Enable browser XSS filter (legacy but still supported)
			w.Header().Set("X-XSS-Protection", "1; mode=block")

			// Referrer-Policy: Control referrer information sharing
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Permissions-Policy: Disable unused browser features
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

			// Content-Security-Policy: Minimal for API (no inline scripts/styles)
			// Since this is an API, we use a restrictive policy
			w.Header().Set("Content-Security-Policy", "default-src 'none'")

			// Strict-Transport-Security (HSTS): Only set if:
			// 1. Request is over HTTPS (r.TLS != nil)
			// 2. Explicitly enabled via enableHSTS config
			// This prevents issues in local development
			if enableHSTS && r.TLS != nil {
				// 1 year max-age with includeSubDomains and preload
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}

			next.ServeHTTP(w, r)
		})
	}
}
