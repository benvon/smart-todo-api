package models

import "time"

// RatelimitConfig holds rate limit configuration (e.g. "5-S", "100-M").
type RatelimitConfig struct {
	ConfigKey string    `json:"config_key"`
	Rate      string    `json:"rate"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
