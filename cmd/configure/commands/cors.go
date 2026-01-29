package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/benvon/smart-todo/internal/config"
	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/spf13/cobra"
)

// NewCorsCmd creates the cors configuration command with list and set subcommands.
func NewCorsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cors",
		Short: "Manage CORS configuration",
		Long:  "List or update CORS allowed origins and options (stored in database).",
	}
	cmd.AddCommand(newCorsListCmd())
	cmd.AddCommand(newCorsSetCmd())
	return cmd
}

func newCorsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List current CORS configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			db, err := database.New(cfg.DatabaseURL)
			if err != nil {
				return fmt.Errorf("connect to database: %w", err)
			}
			defer func() {
				_ = db.Close()
			}()
			repo := database.NewCorsConfigRepository(db)
			c, err := repo.Get(context.Background())
			if err != nil {
				return fmt.Errorf("get cors config: %w", err)
			}
			if c == nil {
				fmt.Println("No CORS configuration in database. Use 'cors set' to add one.")
				return nil
			}
			fmt.Println("CORS configuration:")
			fmt.Printf("  Allowed origins: %s\n", c.AllowedOrigins)
			fmt.Printf("  Allow credentials: %v\n", c.AllowCredentials)
			fmt.Printf("  Max-Age: %d\n", c.MaxAge)
			return nil
		},
	}
}

func newCorsSetCmd() *cobra.Command {
	var origins string
	var allowCreds bool
	var maxAge int
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set CORS configuration",
		Long:  "Update CORS allowed origins (comma-separated). Stored in database.",
		RunE: func(cmd *cobra.Command, args []string) error {
			origins = strings.TrimSpace(origins)
			if origins == "" {
				return fmt.Errorf("--origins is required (comma-separated list)")
			}
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			db, err := database.New(cfg.DatabaseURL)
			if err != nil {
				return fmt.Errorf("connect to database: %w", err)
			}
			defer func() {
				_ = db.Close()
			}()
			repo := database.NewCorsConfigRepository(db)
			c := &models.CorsConfig{
				AllowedOrigins:   origins,
				AllowCredentials: allowCreds,
				MaxAge:           maxAge,
			}
			if err := repo.Set(context.Background(), c); err != nil {
				return fmt.Errorf("set cors config: %w", err)
			}
			fmt.Println("CORS configuration updated.")
			return nil
		},
	}
	cmd.Flags().StringVar(&origins, "origins", "", "Comma-separated allowed origins (required)")
	cmd.Flags().BoolVar(&allowCreds, "allow-credentials", true, "Allow credentials")
	cmd.Flags().IntVar(&maxAge, "max-age", 86400, "Access-Control-Max-Age (seconds)")
	return cmd
}
