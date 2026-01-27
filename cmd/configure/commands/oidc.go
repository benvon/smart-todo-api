package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/benvon/smart-todo/internal/config"
	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// NewOIDCCmd creates the OIDC configuration command
func NewOIDCCmd() *cobra.Command {
	var issuer, domain, clientID, clientSecret, redirectURI string

	cmd := &cobra.Command{
		Use:   "oidc <provider-name>",
		Short: "Configure OIDC provider",
		Long:  "Configure an OIDC provider for authentication. Provider name can be any identifier (e.g., 'cognito', 'okta', 'auth0')",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := args[0]

			// Validate provider name (basic validation - must not be empty)
			if provider == "" {
				return fmt.Errorf("provider name cannot be empty")
			}

			if issuer == "" || clientID == "" || redirectURI == "" {
				return fmt.Errorf("required flags: --issuer, --client-id, --redirect-uri (--client-secret is optional for public clients)")
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			db, err := database.New(cfg.DatabaseURL)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer func() {
				if err := db.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to close database: %v\n", err)
				}
			}()

			oidcRepo := database.NewOIDCConfigRepository(db)
			ctx := context.Background()

			// Check if config already exists
			existing, err := oidcRepo.GetByProvider(ctx, provider)
			if err == nil && existing != nil {
				// Update existing
				existing.Issuer = issuer
				if domain != "" {
					existing.Domain = &domain
				}
				existing.ClientID = clientID
				if clientSecret != "" {
					existing.ClientSecret = &clientSecret
				} else {
					existing.ClientSecret = nil
				}
				existing.RedirectURI = redirectURI
				// Try to derive JWKS URL from issuer
				jwksURL := issuer + "/.well-known/jwks.json"
				existing.JWKSUrl = &jwksURL

				if err := oidcRepo.Update(ctx, existing); err != nil {
					return fmt.Errorf("failed to update OIDC config: %w", err)
				}
				fmt.Printf("Updated OIDC configuration for provider: %s\n", provider)
			} else {
				// Create new
				config := &models.OIDCConfig{
					ID:          uuid.New(),
					Provider:    provider,
					Issuer:      issuer,
					ClientID:    clientID,
					RedirectURI: redirectURI,
				}
				if domain != "" {
					config.Domain = &domain
				}
				if clientSecret != "" {
					config.ClientSecret = &clientSecret
				}
				jwksURL := issuer + "/.well-known/jwks.json"
				config.JWKSUrl = &jwksURL

				if err := oidcRepo.Create(ctx, config); err != nil {
					return fmt.Errorf("failed to create OIDC config: %w", err)
				}
				fmt.Printf("Created OIDC configuration for provider: %s\n", provider)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&issuer, "issuer", "", "OIDC issuer URL (required)")
	cmd.Flags().StringVar(&domain, "domain", "", "OAuth2 domain (optional, e.g., for Cognito custom domains like 'idp.benvon.net')")
	cmd.Flags().StringVar(&clientID, "client-id", "", "OAuth2 client ID (required)")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "OAuth2 client secret (optional for public clients like Cognito SPAs)")
	cmd.Flags().StringVar(&redirectURI, "redirect-uri", "", "OAuth2 redirect URI (required)")

	return cmd
}
