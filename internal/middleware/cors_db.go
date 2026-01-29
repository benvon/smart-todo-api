package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/rs/cors"
	"go.uber.org/zap"
)

// CORSReloader wraps rs/cors and periodically reloads CORS config from the database.
type CORSReloader struct {
	next     http.Handler
	repo     *database.CorsConfigRepository
	fallback string // e.g. FRONTEND_URL
	log      *zap.Logger
	interval time.Duration
	mu       sync.RWMutex
	current  http.Handler
}

// NewCORSReloader creates a CORS middleware that loads config from the DB and hot-reloads it.
func NewCORSReloader(repo *database.CorsConfigRepository, frontendURLFallback string, log *zap.Logger, reloadInterval time.Duration) *CORSReloader {
	return &CORSReloader{
		repo:     repo,
		fallback: strings.TrimSpace(frontendURLFallback),
		log:      log,
		interval: reloadInterval,
	}
}

// Middleware returns a middleware that wraps next with CORS and hot-reload.
func (r *CORSReloader) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		r.next = next
		r.load(context.Background())
		return r
	}
}

// Start runs the reload loop until ctx is cancelled. Call after Middleware() is applied.
func (r *CORSReloader) Start(ctx context.Context) {
	if r.interval <= 0 {
		return
	}
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.load(ctx)
		}
	}
}

func (r *CORSReloader) load(ctx context.Context) {
	if r.next == nil {
		return
	}
	cfg, err := r.repo.Get(ctx)
	var origins []string
	var allowCreds bool
	var maxAge int
	if err != nil || cfg == nil {
		origins = database.AllowedOriginsSlice(r.fallback)
		allowCreds = true
		maxAge = 86400
	} else {
		origins = database.AllowedOriginsSlice(cfg.AllowedOrigins)
		allowCreds = cfg.AllowCredentials
		maxAge = cfg.MaxAge
	}
	if len(origins) == 0 {
		origins = []string{"http://localhost:3000"}
	}
	opts := cors.Options{
		AllowedOrigins:   origins,
		AllowCredentials: allowCreds,
		MaxAge:           maxAge,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
	}
	c := cors.New(opts)
	h := c.Handler(r.next)
	r.mu.Lock()
	r.current = h
	r.mu.Unlock()
}

// ServeHTTP implements http.Handler.
func (r *CORSReloader) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	h := r.current
	r.mu.RUnlock()
	if h != nil {
		h.ServeHTTP(w, req)
		return
	}
	if r.next != nil {
		r.next.ServeHTTP(w, req)
	}
}
