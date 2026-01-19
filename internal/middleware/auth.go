package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/benvon/smart-todo/internal/database"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/services/oidc"
)

type contextKey string

const userContextKey contextKey = "user"

// UserFromContext extracts the user from the request context
func UserFromContext(r *http.Request) *models.User {
	user, ok := r.Context().Value(userContextKey).(*models.User)
	if !ok {
		return nil
	}
	return user
}

// Auth creates authentication middleware that validates JWT tokens
func Auth(db *database.DB, oidcProvider *oidc.Provider, jwksManager *oidc.JWKSManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				respondError(w, http.StatusUnauthorized, "Missing Authorization header")
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				respondError(w, http.StatusUnauthorized, "Invalid Authorization header format")
				return
			}

			tokenString := parts[1]

			// Get OIDC config (assuming cognito for now)
			ctx := r.Context()
			oidcConfig, err := oidcProvider.GetConfig(ctx, "cognito")
			if err != nil {
				respondError(w, http.StatusInternalServerError, "Failed to get OIDC configuration")
				return
			}

			if oidcConfig.JWKSUrl == nil {
				respondError(w, http.StatusInternalServerError, "JWKS URL not configured")
				return
			}

			// Verify token
			verifier := oidc.NewVerifier(jwksManager, oidcConfig.Issuer)
			claims, err := verifier.Verify(ctx, tokenString, *oidcConfig.JWKSUrl)
			if err != nil {
				log.Printf("Token verification failed: %v (issuer: %s, jwks_url: %s)", err, oidcConfig.Issuer, *oidcConfig.JWKSUrl)
				respondError(w, http.StatusUnauthorized, "Invalid or expired token")
				return
			}

			// Get or create user
			userRepo := database.NewUserRepository(db)
			user, err := userRepo.GetByProviderID(ctx, claims.Sub)
			if err != nil {
				// Check if this is a "not found" error vs an actual database error
				// The repository wraps sql.ErrNoRows, so errors.Is will unwrap and check
				if errors.Is(err, sql.ErrNoRows) {
					// User doesn't exist, create it
					user = &models.User{
						ID:            uuid.New(),
						Email:         claims.Email,
						ProviderID:    &claims.Sub,
						Name:          &claims.Name,
						EmailVerified: true,
					}
					if err := userRepo.Create(ctx, user); err != nil {
						respondError(w, http.StatusInternalServerError, "Failed to create user")
						return
					}
				} else {
					// Actual database error (connection failure, timeout, etc.)
					log.Printf("Database error while fetching user: %v", err)
					respondError(w, http.StatusInternalServerError, "Database error")
					return
				}
			} else {
				// Update user info if needed
				updateNeeded := false
				if user.Email != claims.Email {
					user.Email = claims.Email
					updateNeeded = true
				}
				if (user.Name == nil && claims.Name != "") || (user.Name != nil && *user.Name != claims.Name) {
					name := claims.Name
					user.Name = &name
					updateNeeded = true
				}
				if updateNeeded {
					if err := userRepo.Update(ctx, user); err != nil {
						// Log error but continue
					}
				}
			}

			// Add user to context
			ctx = context.WithValue(ctx, userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := map[string]any{
		"success": false,
		"error":   message,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode error response: %v", err)
	}
}
