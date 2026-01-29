package models

import "time"

// CorsConfig holds CORS configuration (allowed origins, etc.).
type CorsConfig struct {
	ConfigKey        string    `json:"config_key"`
	AllowedOrigins   string    `json:"allowed_origins"` // Comma-separated
	AllowCredentials bool      `json:"allow_credentials"`
	MaxAge           int       `json:"max_age"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
