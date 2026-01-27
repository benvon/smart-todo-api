package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"
)

// OpenAPIHandler handles OpenAPI specification requests
type OpenAPIHandler struct {
	openAPIPath string
	baseDir     string
}

// NewOpenAPIHandler creates a new OpenAPI handler with path validation
func NewOpenAPIHandler(openAPIPath string) *OpenAPIHandler {
	// Resolve absolute paths to prevent directory traversal
	absPath, _ := filepath.Abs(openAPIPath)
	baseDir, _ := filepath.Abs(filepath.Dir(openAPIPath))

	return &OpenAPIHandler{
		openAPIPath: absPath,
		baseDir:     baseDir,
	}
}

// RegisterRoutes registers OpenAPI routes
func (h *OpenAPIHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/v1/openapi.yaml", h.ServeYAML).Methods("GET")
	r.HandleFunc("/api/v1/openapi.json", h.ServeJSON).Methods("GET")
}

// validatePath ensures the file path is within the allowed directory
func (h *OpenAPIHandler) validatePath() error {
	// Clean and resolve the path
	cleanPath := filepath.Clean(h.openAPIPath)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return err
	}

	// Ensure the resolved path is within the base directory
	relPath, err := filepath.Rel(h.baseDir, absPath)
	if err != nil {
		return err
	}

	// Check for directory traversal attempts (paths starting with "..")
	if filepath.IsAbs(relPath) || relPath == ".." || len(relPath) > 2 && relPath[:3] == "../" {
		return os.ErrPermission
	}

	return nil
}

// ServeYAML serves the OpenAPI spec in YAML format
func (h *OpenAPIHandler) ServeYAML(w http.ResponseWriter, r *http.Request) {
	// Validate path to prevent directory traversal
	if err := h.validatePath(); err != nil {
		http.Error(w, "OpenAPI specification not found", http.StatusNotFound)
		return
	}

	data, err := os.ReadFile(h.openAPIPath)
	if err != nil {
		http.Error(w, "OpenAPI specification not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/x-yaml")
	if _, err := w.Write(data); err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
		return
	}
}

// ServeJSON serves the OpenAPI spec in JSON format
func (h *OpenAPIHandler) ServeJSON(w http.ResponseWriter, r *http.Request) {
	// Validate path to prevent directory traversal
	if err := h.validatePath(); err != nil {
		http.Error(w, "OpenAPI specification not found", http.StatusNotFound)
		return
	}

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
