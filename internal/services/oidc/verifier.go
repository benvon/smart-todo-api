package oidc

import (
	"context"
	"fmt"

	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/benvon/smart-todo/internal/models"
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

// Verify verifies a JWT token and extracts claims
func (v *Verifier) Verify(ctx context.Context, tokenString string, jwksURL string) (*models.JWTClaims, error) {
	// Get JWKS
	keys, err := v.jwksManager.GetJWKS(ctx, jwksURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get JWKS: %w", err)
	}

	// Parse and verify token
	token, err := jwt.Parse([]byte(tokenString), jwt.WithKeySet(keys), jwt.WithValidate(true))
	if err != nil {
		return nil, fmt.Errorf("failed to parse/verify token: %w", err)
	}

	// Verify issuer
	iss, ok := token.Get("iss")
	if !ok {
		return nil, fmt.Errorf("token missing issuer claim")
	}
	if issStr, ok := iss.(string); !ok || issStr != v.issuer {
		return nil, fmt.Errorf("token issuer mismatch: expected %s, got %v", v.issuer, iss)
	}

	// Extract claims
	claims := &models.JWTClaims{}

	if sub, ok := token.Get("sub"); ok {
		if subStr, ok := sub.(string); ok {
			claims.Sub = subStr
		}
	}

	if email, ok := token.Get("email"); ok {
		if emailStr, ok := email.(string); ok {
			claims.Email = emailStr
		}
	}

	if name, ok := token.Get("name"); ok {
		if nameStr, ok := name.(string); ok {
			claims.Name = nameStr
		}
	}

	if exp, ok := token.Get("exp"); ok {
		if expFloat, ok := exp.(float64); ok {
			claims.Exp = int64(expFloat)
		}
	}

	if iat, ok := token.Get("iat"); ok {
		if iatFloat, ok := iat.(float64); ok {
			claims.Iat = int64(iatFloat)
		}
	}

	if iss, ok := token.Get("iss"); ok {
		if issStr, ok := iss.(string); ok {
			claims.Iss = issStr
		}
	}

	if aud, ok := token.Get("aud"); ok {
		if audStr, ok := aud.(string); ok {
			claims.Aud = audStr
		} else if audArr, ok := aud.([]any); ok && len(audArr) > 0 {
			if audStr, ok := audArr[0].(string); ok {
				claims.Aud = audStr
			}
		}
	}

	return claims, nil
}
