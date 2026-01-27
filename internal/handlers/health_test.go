package handlers

import (
	"encoding/json"
	"testing"
	"time"
)

func TestHealthChecker_BasicMode(t *testing.T) {
	t.Parallel()

	// This test requires a real database connection
	// For now, we'll skip it and focus on testing the response structure
	// In a real test environment, you'd use testcontainers or a test database
	t.Skip("Requires database connection - implement with testcontainers or integration test setup")
	
	// Test structure:
	// 1. Create health checker with real DB
	// 2. Call HealthCheck with basic mode
	// 3. Verify response structure and status
}

func TestHealthChecker_ExtendedMode_ResponseStructure(t *testing.T) {
	t.Parallel()

	// Test the response structure and logic without requiring real connections
	// This validates that the health check logic correctly structures responses
	
	tests := []struct {
		name           string
		mode           string
		expectChecks   bool
		expectStatusOK bool
	}{
		{
			name:           "basic mode - no checks",
			mode:           "",
			expectChecks:   false,
			expectStatusOK: true,
		},
		{
			name:           "extended mode - has checks",
			mode:           "extended",
			expectChecks:   true,
			expectStatusOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			
			// This test requires real database/Redis/RabbitMQ connections
			// For unit testing, we validate the response structure separately
			// Integration tests would use testcontainers
			t.Skip("Requires database connection - implement with testcontainers or integration test setup")
		})
	}
}

func TestHealthResponse_Structure(t *testing.T) {
	t.Parallel()

	// Test that HealthResponse can be marshaled/unmarshaled correctly
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks: map[string]string{
			"database": "healthy",
			"redis":    "healthy",
			"rabbitmq": "healthy",
		},
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	var unmarshaled HealthResponse
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if unmarshaled.Status != response.Status {
		t.Errorf("Expected status %s, got %s", response.Status, unmarshaled.Status)
	}

	if len(unmarshaled.Checks) != len(response.Checks) {
		t.Errorf("Expected %d checks, got %d", len(response.Checks), len(unmarshaled.Checks))
	}

	for key, value := range response.Checks {
		if unmarshaled.Checks[key] != value {
			t.Errorf("Expected check[%s] = %s, got %s", key, value, unmarshaled.Checks[key])
		}
	}
}

func TestHealthResponse_UnhealthyStatus(t *testing.T) {
	t.Parallel()

	// Test unhealthy response structure
	response := HealthResponse{
		Status:    "unhealthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks: map[string]string{
			"database": "unhealthy",
			"redis":    "healthy",
			"rabbitmq": "healthy",
		},
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	var unmarshaled HealthResponse
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if unmarshaled.Status != "unhealthy" {
		t.Errorf("Expected status 'unhealthy', got %s", unmarshaled.Status)
	}

	if unmarshaled.Checks["database"] != "unhealthy" {
		t.Errorf("Expected database check to be 'unhealthy', got %s", unmarshaled.Checks["database"])
	}
}

func TestHealthResponse_NotConfigured(t *testing.T) {
	t.Parallel()

	// Test response when Redis/RabbitMQ are not configured
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks: map[string]string{
			"database": "healthy",
			"redis":    "not configured",
			"rabbitmq": "not configured",
		},
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	var unmarshaled HealthResponse
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if unmarshaled.Checks["redis"] != "not configured" {
		t.Errorf("Expected redis check to be 'not configured', got %s", unmarshaled.Checks["redis"])
	}

	if unmarshaled.Checks["rabbitmq"] != "not configured" {
		t.Errorf("Expected rabbitmq check to be 'not configured', got %s", unmarshaled.Checks["rabbitmq"])
	}
}
