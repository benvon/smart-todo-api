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
	authEndpoint := getAuthEndpoint(config)
	tokenEndpoint := getTokenEndpoint(config)
	return &LoginConfig{
		AuthorizationEndpoint: authEndpoint,
		TokenEndpoint:         tokenEndpoint,
		ClientID:              config.ClientID,
		RedirectURI:           config.RedirectURI,
		Scope:                 "openid email profile",
	}, nil
}

func getAuthEndpoint(config *models.OIDCConfig) string {
	if config.Domain != nil && *config.Domain != "" && strings.Contains(config.Issuer, "cognito-idp.") {
		return fmt.Sprintf("%s/oauth2/authorize", cognitoDomainBaseURL(*config.Domain))
	}
	if s := fetchAuthEndpointFromDiscovery(config.Issuer); s != "" {
		return s
	}
	return authEndpointFromIssuer(config.Issuer)
}

func fetchAuthEndpointFromDiscovery(issuer string) string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(issuer + "/.well-known/openid-configuration")
	if err != nil || resp.StatusCode != http.StatusOK {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	var discovery struct {
		AuthorizationEndpoint string `json:"authorization_endpoint"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil || discovery.AuthorizationEndpoint == "" {
		return ""
	}
	return discovery.AuthorizationEndpoint
}

func authEndpointFromIssuer(issuer string) string {
	if len(issuer) > 0 && issuer[len(issuer)-1] == '/' {
		return issuer + "oauth2/authorize"
	}
	return issuer + "/oauth2/authorize"
}

func getTokenEndpoint(config *models.OIDCConfig) string {
	if config.Domain != nil && *config.Domain != "" && strings.Contains(config.Issuer, "cognito-idp.") {
		baseURL := cognitoDomainBaseURL(*config.Domain)
		return fmt.Sprintf("%s/oauth2/token", baseURL)
	}
	return tokenEndpointFromIssuer(config.Issuer)
}

func cognitoDomainBaseURL(domain string) string {
	if strings.HasPrefix(domain, "https://") {
		return domain
	}
	return fmt.Sprintf("https://%s", domain)
}

func tokenEndpointFromIssuer(issuer string) string {
	if len(issuer) > 0 && issuer[len(issuer)-1] == '/' {
		return issuer + "oauth2/token"
	}
	return issuer + "/oauth2/token"
}

// LoginConfig contains OIDC login configuration for frontend
type LoginConfig struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	ClientID              string `json:"client_id"`
	RedirectURI           string `json:"redirect_uri"`
	Scope                 string `json:"scope"`
}
