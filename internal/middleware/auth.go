package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/benvon/smart-todo/internal/database"
	logpkg "github.com/benvon/smart-todo/internal/logger"
	"github.com/benvon/smart-todo/internal/models"
	"github.com/benvon/smart-todo/internal/request"
	"github.com/benvon/smart-todo/internal/services/oidc"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// MaxTokenSize is the maximum size for JWT tokens (8KB)
	MaxTokenSize = 8 * 1024 // 8KB
)

var (
	errAuthDatabaseFetch = errors.New("auth: database fetch error")
	errAuthCreateUser    = errors.New("auth: create user error")
)

func getOrCreateUser(ctx context.Context, userRepo *database.UserRepository, claims *models.JWTClaims, logger *zap.Logger) (*models.User, error) {
	user, err := userRepo.GetByProviderID(ctx, claims.Sub)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		logger.Error("database_error_fetching_user",
			zap.String("operation", "auth_fetch_user"),
			zap.String("error", logpkg.SanitizeError(err)),
			zap.String("provider_id", logpkg.SanitizeUserID(claims.Sub)),
		)
		return nil, fmt.Errorf("%w: %w", errAuthDatabaseFetch, err)
	}
	user = &models.User{
		ID:            uuid.New(),
		Email:         claims.Email,
		ProviderID:    &claims.Sub,
		Name:          &claims.Name,
		EmailVerified: true,
	}
	if err := userRepo.Create(ctx, user); err != nil {
		logger.Error("failed_to_create_user",
			zap.String("operation", "auth_create_user"),
			zap.String("error", logpkg.SanitizeError(err)),
			zap.String("provider_id", logpkg.SanitizeUserID(claims.Sub)),
			zap.String("email", logpkg.SanitizeString(claims.Email, logpkg.MaxGeneralStringLength)),
		)
		return nil, fmt.Errorf("%w: %w", errAuthCreateUser, err)
	}
	return user, nil
}

func maybeUpdateUser(ctx context.Context, userRepo *database.UserRepository, user *models.User, claims *models.JWTClaims, logger *zap.Logger) {
	updateNeeded := false
	if user.Email != claims.Email {
		user.Email = claims.Email
		updateNeeded = true
	}
	if (user.Name == nil && claims.Name != "") || (user.Name != nil && *user.Name != claims.Name) {
		n := claims.Name
		user.Name = &n
		updateNeeded = true
	}
	if !updateNeeded {
		return
	}
	if err := userRepo.Update(ctx, user); err != nil {
		logger.Warn("failed_to_update_user_info",
			zap.String("error", logpkg.SanitizeError(err)),
			zap.String("user_id", logpkg.SanitizeUserID(user.ID.String())),
		)
	}
}

// Auth creates authentication middleware that validates JWT tokens
func Auth(db *database.DB, oidcProvider *oidc.Provider, jwksManager *oidc.JWKSManager, providerName string, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				respondError(w, http.StatusUnauthorized, "Missing Authorization header", logger)
				return
			}
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				respondError(w, http.StatusUnauthorized, "Invalid Authorization header format", logger)
				return
			}
			tokenString := parts[1]
			if len(tokenString) > MaxTokenSize {
				logger.Warn("token_exceeds_max_size",
					zap.Int("token_size", len(tokenString)),
					zap.Int("max_size", MaxTokenSize),
				)
				respondError(w, http.StatusBadRequest, "Invalid token", logger)
				return
			}
			ctx := r.Context()
			oidcConfig, err := oidcProvider.GetConfig(ctx, providerName)
			if err != nil {
				logger.Error("failed_to_get_oidc_config",
					zap.String("operation", "auth_middleware"),
					zap.String("provider", logpkg.SanitizeString(providerName, logpkg.MaxGeneralStringLength)),
					zap.String("error", logpkg.SanitizeError(err)),
				)
				respondError(w, http.StatusInternalServerError, "Failed to get OIDC configuration", logger)
				return
			}
			if oidcConfig.JWKSUrl == nil {
				respondError(w, http.StatusInternalServerError, "JWKS URL not configured", logger)
				return
			}
			verifier := oidc.NewVerifier(jwksManager, oidcConfig.Issuer)
			claims, err := verifier.Verify(ctx, tokenString, *oidcConfig.JWKSUrl)
			if err != nil {
				logger.Warn("token_verification_failed",
					zap.String("ip", logpkg.SanitizeString(request.ClientIP(r), logpkg.MaxGeneralStringLength)),
					zap.String("issuer", logpkg.SanitizeString(oidcConfig.Issuer, logpkg.MaxGeneralStringLength)),
					zap.String("error", logpkg.SanitizeError(err)),
					zap.String("path", logpkg.SanitizePath(r.URL.Path)),
					zap.String("method", r.Method),
				)
				respondError(w, http.StatusUnauthorized, "Invalid or expired token", logger)
				return
			}
			userRepo := database.NewUserRepository(db)
			user, err := getOrCreateUser(ctx, userRepo, claims, logger)
			if err != nil {
				if errors.Is(err, errAuthDatabaseFetch) {
					respondError(w, http.StatusInternalServerError, "Database error", logger)
				} else {
					respondError(w, http.StatusInternalServerError, "Failed to create user", logger)
				}
				return
			}
			maybeUpdateUser(ctx, userRepo, user, claims, logger)
			ctx = request.WithUser(ctx, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func respondError(w http.ResponseWriter, status int, message string, logger *zap.Logger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := map[string]any{
		"success": false,
		"error":   message,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Use fallback logging to avoid recursion if logger fails
		// Write directly to response writer as last resort
		if _, writeErr := w.Write([]byte(`{"success":false,"error":"Failed to encode error response"}`)); writeErr != nil {
			// If even writing fails, there's nothing more we can do
			_ = writeErr
		}
		logger.Error("failed_to_encode_error_response",
			zap.String("operation", "auth_respond_error"),
			zap.String("error", logpkg.SanitizeError(err)),
			zap.Int("status_code", status),
		)
	}
}
