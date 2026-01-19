package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/benvon/smart-todo/internal/models"
)

// OIDCConfigRepository handles OIDC configuration database operations
type OIDCConfigRepository struct {
	db *DB
}

// NewOIDCConfigRepository creates a new OIDC config repository
func NewOIDCConfigRepository(db *DB) *OIDCConfigRepository {
	return &OIDCConfigRepository{db: db}
}

// Create creates a new OIDC configuration
func (r *OIDCConfigRepository) Create(ctx context.Context, config *models.OIDCConfig) error {
	query := `
		INSERT INTO oidc_config (id, provider, issuer, domain, client_id, client_secret, redirect_uri, jwks_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at
	`
	
	now := time.Now()
	err := r.db.QueryRowContext(ctx, query,
		config.ID,
		config.Provider,
		config.Issuer,
		config.Domain,
		config.ClientID,
		config.ClientSecret,
		config.RedirectURI,
		config.JWKSUrl,
		now,
		now,
	).Scan(&config.CreatedAt, &config.UpdatedAt)
	
	if err != nil {
		return fmt.Errorf("failed to create OIDC config: %w", err)
	}
	
	return nil
}

// GetByProvider retrieves an OIDC configuration by provider name
func (r *OIDCConfigRepository) GetByProvider(ctx context.Context, provider string) (*models.OIDCConfig, error) {
	config := &models.OIDCConfig{}
	query := `
		SELECT id, provider, issuer, domain, client_id, client_secret, redirect_uri, jwks_url, created_at, updated_at
		FROM oidc_config
		WHERE provider = $1
	`
	
	err := r.db.QueryRowContext(ctx, query, provider).Scan(
		&config.ID,
		&config.Provider,
		&config.Issuer,
		&config.Domain,
		&config.ClientID,
		&config.ClientSecret,
		&config.RedirectURI,
		&config.JWKSUrl,
		&config.CreatedAt,
		&config.UpdatedAt,
	)
	
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("OIDC config not found for provider %s: %w", provider, err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get OIDC config: %w", err)
	}
	
	return config, nil
}

// GetAll retrieves all OIDC configurations
func (r *OIDCConfigRepository) GetAll(ctx context.Context) ([]*models.OIDCConfig, error) {
	query := `
		SELECT id, provider, issuer, domain, client_id, client_secret, redirect_uri, jwks_url, created_at, updated_at
		FROM oidc_config
		ORDER BY provider
	`
	
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query OIDC configs: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Log error but continue - rows may already be closed
			// This is in database layer, logging would require passing logger
			// The error is non-critical as rows are already processed
			_ = err // Explicitly ignore error to satisfy linter
		}
	}()
	
	var configs []*models.OIDCConfig
	for rows.Next() {
		config := &models.OIDCConfig{}
		err := rows.Scan(
			&config.ID,
			&config.Provider,
			&config.Issuer,
			&config.Domain,
			&config.ClientID,
			&config.ClientSecret,
			&config.RedirectURI,
			&config.JWKSUrl,
			&config.CreatedAt,
			&config.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan OIDC config: %w", err)
		}
		configs = append(configs, config)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating OIDC configs: %w", err)
	}
	
	return configs, nil
}

// Update updates an existing OIDC configuration
func (r *OIDCConfigRepository) Update(ctx context.Context, config *models.OIDCConfig) error {
	query := `
		UPDATE oidc_config
		SET issuer = $2, domain = $3, client_id = $4, client_secret = $5, redirect_uri = $6, jwks_url = $7, updated_at = $8
		WHERE provider = $1
		RETURNING updated_at
	`
	
	now := time.Now()
	err := r.db.QueryRowContext(ctx, query,
		config.Provider,
		config.Issuer,
		config.Domain,
		config.ClientID,
		config.ClientSecret,
		config.RedirectURI,
		config.JWKSUrl,
		now,
	).Scan(&config.UpdatedAt)
	
	if err == sql.ErrNoRows {
		return fmt.Errorf("OIDC config not found")
	}
	if err != nil {
		return fmt.Errorf("failed to update OIDC config: %w", err)
	}
	
	return nil
}

// Delete deletes an OIDC configuration by provider
func (r *OIDCConfigRepository) Delete(ctx context.Context, provider string) error {
	query := `DELETE FROM oidc_config WHERE provider = $1`
	
	result, err := r.db.ExecContext(ctx, query, provider)
	if err != nil {
		return fmt.Errorf("failed to delete OIDC config: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("OIDC config not found")
	}
	
	return nil
}
