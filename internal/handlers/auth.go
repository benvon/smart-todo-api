package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/benvon/smart-todo/internal/middleware"
	"github.com/benvon/smart-todo/internal/services/oidc"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	oidcProvider *oidc.Provider
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(oidcProvider *oidc.Provider) *AuthHandler {
	return &AuthHandler{oidcProvider: oidcProvider}
}

// RegisterRoutes registers auth routes on the given router
// The router should already have the /api/v1/auth prefix
func (h *AuthHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/oidc/login", h.GetOIDCLogin).Methods("GET")
	r.HandleFunc("/me", h.GetMe).Methods("GET")
}

// GetOIDCLogin returns OIDC configuration for frontend
func (h *AuthHandler) GetOIDCLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get login config for cognito provider
	loginConfig, err := h.oidcProvider.GetLoginConfig(ctx, "cognito")
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Failed to get OIDC configuration", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, loginConfig)
}

// GetMe returns current user information
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r)
	if user == nil {
		respondJSONError(w, http.StatusUnauthorized, "Unauthorized", "User not found in context")
		return
	}

	respondJSON(w, http.StatusOK, user)
}
