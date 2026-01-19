package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Parallel()

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
				"SERVER_PORT":  "9090",
				"BASE_URL":     "http://localhost:9090",
			},
			expectError: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.DatabaseURL != "postgres://user:pass@localhost/db" {
					t.Errorf("Expected DatabaseURL to be 'postgres://user:pass@localhost/db', got '%s'", cfg.DatabaseURL)
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
				"SERVER_PORT": "9090",
			},
			expectError: true,
		},
		{
			name: "default values",
			envVars: map[string]string{
				"DATABASE_URL": "postgres://user:pass@localhost/db",
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
			},
		},
		{
			name: "OPENAI_API_KEY optional",
			envVars: map[string]string{
				"DATABASE_URL": "postgres://user:pass@localhost/db",
				"OPENAI_API_KEY": "sk-test-key",
			},
			expectError: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.OpenAIKey != "sk-test-key" {
					t.Errorf("Expected OpenAIKey to be 'sk-test-key', got '%s'", cfg.OpenAIKey)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Save original env vars
			originalEnv := make(map[string]string)
			for key := range tt.envVars {
				originalEnv[key] = os.Getenv(key)
			}

			// Clear relevant env vars
			for key := range tt.envVars {
				os.Unsetenv(key)
			}

			// Set test env vars
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Cleanup
			defer func() {
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
				for key, value := range originalEnv {
					if value != "" {
						os.Setenv(key, value)
					}
				}
			}()

			cfg, err := Load()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
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
	t.Parallel()

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
			t.Parallel()

			// Save original value
			original := os.Getenv(tt.key)
			defer os.Setenv(tt.key, original)

			if tt.value != "" {
				os.Setenv(tt.key, tt.value)
			} else {
				os.Unsetenv(tt.key)
			}

			got := getEnv(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnv(%s, %s) = %s, want %s", tt.key, tt.defaultValue, got, tt.want)
			}
		})
	}
}
