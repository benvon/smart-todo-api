package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/benvon/smart-todo/internal/models"
)

const defaultRatelimitConfigKey = "default"

// RatelimitConfigRepository handles rate limit configuration in the database.
type RatelimitConfigRepository struct {
	db *DB
}

// NewRatelimitConfigRepository creates a new ratelimit config repository.
func NewRatelimitConfigRepository(db *DB) *RatelimitConfigRepository {
	return &RatelimitConfigRepository{db: db}
}

// Get retrieves the default rate limit config.
func (r *RatelimitConfigRepository) Get(ctx context.Context) (*models.RatelimitConfig, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT config_key, rate, created_at, updated_at
		FROM ratelimit_config WHERE config_key = $1
	`, defaultRatelimitConfigKey)
	c := &models.RatelimitConfig{}
	err := row.Scan(&c.ConfigKey, &c.Rate, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ratelimit config: %w", err)
	}
	return c, nil
}

// Set upserts the default rate limit config. Rate format: e.g. "5-S", "100-M".
func (r *RatelimitConfigRepository) Set(ctx context.Context, c *models.RatelimitConfig) error {
	rate := strings.TrimSpace(c.Rate)
	if rate == "" {
		return fmt.Errorf("rate cannot be empty")
	}
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ratelimit_config (config_key, rate, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (config_key) DO UPDATE SET
			rate = EXCLUDED.rate,
			updated_at = EXCLUDED.updated_at
	`, defaultRatelimitConfigKey, rate, now, now)
	if err != nil {
		return fmt.Errorf("set ratelimit config: %w", err)
	}
	return nil
}
