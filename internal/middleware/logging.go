package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Logging creates logging middleware
func Logging(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap ResponseWriter to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			logger.Info("http_request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status_code", wrapped.statusCode),
				zap.Duration("duration_ms", duration),
			)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
