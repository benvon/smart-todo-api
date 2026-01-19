package oidc

import (
	"context"
	"testing"

	"github.com/benvon/smart-todo/internal/models"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		oidcConfig *models.OIDCConfig
		validate  func(*testing.T, *Client)
	}{
		{
			name: "with client secret",
			oidcConfig: &models.OIDCConfig{
				ClientID:     "test-client-id",
				ClientSecret: stringPtr("test-secret"),
				RedirectURI:  "http://localhost:3000/callback",
				Issuer:       "https://auth.example.com",
			},
			validate: func(t *testing.T, client *Client) {
				if client == nil {
					t.Fatal("Client is nil")
				}
				if client.config == nil {
					t.Fatal("OAuth2 config is nil")
				}
				if client.config.ClientID != "test-client-id" {
					t.Errorf("Expected ClientID 'test-client-id', got '%s'", client.config.ClientID)
				}
				if client.config.ClientSecret != "test-secret" {
					t.Errorf("Expected ClientSecret 'test-secret', got '%s'", client.config.ClientSecret)
				}
				if client.config.RedirectURL != "http://localhost:3000/callback" {
					t.Errorf("Expected RedirectURL 'http://localhost:3000/callback', got '%s'", client.config.RedirectURL)
				}
			},
		},
		{
			name: "without client secret (public client)",
			oidcConfig: &models.OIDCConfig{
				ClientID:     "test-client-id",
				ClientSecret: nil,
				RedirectURI:  "http://localhost:3000/callback",
				Issuer:       "https://auth.example.com",
			},
			validate: func(t *testing.T, client *Client) {
				if client == nil {
					t.Fatal("Client is nil")
				}
				if client.config.ClientSecret != "" {
					t.Errorf("Expected empty ClientSecret for public client, got '%s'", client.config.ClientSecret)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := NewClient(tt.oidcConfig)

			if tt.validate != nil {
				tt.validate(t, client)
			}
		})
	}
}

func TestClient_AuthCodeURL(t *testing.T) {
	t.Parallel()

	config := &models.OIDCConfig{
		ClientID:     "test-client-id",
		RedirectURI:  "http://localhost:3000/callback",
		Issuer:       "https://auth.example.com",
	}

	client := NewClient(config)
	state := "test-state-123"

	url := client.AuthCodeURL(state)

	if url == "" {
		t.Error("AuthCodeURL returned empty string")
	}

	if len(url) < len(state) {
		t.Error("AuthCodeURL should be longer than state")
	}

	// URL should contain the state
	if len(url) < 50 { // Basic sanity check
		t.Errorf("AuthCodeURL seems too short: %s", url)
	}
}

func stringPtr(s string) *string {
	return &s
}

// Note: ExchangeCode is hard to test without actual OAuth2 provider
// This would typically be tested in integration tests
func TestClient_ExchangeCode(t *testing.T) {
	t.Parallel()

	t.Skip("ExchangeCode requires actual OAuth2 provider - test in integration tests")
}
