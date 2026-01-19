package config

import (
	"os"
	"sync"
	"testing"
)

var envMutex sync.Mutex

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
				"DATABASE_URL": "",
				"SERVER_PORT":  "9090",
			},
			expectError: true,
		},
		{
			name: "default values",
			envVars: map[string]string{
				"DATABASE_URL": "postgres://user:pass@localhost/db",
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

	// All config-related env vars that might be modified
	allConfigEnvVars := []string{
		"DATABASE_URL",
		"SERVER_PORT",
		"BASE_URL",
		"FRONTEND_URL",
		"OPENAI_API_KEY",
		"ENABLE_HSTS",
		"OIDC_PROVIDER",
		"REDIS_URL",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			envMutex.Lock()
			// Save original env vars for all config-related vars
			originalEnv := make(map[string]string)
			for _, key := range allConfigEnvVars {
				originalEnv[key] = os.Getenv(key)
			}

			// Clear only the env vars that this test will modify
			for key := range tt.envVars {
				_ = os.Unsetenv(key) // Ignore error in test setup
			}

			// Set test env vars
			for key, value := range tt.envVars {
				if value == "" {
					_ = os.Unsetenv(key) // Ignore error in test setup
				} else {
					_ = os.Setenv(key, value) // Ignore error in test setup
				}
			}
			envMutex.Unlock()

			// Cleanup: restore original env vars
			defer func() {
				envMutex.Lock()
				defer envMutex.Unlock()
				for key, value := range originalEnv {
					if value != "" {
						_ = os.Setenv(key, value) // Ignore error in test cleanup
					} else {
						_ = os.Unsetenv(key) // Ignore error in test cleanup
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

			envMutex.Lock()
			// Save original value
			original := os.Getenv(tt.key)

			if tt.value != "" {
				_ = os.Setenv(tt.key, tt.value) // Ignore error in test setup
			} else {
				_ = os.Unsetenv(tt.key) // Ignore error in test setup
			}
			envMutex.Unlock()

			defer func() {
				envMutex.Lock()
				defer envMutex.Unlock()
				if original != "" {
					_ = os.Setenv(tt.key, original) // Ignore error in test cleanup
				} else {
					_ = os.Unsetenv(tt.key) // Ignore error in test cleanup
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
	t.Parallel()

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
			t.Parallel()

			envMutex.Lock()
			// Save original value
			original := os.Getenv(tt.key)

			if tt.value != "" {
				_ = os.Setenv(tt.key, tt.value) // Ignore error in test setup
			} else {
				_ = os.Unsetenv(tt.key) // Ignore error in test setup
			}
			envMutex.Unlock()

			defer func() {
				envMutex.Lock()
				defer envMutex.Unlock()
				if original != "" {
					_ = os.Setenv(tt.key, original) // Ignore error in test cleanup
				} else {
					_ = os.Unsetenv(tt.key) // Ignore error in test cleanup
				}
			}()

			got := getEnvBool(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvBool(%s, %v) = %v, want %v", tt.key, tt.defaultValue, got, tt.want)
			}
		})
	}
}
