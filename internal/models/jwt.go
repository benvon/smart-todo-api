package models

// JWTClaims represents the claims extracted from a JWT token
type JWTClaims struct {
	Sub   string `json:"sub"`   // Subject (user ID from provider)
	Email string `json:"email"` // User email
	Name  string `json:"name"`  // User name
	Exp   int64  `json:"exp"`   // Expiration time
	Iat   int64  `json:"iat"`   // Issued at
	Iss   string `json:"iss"`   // Issuer
	Aud   string `json:"aud"`   // Audience
}
