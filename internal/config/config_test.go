package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func TestLoad(t *testing.T) {
	// Do not run in parallel - environment variables are global state

	tests := []struct {
		name        string
		envVars     map[string]string
		expectError bool
		validate    func(*testing.T, *Config)
	}{
		{
			name: "all required env vars set",
			envVars: map[string]string{
				"DATABASE_URL": "postgres://user:pass@localhost/db",
				"RABBITMQ_URL": "amqp://guest:guest@localhost:5672/",
				"SERVER_PORT":  "9090",
				"BASE_URL":     "http://localhost:9090",
			},
			expectError: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.DatabaseURL != "postgres://user:pass@localhost/db" {
					t.Errorf("Expected DatabaseURL to be 'postgres://user:pass@localhost/db', got '%s'", cfg.DatabaseURL)
				}
				if cfg.RabbitMQURL != "amqp://guest:guest@localhost:5672/" {
					t.Errorf("Expected RabbitMQURL to be 'amqp://guest:guest@localhost:5672/', got '%s'", cfg.RabbitMQURL)
				}
				if cfg.ServerPort != "9090" {
					t.Errorf("Expected ServerPort to be '9090', got '%s'", cfg.ServerPort)
				}
				if cfg.BaseURL != "http://localhost:9090" {
					t.Errorf("Expected BaseURL to be 'http://localhost:9090', got '%s'", cfg.BaseURL)
				}
			},
		},
		{
			name: "missing DATABASE_URL",
			envVars: map[string]string{
				"DATABASE_URL": "",
				"SERVER_PORT":  "9090",
			},
			expectError: true,
			validate: func(t *testing.T, cfg *Config) {
				// This should not be called when expectError is true, but if it is, cfg should be nil
				if cfg != nil {
					t.Error("Expected config to be nil when error occurs")
				}
			},
		},
		{
			name: "default values",
			envVars: map[string]string{
				"DATABASE_URL": "postgres://user:pass@localhost/db",
				"RABBITMQ_URL": "amqp://guest:guest@localhost:5672/",
				"SERVER_PORT":  "",
				"BASE_URL":     "",
			},
			expectError: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.ServerPort != "8080" {
					t.Errorf("Expected default ServerPort to be '8080', got '%s'", cfg.ServerPort)
				}
				if cfg.BaseURL != "http://localhost:8080" {
					t.Errorf("Expected default BaseURL to be 'http://localhost:8080', got '%s'", cfg.BaseURL)
				}
				if cfg.FrontendURL != "http://localhost:3000" {
					t.Errorf("Expected default FrontendURL to be 'http://localhost:3000', got '%s'", cfg.FrontendURL)
				}
				if cfg.EnableHSTS != false {
					t.Errorf("Expected default EnableHSTS to be false, got %v", cfg.EnableHSTS)
				}
				if cfg.OIDCProvider != "cognito" {
					t.Errorf("Expected default OIDCProvider to be 'cognito', got '%s'", cfg.OIDCProvider)
				}
				if cfg.RedisURL != "redis://localhost:6379/0" {
					t.Errorf("Expected default RedisURL to be 'redis://localhost:6379/0', got '%s'", cfg.RedisURL)
				}
				if cfg.RabbitMQPrefetch != 1 {
					t.Errorf("Expected default RabbitMQPrefetch to be 1, got %d", cfg.RabbitMQPrefetch)
				}
			},
		},
		{
			name: "OPENAI_API_KEY optional",
			envVars: map[string]string{
				"DATABASE_URL":   "postgres://user:pass@localhost/db",
				"RABBITMQ_URL":   "amqp://guest:guest@localhost:5672/",
				"OPENAI_API_KEY": "sk-test-key",
			},
			expectError: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.OpenAIKey != "sk-test-key" {
					t.Errorf("Expected OpenAIKey to be 'sk-test-key', got '%s'", cfg.OpenAIKey)
				}
			},
		},
		{
			name: "missing RABBITMQ_URL",
			envVars: map[string]string{
				"DATABASE_URL": "postgres://user:pass@localhost/db",
				"RABBITMQ_URL": "",
			},
			expectError: true,
			validate: func(t *testing.T, cfg *Config) {
				// This should not be called when expectError is true, but if it is, cfg should be nil
				if cfg != nil {
					t.Error("Expected config to be nil when error occurs")
				}
			},
		},
	}

	// All config-related env vars that might be modified
	allConfigEnvVars := []string{
		"DATABASE_URL",
		"RABBITMQ_URL",
		"SERVER_PORT",
		"BASE_URL",
		"FRONTEND_URL",
		"OPENAI_API_KEY",
		"ENABLE_HSTS",
		"OIDC_PROVIDER",
		"REDIS_URL",
		"RABBITMQ_PREFETCH",
		"AI_PROVIDER",
		"AI_MODEL",
		"AI_BASE_URL",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env vars for all config-related vars
			originalEnv := make(map[string]string)
			for _, key := range allConfigEnvVars {
				originalEnv[key] = os.Getenv(key)
			}

			// Clear all config-related env vars before setting test-specific ones
			for _, key := range allConfigEnvVars {
				if err := os.Unsetenv(key); err != nil {
					t.Logf("Warning: failed to unset %s: %v", key, err)
				}
			}

			// Set test env vars
			for key, value := range tt.envVars {
				if value == "" {
					if err := os.Unsetenv(key); err != nil {
						t.Logf("Warning: failed to unset %s: %v", key, err)
					}
				} else {
					if err := os.Setenv(key, value); err != nil {
						t.Fatalf("Failed to set env var %s: %v", key, err)
					}
				}
			}

			// Cleanup: restore original env vars
			defer func() {
				for key, value := range originalEnv {
					if value != "" {
						if err := os.Setenv(key, value); err != nil {
							t.Logf("Warning: failed to restore %s: %v", key, err)
						}
					} else {
						if err := os.Unsetenv(key); err != nil {
							t.Logf("Warning: failed to unset %s: %v", key, err)
						}
					}
				}
			}()

			cfg, err := Load()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
					return
				}
				// Validate error message contains expected content
				errMsg := err.Error()
				if errMsg == "" {
					t.Error("Expected error message but got empty string")
				}
				// Check for specific error messages based on test case
				if tt.name == "missing DATABASE_URL" && !errors.Is(err, fmt.Errorf("DATABASE_URL is required")) {
					if !contains(errMsg, "DATABASE_URL") {
						t.Errorf("Expected error to mention DATABASE_URL, got: %s", errMsg)
					}
				}
				if tt.name == "missing RABBITMQ_URL" && !errors.Is(err, fmt.Errorf("RABBITMQ_URL is required")) {
					if !contains(errMsg, "RABBITMQ_URL") {
						t.Errorf("Expected error to mention RABBITMQ_URL, got: %s", errMsg)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if cfg == nil {
				t.Fatal("Config is nil")
			}

			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	// Do not run in parallel - environment variables are global state

	tests := []struct {
		name         string
		key          string
		value        string
		defaultValue string
		want         string
	}{
		{
			name:         "env var set",
			key:          "TEST_KEY",
			value:        "test-value",
			defaultValue: "default",
			want:         "test-value",
		},
		{
			name:         "env var not set",
			key:          "TEST_KEY_NOT_SET",
			value:        "",
			defaultValue: "default",
			want:         "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			original := os.Getenv(tt.key)

			if tt.value != "" {
				if err := os.Setenv(tt.key, tt.value); err != nil {
					t.Fatalf("Failed to set env var %s: %v", tt.key, err)
				}
			} else {
				if err := os.Unsetenv(tt.key); err != nil {
					t.Logf("Warning: failed to unset %s: %v", tt.key, err)
				}
			}

			defer func() {
				if original != "" {
					if err := os.Setenv(tt.key, original); err != nil {
						t.Logf("Warning: failed to restore %s: %v", tt.key, err)
					}
				} else {
					if err := os.Unsetenv(tt.key); err != nil {
						t.Logf("Warning: failed to unset %s: %v", tt.key, err)
					}
				}
			}()

			got := getEnv(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnv(%s, %s) = %s, want %s", tt.key, tt.defaultValue, got, tt.want)
			}
		})
	}
}

