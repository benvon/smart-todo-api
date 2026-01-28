package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/models"
)

// Provider manages OIDC provider configuration
type Provider struct {
	repo *database.OIDCConfigRepository
}

// NewProvider creates a new OIDC provider manager
func NewProvider(repo *database.OIDCConfigRepository) *Provider {
	return &Provider{repo: repo}
}

// GetConfig retrieves OIDC configuration for a provider
func (p *Provider) GetConfig(ctx context.Context, providerName string) (*models.OIDCConfig, error) {
	config, err := p.repo.GetByProvider(ctx, providerName)
	if err != nil {
		return nil, fmt.Errorf("failed to get OIDC config: %w", err)
	}
	return config, nil
}

// GetLoginConfig returns the configuration needed for frontend OIDC login
func (p *Provider) GetLoginConfig(ctx context.Context, providerName string) (*LoginConfig, error) {
	config, err := p.GetConfig(ctx, providerName)
	if err != nil {
		return nil, err
	}

	// Try to fetch authorization endpoint from OIDC discovery document
	// Fall back to constructing it from issuer if discovery fails
	var authEndpoint string
	discoveryURL := config.Issuer + "/.well-known/openid-configuration"
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(discoveryURL)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil {
				// Log error but don't fail the request - body already read
				// Use standard log here since OIDC provider doesn't have logger
				// This is a minor cleanup error, can be ignored
				_ = closeErr
			}
		}()
		var discovery struct {
			AuthorizationEndpoint string `json:"authorization_endpoint"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&discovery); err == nil && discovery.AuthorizationEndpoint != "" {
			authEndpoint = discovery.AuthorizationEndpoint
		}
	}

	// Fallback: construct from issuer if discovery didn't work
	if authEndpoint == "" {
		if len(config.Issuer) > 0 && config.Issuer[len(config.Issuer)-1] == '/' {
			authEndpoint = config.Issuer + "oauth2/authorize"
		} else {
			authEndpoint = config.Issuer + "/oauth2/authorize"
		}
	}

	var tokenEndpoint string
	// For Cognito, if a domain is configured, use it for the OAuth2 endpoints
	// This is required because Cognito OAuth2 flows require domain-based endpoints, not issuer-based ones
	if config.Domain != nil && *config.Domain != "" && strings.Contains(config.Issuer, "cognito-idp.") {
		// Use domain to construct OAuth2 endpoints
		// Domain can be either a custom domain (idp.benvon.net) or Cognito domain format
		domain := *config.Domain
		var baseURL string
		if strings.Contains(domain, ".") && !strings.Contains(domain, "auth.") {
			// Custom domain - use as-is
			baseURL = fmt.Sprintf("https://%s", domain)
		} else {
			// Assume it's already in the correct format
			if strings.HasPrefix(domain, "https://") {
				baseURL = domain
			} else {
				baseURL = fmt.Sprintf("https://%s", domain)
			}
		}
		authEndpoint = fmt.Sprintf("%s/oauth2/authorize", baseURL)
		tokenEndpoint = fmt.Sprintf("%s/oauth2/token", baseURL)
	} else {
		// Fallback: construct token endpoint from issuer
		if len(config.Issuer) > 0 && config.Issuer[len(config.Issuer)-1] == '/' {
			tokenEndpoint = config.Issuer + "oauth2/token"
		} else {
			tokenEndpoint = config.Issuer + "/oauth2/token"
		}
	}

	return &LoginConfig{
		AuthorizationEndpoint: authEndpoint,
		TokenEndpoint:         tokenEndpoint,
		ClientID:              config.ClientID,
		RedirectURI:           config.RedirectURI,
		Scope:                 "openid email profile",
	}, nil
}

// LoginConfig contains OIDC login configuration for frontend
type LoginConfig struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	ClientID              string `json:"client_id"`
	RedirectURI           string `json:"redirect_uri"`
	Scope                 string `json:"scope"`
}
