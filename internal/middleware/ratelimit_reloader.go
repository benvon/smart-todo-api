package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/request"
	"github.com/redis/go-redis/v9"
	"github.com/ulule/limiter/v3"
	stdlibmw "github.com/ulule/limiter/v3/drivers/middleware/stdlib"
	redisstore "github.com/ulule/limiter/v3/drivers/store/redis"
	"go.uber.org/zap"
)

// RateLimitReloader wraps ulule/limiter and periodically reloads rate limit config from the database.
type RateLimitReloader struct {
	next        http.Handler
	redisClient *redis.Client
	store       limiter.Store
	repo        *database.RatelimitConfigRepository
	defaultRate string
	log         *zap.Logger
	interval    time.Duration
	mu          sync.RWMutex
	current     http.Handler
}

// NewRateLimitReloader creates a rate limit middleware that loads config from the DB and hot-reloads it.
func NewRateLimitReloader(redisClient *redis.Client, repo *database.RatelimitConfigRepository, defaultRate string, log *zap.Logger, reloadInterval time.Duration) *RateLimitReloader {
	if defaultRate == "" {
		defaultRate = defaultRatelimitRate
	}
	// Create Redis store once during initialization
	store, err := redisstore.NewStore(redisClient)
	if err != nil {
		log.Error("failed_to_create_redis_store_for_rate_limiter",
			zap.Error(err),
		)
		return nil
	}
	return &RateLimitReloader{
		redisClient: redisClient,
		store:       store,
		repo:        repo,
		defaultRate: defaultRate,
		log:         log,
		interval:    reloadInterval,
	}
}

// Middleware returns a middleware that wraps next with rate limiting and hot-reload.
func (r *RateLimitReloader) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		r.next = next
		r.load(context.Background())
		return r
	}
}

// Start runs the reload loop until ctx is cancelled. Call after Middleware() is applied.
func (r *RateLimitReloader) Start(ctx context.Context) {
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

func (r *RateLimitReloader) load(ctx context.Context) {
	if r.next == nil {
		return
	}
	cfg, err := r.repo.Get(ctx)
	rateStr := r.defaultRate
	if err != nil {
		r.log.Warn("failed_to_load_ratelimit_config_from_db_using_default",
			zap.Error(err),
			zap.String("default_rate", r.defaultRate),
		)
	} else if cfg != nil && cfg.Rate != "" {
		rateStr = cfg.Rate
	} else {
		// Save default config if none exists
		if err = r.repo.Set(ctx, &models.RatelimitConfig{Rate: r.defaultRate}); err != nil {
			r.log.Error("failed_to_save_default_ratelimit_config",
				zap.Error(err),
				zap.String("default_rate", r.defaultRate),
			)
		}
	}

	rate, err := limiter.NewRateFromFormatted(rateStr)
	if err != nil {
		r.log.Error("failed_to_parse_rate_limit_using_default",
			zap.Error(err),
			zap.String("rate_str", rateStr),
			zap.String("default_rate", r.defaultRate),
		)
		// Try to use default rate as fallback
		rate, err = limiter.NewRateFromFormatted(r.defaultRate)
		if err != nil {
			r.log.Error("failed_to_parse_default_rate_limit",
				zap.Error(err),
				zap.String("default_rate", r.defaultRate),
			)
			return
		}
	}

	// Reuse the existing Redis store, only create a new limiter instance with the new rate
	instance := limiter.New(r.store, rate)
	keyGetter := func(req *http.Request) string {
		return request.ClientIP(req)
	}
	mw := stdlibmw.NewMiddleware(instance, stdlibmw.WithKeyGetter(keyGetter))
	h := mw.Handler(r.next)

	r.mu.Lock()
	r.current = h
	r.mu.Unlock()
}

// ServeHTTP implements http.Handler.
func (r *RateLimitReloader) ServeHTTP(w http.ResponseWriter, req *http.Request) {
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