func TestGetEnvBool(t *testing.T) {
	// Do not run in parallel - environment variables are global state

	tests := []struct {
		name         string
		key          string
		value        string
		defaultValue bool
		want         bool
	}{
		{
			name:         "env var set to 'true'",
			key:          "TEST_BOOL_KEY",
			value:        "true",
			defaultValue: false,
			want:         true,
		},
		{
			name:         "env var set to '1'",
			key:          "TEST_BOOL_KEY",
			value:        "1",
			defaultValue: false,
			want:         true,
		},
		{
			name:         "env var set to 'yes'",
			key:          "TEST_BOOL_KEY",
			value:        "yes",
			defaultValue: false,
			want:         true,
		},
		{
			name:         "env var set to 'false'",
			key:          "TEST_BOOL_KEY",
			value:        "false",
			defaultValue: true,
			want:         false,
		},
		{
			name:         "env var not set",
			key:          "TEST_BOOL_KEY_NOT_SET",
			value:        "",
			defaultValue: false,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			original := os.Getenv(tt.key)

			if tt.value != "" {
				if err := os.Setenv(tt.key, tt.value); err != nil {
					t.Fatalf("Failed to set env var %s: %v", tt.key, err)
				}
			} else {
				if err := os.Unsetenv(tt.key); err != nil {
					t.Logf("Warning: failed to unset %s: %v", tt.key, err)
				}
			}

			defer func() {
				if original != "" {
					if err := os.Setenv(tt.key, original); err != nil {
						t.Logf("Warning: failed to restore %s: %v", tt.key, err)
					}
				} else {
					if err := os.Unsetenv(tt.key); err != nil {
						t.Logf("Warning: failed to unset %s: %v", tt.key, err)
					}
				}
			}()

			got := getEnvBool(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvBool(%s, %v) = %v, want %v", tt.key, tt.defaultValue, got, tt.want)
			}
		})
	}
}
