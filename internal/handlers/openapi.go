package handlers

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"
)

// OpenAPIHandler handles OpenAPI specification requests
type OpenAPIHandler struct {
	openAPIPath string
}

// NewOpenAPIHandler creates a new OpenAPI handler
func NewOpenAPIHandler(openAPIPath string) *OpenAPIHandler {
	return &OpenAPIHandler{openAPIPath: openAPIPath}
}

// RegisterRoutes registers OpenAPI routes
func (h *OpenAPIHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/v1/openapi.yaml", h.ServeYAML).Methods("GET")
	r.HandleFunc("/api/v1/openapi.json", h.ServeJSON).Methods("GET")
}

// ServeYAML serves the OpenAPI spec in YAML format
func (h *OpenAPIHandler) ServeYAML(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(h.openAPIPath)
	if err != nil {
		http.Error(w, "OpenAPI specification not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/x-yaml")
	w.Write(data)
}

// ServeJSON serves the OpenAPI spec in JSON format
func (h *OpenAPIHandler) ServeJSON(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(h.openAPIPath)
	if err != nil {
		http.Error(w, "OpenAPI specification not found", http.StatusNotFound)
		return
	}

	// Parse YAML into a map
	var yamlData map[string]any
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		http.Error(w, "Failed to parse OpenAPI specification", http.StatusInternalServerError)
		return
	}

	// Convert to JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(yamlData); err != nil {
		http.Error(w, "Failed to encode JSON response", http.StatusInternalServerError)
		return
	}
}
