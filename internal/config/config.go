package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds application configuration
type Config struct {
	DatabaseURL      string
	ServerPort       string
	BaseURL          string
	FrontendURL      string
	OpenAIKey        string
	AIProvider       string
	AIModel          string
	AIBaseURL        string
	EnableHSTS       bool
	OIDCProvider     string
	RedisURL         string
	RabbitMQURL      string
	RabbitMQPrefetch int
	WorkerDebugMode  bool
	ServerDebugMode  bool
	OTELEnabled      bool
	OTELEndpoint     string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:      getEnv("DATABASE_URL", ""),
		ServerPort:       getEnv("SERVER_PORT", "8080"),
		BaseURL:          getEnv("BASE_URL", "http://localhost:8080"),
		FrontendURL:      getEnv("FRONTEND_URL", "http://localhost:3000"),
		OpenAIKey:        getEnv("OPENAI_API_KEY", ""),
		AIProvider:       getEnv("AI_PROVIDER", "openai"),
		AIModel:          getEnv("AI_MODEL", ""),
		AIBaseURL:        getEnv("AI_BASE_URL", ""),
		EnableHSTS:       getEnvBool("ENABLE_HSTS", false),
		OIDCProvider:     getEnv("OIDC_PROVIDER", "cognito"),
		RedisURL:         getEnv("REDIS_URL", "redis://localhost:6379/0"),
		RabbitMQURL:      getEnv("RABBITMQ_URL", ""),
		RabbitMQPrefetch: getEnvInt("RABBITMQ_PREFETCH", 1),
		WorkerDebugMode:  getEnvBool("WORKER_DEBUG_MODE", false),
		ServerDebugMode:  getEnvBool("SERVER_DEBUG_MODE", false),
		OTELEnabled:      getEnvBool("OTEL_ENABLED", false),
		OTELEndpoint:     getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.RabbitMQURL == "" {
		return nil, fmt.Errorf("RABBITMQ_URL is required for job queueing (AI features require RabbitMQ)")
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

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
