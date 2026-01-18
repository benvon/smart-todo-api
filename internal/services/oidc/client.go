package oidc

import (
	"context"

	"golang.org/x/oauth2"
	"github.com/benvon/smart-todo/internal/models"
)

// Client wraps OAuth2 client functionality
type Client struct {
	config *oauth2.Config
}

// NewClient creates a new OAuth2 client from OIDC config
func NewClient(oidcConfig *models.OIDCConfig) *Client {
	clientSecret := ""
	if oidcConfig.ClientSecret != nil {
		clientSecret = *oidcConfig.ClientSecret
	}
	
	config := &oauth2.Config{
		ClientID:     oidcConfig.ClientID,
		ClientSecret: clientSecret,
		RedirectURL:  oidcConfig.RedirectURI,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  oidcConfig.Issuer + "/oauth2/authorize",
			TokenURL: oidcConfig.Issuer + "/oauth2/token",
		},
	}

	return &Client{config: config}
}

// ExchangeCode exchanges an authorization code for tokens
func (c *Client) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	return c.config.Exchange(ctx, code)
}

// AuthCodeURL returns the authorization URL
func (c *Client) AuthCodeURL(state string) string {
	return c.config.AuthCodeURL(state)
}
