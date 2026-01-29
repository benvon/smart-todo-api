package middleware

import (
	"context"
	"net/http"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/request"
	"github.com/redis/go-redis/v9"
	"github.com/ulule/limiter/v3"
	stdlibmw "github.com/ulule/limiter/v3/drivers/middleware/stdlib"
	redisstore "github.com/ulule/limiter/v3/drivers/store/redis"
)

const defaultRatelimitRate = "5-S"

// RateLimitFromDB returns middleware that uses ulule/limiter with Redis, loading rate from DB.
// If no config exists, defaultRate is saved to DB. Uses request.ClientIP for the limit key.
func RateLimitFromDB(redisClient *redis.Client, repo *database.RatelimitConfigRepository, defaultRate string) (func(http.Handler) http.Handler, error) {
	if defaultRate == "" {
		defaultRate = defaultRatelimitRate
	}
	ctx := context.Background()
	cfg, err := repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	rateStr := defaultRate
	if cfg != nil && cfg.Rate != "" {
		rateStr = cfg.Rate
	} else {
		if err = repo.Set(ctx, &models.RatelimitConfig{Rate: defaultRate}); err != nil {
			return nil, err
		}
	}
	rate, err := limiter.NewRateFromFormatted(rateStr)
	if err != nil {
		return nil, err
	}
	store, err := redisstore.NewStore(redisClient)
	if err != nil {
		return nil, err
	}
	instance := limiter.New(store, rate)
	keyGetter := func(r *http.Request) string {
		return request.ClientIP(r)
	}
	mw := stdlibmw.NewMiddleware(instance, stdlibmw.WithKeyGetter(keyGetter))
	return mw.Handler, nil
}
