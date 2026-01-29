package oidc

import (
	"context"
	"fmt"

	"github.com/benvon/smart-todo/internal/models"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// Verifier verifies JWT tokens
type Verifier struct {
	jwksManager *JWKSManager
	issuer      string
}

// NewVerifier creates a new JWT verifier
func NewVerifier(jwksManager *JWKSManager, issuer string) *Verifier {
	return &Verifier{
		jwksManager: jwksManager,
		issuer:      issuer,
	}
}

const (
	// MaxTokenSize is the maximum size for JWT tokens (8KB)
	MaxTokenSize = 8 * 1024 // 8KB
)

// Verify verifies a JWT token and extracts claims
func (v *Verifier) Verify(ctx context.Context, tokenString string, jwksURL string) (*models.JWTClaims, error) {
	if len(tokenString) > MaxTokenSize {
		return nil, fmt.Errorf("token exceeds maximum size of %d bytes", MaxTokenSize)
	}
	keys, err := v.jwksManager.GetJWKS(ctx, jwksURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get JWKS: %w", err)
	}
	token, err := jwt.Parse([]byte(tokenString), jwt.WithKeySet(keys), jwt.WithValidate(true))
	if err != nil {
		return nil, fmt.Errorf("failed to parse/verify token: %w", err)
	}
	if err := jwt.Validate(token, jwt.WithIssuer(v.issuer)); err != nil {
		return nil, fmt.Errorf("token issuer validation failed: %w", err)
	}
	return extractJWTClaims(token), nil
}

// extractJWTClaims copies standard claims from the token into a JWTClaims struct.
func extractJWTClaims(token jwt.Token) *models.JWTClaims {
	claims := &models.JWTClaims{}
	setStringClaim(token, "sub", &claims.Sub)
	setStringClaim(token, "email", &claims.Email)
	setStringClaim(token, "name", &claims.Name)
	setStringClaim(token, "iss", &claims.Iss)
	setInt64Claim(token, "exp", &claims.Exp)
	setInt64Claim(token, "iat", &claims.Iat)
	setAudClaim(token, &claims.Aud)
	return claims
}

func setStringClaim(token jwt.Token, key string, out *string) {
	if v, ok := token.Get(key); ok {
		if s, ok := v.(string); ok {
			*out = s
		}
	}
}

func setInt64Claim(token jwt.Token, key string, out *int64) {
	if v, ok := token.Get(key); ok {
		if f, ok := v.(float64); ok {
			*out = int64(f)
		}
	}
}

func setAudClaim(token jwt.Token, out *string) {
	if v, ok := token.Get("aud"); ok {
		if s, ok := v.(string); ok {
			*out = s
			return
		}
		if arr, ok := v.([]any); ok && len(arr) > 0 {
			if s, ok := arr[0].(string); ok {
				*out = s
			}
		}
	}
}
