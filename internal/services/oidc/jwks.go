package oidc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
)

// JWKSCache caches JWKS keys
type JWKSCache struct {
	keys    jwk.Set
	expires time.Time
	mu      sync.RWMutex
}

// JWKSManager manages JWKS fetching and caching
type JWKSManager struct {
	cache map[string]*JWKSCache
	mu    sync.RWMutex
	ttl   time.Duration
}

// NewJWKSManager creates a new JWKS manager
func NewJWKSManager() *JWKSManager {
	return &JWKSManager{
		cache: make(map[string]*JWKSCache),
		ttl:   1 * time.Hour, // Cache for 1 hour
	}
}

// GetJWKS retrieves JWKS for a given JWKS URL, with caching
func (m *JWKSManager) GetJWKS(ctx context.Context, jwksURL string) (jwk.Set, error) {
	m.mu.RLock()
	cache, exists := m.cache[jwksURL]
	m.mu.RUnlock()

	if exists {
		cache.mu.RLock()
		if time.Now().Before(cache.expires) && cache.keys != nil {
			keys := cache.keys
			cache.mu.RUnlock()
			return keys, nil
		}
		cache.mu.RUnlock()
	}

	// Fetch fresh JWKS
	keys, err := m.fetchJWKS(ctx, jwksURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}

	// Update cache
	m.mu.Lock()
	m.cache[jwksURL] = &JWKSCache{
		keys:    keys,
		expires: time.Now().Add(m.ttl),
	}
	m.mu.Unlock()

	return keys, nil
}

func (m *JWKSManager) fetchJWKS(ctx context.Context, jwksURL string) (jwk.Set, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read JWKS response: %w", err)
	}

	keys, err := jwk.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWKS: %w", err)
	}

	return keys, nil
}
