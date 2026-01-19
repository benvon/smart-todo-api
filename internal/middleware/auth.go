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

// getClientIP extracts the client IP from the request, respecting X-Forwarded-For
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies/load balancers)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs (comma-separated)
		// The first one is typically the original client IP
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header (alternative header used by some proxies)
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

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

const (
	// MaxTokenSize is the maximum size for JWT tokens (8KB)
	MaxTokenSize = 8 * 1024 // 8KB
)

// Auth creates authentication middleware that validates JWT tokens
func Auth(db *database.DB, oidcProvider *oidc.Provider, jwksManager *oidc.JWKSManager, providerName string) func(http.Handler) http.Handler {
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

			// Validate token length to prevent DoS attacks
			if len(tokenString) > MaxTokenSize {
				log.Printf("Token exceeds maximum size: %d bytes (max: %d)", len(tokenString), MaxTokenSize)
				respondError(w, http.StatusBadRequest, "Invalid token")
				return
			}

			// Get OIDC config using configured provider name
			ctx := r.Context()
			oidcConfig, err := oidcProvider.GetConfig(ctx, providerName)
			if err != nil {
				log.Printf("Failed to get OIDC config for provider '%s': %v", providerName, err)
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
				// Log detailed error server-side, but send generic message to client
				ip := getClientIP(r)
				log.Printf("[AUDIT] Token verification failed: ip=%s issuer=%s error=%v", ip, oidcConfig.Issuer, err)
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
						// Log error but continue - user can still use the app with stale data
						log.Printf("Failed to update user info: %v", err)
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
