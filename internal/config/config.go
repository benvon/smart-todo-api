package config

import (
	"fmt"
	"os"
)

// Config holds application configuration
type Config struct {
	DatabaseURL   string
	ServerPort    string
	BaseURL       string
	FrontendURL   string
	OpenAIKey     string
	EnableHSTS    bool
	OIDCProvider  string
	RedisURL      string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:  getEnv("DATABASE_URL", ""),
		ServerPort:   getEnv("SERVER_PORT", "8080"),
		BaseURL:      getEnv("BASE_URL", "http://localhost:8080"),
		FrontendURL:  getEnv("FRONTEND_URL", "http://localhost:3000"),
		OpenAIKey:    getEnv("OPENAI_API_KEY", ""),
		EnableHSTS:   getEnvBool("ENABLE_HSTS", false),
		OIDCProvider: getEnv("OIDC_PROVIDER", "cognito"),
		RedisURL:     getEnv("REDIS_URL", "redis://localhost:6379/0"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}
