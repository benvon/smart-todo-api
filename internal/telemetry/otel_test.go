package telemetry

import (
	"context"
	"testing"
	"time"
)

func TestInitTracer(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		endpoint    string
		wantErr     bool
	}{
		{
			name:        "valid configuration",
			serviceName: "test-service",
			endpoint:    "localhost:4318",
			wantErr:     false,
		},
		{
			name:        "empty service name",
			serviceName: "",
			endpoint:    "localhost:4318",
			wantErr:     false, // Should still succeed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			tp, err := InitTracer(ctx, tt.serviceName, tt.endpoint)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitTracer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tp != nil {
				// Clean up
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer shutdownCancel()
				if err := Shutdown(shutdownCtx, tp); err != nil {
					t.Errorf("Shutdown() error = %v", err)
				}
			}
		})
	}
}

func TestShutdown(t *testing.T) {
	t.Run("shutdown with nil provider", func(t *testing.T) {
		ctx := context.Background()
		err := Shutdown(ctx, nil)
		if err != nil {
			t.Errorf("Shutdown() with nil provider should not error, got: %v", err)
		}
	})

	t.Run("shutdown with valid provider", func(t *testing.T) {
		ctx := context.Background()
		tp, err := InitTracer(ctx, "test-service", "localhost:4318")
		if err != nil {
			t.Fatalf("Failed to initialize tracer: %v", err)
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = Shutdown(shutdownCtx, tp)
		if err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})
}
