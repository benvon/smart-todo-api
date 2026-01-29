package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/benvon/smart-todo/internal/models"
)

const defaultCorsConfigKey = "default"

// CorsConfigRepository handles CORS configuration in the database.
type CorsConfigRepository struct {
	db *DB
}

// NewCorsConfigRepository creates a new CORS config repository.
func NewCorsConfigRepository(db *DB) *CorsConfigRepository {
	return &CorsConfigRepository{db: db}
}

// Get retrieves the default CORS config.
func (r *CorsConfigRepository) Get(ctx context.Context) (*models.CorsConfig, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT config_key, allowed_origins, allow_credentials, max_age, created_at, updated_at
		FROM cors_config WHERE config_key = $1
	`, defaultCorsConfigKey)
	c := &models.CorsConfig{}
	err := row.Scan(
		&c.ConfigKey,
		&c.AllowedOrigins,
		&c.AllowCredentials,
		&c.MaxAge,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get cors config: %w", err)
	}
	return c, nil
}

// Set upserts the default CORS config. AllowedOrigins is comma-separated.
func (r *CorsConfigRepository) Set(ctx context.Context, c *models.CorsConfig) error {
	if c.AllowedOrigins == "" {
		return fmt.Errorf("allowed_origins cannot be empty")
	}
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO cors_config (config_key, allowed_origins, allow_credentials, max_age, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (config_key) DO UPDATE SET
			allowed_origins = EXCLUDED.allowed_origins,
			allow_credentials = EXCLUDED.allow_credentials,
			max_age = EXCLUDED.max_age,
			updated_at = EXCLUDED.updated_at
	`, defaultCorsConfigKey, strings.TrimSpace(c.AllowedOrigins), c.AllowCredentials, c.MaxAge, now, now)
	if err != nil {
		return fmt.Errorf("set cors config: %w", err)
	}
	return nil
}

// AllowedOriginsSlice returns allowed origins as a slice (split by comma).
func AllowedOriginsSlice(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var out []string
	seen := make(map[string]bool)
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
