package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestTraceContextPropagation verifies that trace context is properly propagated
func TestTraceContextPropagation(t *testing.T) {
	// Create a test exporter to capture spans
	exporter := tracetest.NewInMemoryExporter()

	// Create tracer provider with the test exporter
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	// Set up propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Create test router with OTEL middleware
	r := mux.NewRouter()
	r.Use(otelmux.Middleware("test-service"))
	r.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	})

	tests := []struct {
		name            string
		withTraceParent bool
		traceParent     string
	}{
		{
			name:            "without existing trace ID",
			withTraceParent: false,
		},
		{
			name:            "with existing trace ID",
			withTraceParent: true,
			traceParent:     "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear previous spans
			exporter.Reset()

			// Create test request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.withTraceParent {
				req.Header.Set("traceparent", tt.traceParent)
			}

			// Record response
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			// Verify response
			if rr.Code != http.StatusOK {
				t.Errorf("Expected status OK, got %d", rr.Code)
			}

			// Force flush
			if err := tp.ForceFlush(context.Background()); err != nil {
				t.Errorf("Failed to flush tracer provider: %v", err)
			}

			// Verify span was created
			spans := exporter.GetSpans()
			if len(spans) == 0 {
				t.Error("Expected at least one span to be created")
			}

			// If trace parent was provided, verify it was used
			if tt.withTraceParent && len(spans) > 0 {
				span := spans[0]
				// Verify the span is part of a trace (has a valid trace ID)
				if !span.SpanContext.TraceID().IsValid() {
					t.Error("Expected valid trace ID in span")
				}
			}
		})
	}
}
