package main

import (
	"fmt"
	"os"

	"github.com/benvon/smart-todo/cmd/configure/commands"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "smart-todo-configure",
		Short: "Configuration tool for Smart Todo API",
		Long:  "CLI tool for configuring OIDC providers and other settings",
	}

	rootCmd.AddCommand(commands.NewOIDCCmd())
	rootCmd.AddCommand(commands.NewListCmd())
	rootCmd.AddCommand(commands.NewTestCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
