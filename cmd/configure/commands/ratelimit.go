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

// NewRatelimitCmd creates the ratelimit configuration command with list and set subcommands.
func NewRatelimitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ratelimit",
		Short: "Manage rate limit configuration",
		Long:  "List or update rate limit (e.g. 5-S, 100-M). Stored in database.",
	}
	cmd.AddCommand(newRatelimitListCmd())
	cmd.AddCommand(newRatelimitSetCmd())
	return cmd
}

func newRatelimitListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List current rate limit configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			db, err := database.New(cfg.DatabaseURL)
			if err != nil {
				return fmt.Errorf("connect to database: %w", err)
			}
			defer func() { _ = db.Close() }()
			repo := database.NewRatelimitConfigRepository(db)
			c, err := repo.Get(context.Background())
			if err != nil {
				return fmt.Errorf("get ratelimit config: %w", err)
			}
			if c == nil {
				fmt.Println("No rate limit configuration in database. Use 'ratelimit set' to add one.")
				return nil
			}
			fmt.Println("Rate limit configuration:")
			fmt.Printf("  Rate: %s\n", c.Rate)
			return nil
		},
	}
}

func newRatelimitSetCmd() *cobra.Command {
	var rate string
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set rate limit configuration",
		Long:  "Update rate limit (e.g. 5-S, 100-M, 1000-H). Stored in database.",
		RunE: func(cmd *cobra.Command, args []string) error {
			rate = strings.TrimSpace(rate)
			if rate == "" {
				return fmt.Errorf("--rate is required (e.g. 5-S, 100-M)")
			}
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			db, err := database.New(cfg.DatabaseURL)
			if err != nil {
				return fmt.Errorf("connect to database: %w", err)
			}
			defer func() { _ = db.Close() }()
			repo := database.NewRatelimitConfigRepository(db)
			c := &models.RatelimitConfig{Rate: rate}
			if err := repo.Set(context.Background(), c); err != nil {
				return fmt.Errorf("set ratelimit config: %w", err)
			}
			fmt.Println("Rate limit configuration updated.")
			return nil
		},
	}
	cmd.Flags().StringVar(&rate, "rate", "", "Rate (e.g. 5-S, 100-M, 1000-H) (required)")
	return cmd
}
