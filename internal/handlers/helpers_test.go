package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRespondJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   int
		data     any
		validate func(*testing.T, *http.Response)
	}{
		{
			name:   "simple object",
			status: http.StatusOK,
			data:   map[string]string{"message": "hello"},
			validate: func(t *testing.T, resp *http.Response) {
				if resp.StatusCode != http.StatusOK {
					t.Errorf("Expected status 200, got %d", resp.StatusCode)
				}

				contentType := resp.Header.Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
				}

				var body map[string]any
				if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if success, ok := body["success"].(bool); !ok || !success {
					t.Error("Expected success to be true")
				}

				if _, ok := body["timestamp"].(string); !ok {
					t.Error("Expected timestamp to be present")
				}

				if data, ok := body["data"].(map[string]any); !ok {
					t.Error("Expected data to be present")
				} else {
					if msg, ok := data["message"].(string); !ok || msg != "hello" {
						t.Errorf("Expected message 'hello', got %v", data["message"])
					}
				}
			},
		},
		{
			name:   "nil data",
			status: http.StatusCreated,
			data:   nil,
			validate: func(t *testing.T, resp *http.Response) {
				if resp.StatusCode != http.StatusCreated {
					t.Errorf("Expected status 201, got %d", resp.StatusCode)
				}

				var body map[string]any
				if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if body["data"] != nil {
					t.Error("Expected data to be nil")
				}
			},
		},
		{
			name:   "array data",
			status: http.StatusOK,
			data:   []string{"a", "b", "c"},
			validate: func(t *testing.T, resp *http.Response) {
				var body map[string]any
				if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if data, ok := body["data"].([]any); !ok {
					t.Error("Expected data to be an array")
				} else {
					if len(data) != 3 {
						t.Errorf("Expected array length 3, got %d", len(data))
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			respondJSON(w, tt.status, tt.data)

			resp := w.Result()
			defer resp.Body.Close()

			if tt.validate != nil {
				tt.validate(t, resp)
			}
		})
	}
}

func TestRespondJSONError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		status    int
		errorType string
		message   string
		validate  func(*testing.T, *http.Response)
	}{
		{
			name:      "bad request",
			status:    http.StatusBadRequest,
			errorType: "Bad Request",
			message:   "Invalid input",
			validate: func(t *testing.T, resp *http.Response) {
				if resp.StatusCode != http.StatusBadRequest {
					t.Errorf("Expected status 400, got %d", resp.StatusCode)
				}

				contentType := resp.Header.Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
				}

				var body map[string]any
				if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if success, ok := body["success"].(bool); !ok || success {
					t.Error("Expected success to be false")
				}

				if errorType, ok := body["error"].(string); !ok || errorType != "Bad Request" {
					t.Errorf("Expected error 'Bad Request', got '%v'", body["error"])
				}

				if msg, ok := body["message"].(string); !ok || msg != "Invalid input" {
					t.Errorf("Expected message 'Invalid input', got '%v'", body["message"])
				}

				if _, ok := body["timestamp"].(string); !ok {
					t.Error("Expected timestamp to be present")
				}
			},
		},
		{
			name:      "internal server error",
			status:    http.StatusInternalServerError,
			errorType: "Internal Server Error",
			message:   "Database connection failed",
			validate: func(t *testing.T, resp *http.Response) {
				if resp.StatusCode != http.StatusInternalServerError {
					t.Errorf("Expected status 500, got %d", resp.StatusCode)
				}

				var body map[string]any
				if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if errorType, ok := body["error"].(string); !ok || errorType != "Internal Server Error" {
					t.Errorf("Expected error 'Internal Server Error', got '%v'", body["error"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			respondJSONError(w, tt.status, tt.errorType, tt.message)

			resp := w.Result()
			defer resp.Body.Close()

			if tt.validate != nil {
				tt.validate(t, resp)
			}
		})
	}
}

// Test helper to verify timestamp is valid RFC3339
func TestRespondJSONTimestamp(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	respondJSON(w, http.StatusOK, "test")

	resp := w.Result()
	defer resp.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	timestamp, ok := body["timestamp"].(string)
	if !ok {
		t.Fatal("Timestamp not found in response")
	}

	if _, err := time.Parse(time.RFC3339, timestamp); err != nil {
		t.Errorf("Timestamp '%s' is not valid RFC3339: %v", timestamp, err)
	}
}

// Test helper to create a test request with body
func newTestRequest(method, path string, body any) *http.Request {
	var bodyReader *bytes.Reader
	if body != nil {
		bodyBytes, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(bodyBytes)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	return httptest.NewRequest(method, path, bodyReader)
}
