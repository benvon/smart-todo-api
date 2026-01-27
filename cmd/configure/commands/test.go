package commands

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/benvon/smart-todo/internal/config"
	"github.com/benvon/smart-todo/internal/database"
	"github.com/spf13/cobra"
)

// NewTestCmd creates the test command
func NewTestCmd() *cobra.Command {
	var provider string

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test OIDC configuration",
		Long:  "Test OIDC provider configuration by validating endpoints",
		RunE: func(cmd *cobra.Command, args []string) error {
			if provider == "" {
				return fmt.Errorf("--provider is required")
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

			config, err := oidcRepo.GetByProvider(ctx, provider)
			if err != nil {
				return fmt.Errorf("failed to get OIDC config: %w", err)
			}

			fmt.Printf("Testing OIDC configuration for provider: %s\n", provider)
			fmt.Printf("Issuer: %s\n", config.Issuer)

			// Test issuer discovery endpoint
			discoveryURL := config.Issuer + "/.well-known/openid-configuration"
			fmt.Printf("\nTesting discovery endpoint: %s\n", discoveryURL)
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(discoveryURL)
			if err != nil {
				return fmt.Errorf("failed to reach discovery endpoint: %w", err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to close response body: %v\n", err)
				}
			}()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("discovery endpoint returned status: %d", resp.StatusCode)
			}
			fmt.Println("✓ Discovery endpoint is accessible")

			// Test JWKS endpoint if available
			if config.JWKSUrl != nil {
				fmt.Printf("\nTesting JWKS endpoint: %s\n", *config.JWKSUrl)
				resp, err := client.Get(*config.JWKSUrl)
				if err != nil {
					return fmt.Errorf("failed to reach JWKS endpoint: %w", err)
				}
				defer func() {
					if err := resp.Body.Close(); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to close response body: %v\n", err)
					}
				}()

				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("JWKS endpoint returned status: %d", resp.StatusCode)
				}
				fmt.Println("✓ JWKS endpoint is accessible")
			}

			fmt.Println("\n✓ OIDC configuration test passed")
			return nil
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Provider name to test (required)")

	return cmd
}
