package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/benvon/smart-todo/internal/config"
	"github.com/benvon/smart-todo/internal/database"
	"github.com/spf13/cobra"
)

// NewListCmd creates the list command
func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured OIDC providers",
		Long:  "List all configured OIDC providers",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			configs, err := oidcRepo.GetAll(ctx)
			if err != nil {
				return fmt.Errorf("failed to list OIDC configs: %w", err)
			}

			if len(configs) == 0 {
				fmt.Println("No OIDC providers configured")
				return nil
			}

			fmt.Println("Configured OIDC providers:")
			for _, config := range configs {
				fmt.Printf("  - Provider: %s\n", config.Provider)
				fmt.Printf("    Issuer: %s\n", config.Issuer)
				fmt.Printf("    Client ID: %s\n", config.ClientID)
				fmt.Printf("    Redirect URI: %s\n", config.RedirectURI)
				if config.JWKSUrl != nil {
					fmt.Printf("    JWKS URL: %s\n", *config.JWKSUrl)
				}
				fmt.Println()
			}

			return nil
		},
	}

	return cmd
}
