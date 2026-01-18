package models

import (
	"time"

	"github.com/google/uuid"
)

// OIDCConfig represents OIDC provider configuration
type OIDCConfig struct {
	ID          uuid.UUID `json:"id"`
	Provider    string    `json:"provider"`
	Issuer      string    `json:"issuer"`
	Domain      *string   `json:"domain,omitempty"` // Optional: OAuth2 domain (e.g., for Cognito custom domains)
	ClientID    string    `json:"client_id"`
	ClientSecret *string  `json:"client_secret,omitempty"` // Optional for public OIDC clients
	RedirectURI string    `json:"redirect_uri"`
	JWKSUrl     *string   `json:"jwks_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
