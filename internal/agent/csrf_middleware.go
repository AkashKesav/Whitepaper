// Package agent provides CSRF protection middleware.
package agent

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CSRFConfig holds configuration for CSRF protection
type CSRFConfig struct {
	Enabled          bool
	TokenLength      int
	TokenExpiration  time.Duration
	SafeMethods      []string
	RequireAuth      bool
	OriginValidation bool
	AllowedOrigins   []string // For origin validation
}

// DefaultCSRFConfig returns secure defaults for CSRF protection
func DefaultCSRFConfig() CSRFConfig {
	return CSRFConfig{
		Enabled:          true,
		TokenLength:      32,
		TokenExpiration:  24 * time.Hour,
		SafeMethods:      []string{"GET", "HEAD", "OPTIONS", "TRACE"},
		RequireAuth:      true,
		OriginValidation: true,
		AllowedOrigins:   []string{}, // Empty means use server's allowed origins
	}
}

// CSRFMiddleware provides CSRF protection for HTTP endpoints
type CSRFMiddleware struct {
	config     CSRFConfig
	logger     *zap.Logger
	secretKey  []byte
	tokenCache map[string]time.Time
	cacheMu    sync.RWMutex
}

// NewCSRFMiddleware creates a new CSRF middleware
func NewCSRFMiddleware(config CSRFConfig, logger *zap.Logger, secretKey []byte) *CSRFMiddleware {
	if len(config.SafeMethods) == 0 {
		config.SafeMethods = []string{"GET", "HEAD", "OPTIONS", "TRACE"}
	}

	return &CSRFMiddleware{
		config:     config,
		logger:     logger,
		secretKey:  secretKey,
		tokenCache: make(map[string]time.Time),
	}
}

// isSafeMethod checks if the HTTP method is safe (doesn't modify state)
func (m *CSRFMiddleware) isSafeMethod(method string) bool {
	for _, safe := range m.config.SafeMethods {
		if method == safe {
			return true
		}
	}
	return false
}

// isValidOrigin validates the Origin or Referer header against allowed origins
func (m *CSRFMiddleware) isValidOrigin(r *http.Request, origin, referer string) bool {
	// If origin is present, validate it
	if origin != "" {
		// Same-origin request (no origin header, or matching)
		if origin == r.Host {
			return true
		}

		// Check against allowed origins
		for _, allowed := range m.config.AllowedOrigins {
			if allowed == "*" {
				return true
			}
			if originMatches(origin, allowed) {
				return true
			}
		}

		m.logger.Warn("CSRF: Invalid origin",
			zap.String("origin", origin),
			zap.String("host", r.Host))
		return false
	}

	// Fallback to referer validation if origin is not set
	if referer != "" {
		// Extract origin from referer
		refererOrigin := referer
		if idx := strings.Index(referer, "/"); idx > 8 { // After "https://"
			refererOrigin = referer[:idx]
		}

		for _, allowed := range m.config.AllowedOrigins {
			if allowed == "*" {
				return true
			}
			if originMatches(refererOrigin, allowed) {
				return true
			}
		}

		m.logger.Warn("CSRF: Invalid referer",
			zap.String("referer", referer))
		return false
	}

	// No origin or referer - likely same-origin request
	return true
}

// generateToken creates a new CSRF token
func (m *CSRFMiddleware) generateToken() (string, error) {
	bytes := make([]byte, m.config.TokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// validateToken checks if a token is valid
func (m *CSRFMiddleware) validateToken(token string) bool {
	if token == "" {
		return false
	}

	m.cacheMu.RLock()
	expiry, exists := m.tokenCache[token]
	m.cacheMu.RUnlock()

	if !exists {
		return false
	}

	if time.Now().After(expiry) {
		// Token expired
		m.cacheMu.Lock()
		delete(m.tokenCache, token)
		m.cacheMu.Unlock()
		return false
	}

	return true
}

// storeToken stores a token with expiration
func (m *CSRFMiddleware) storeToken(token string) {
	expiry := time.Now().Add(m.config.TokenExpiration)
	m.cacheMu.Lock()
	m.tokenCache[token] = expiry
	m.cacheMu.Unlock()

	// Clean up expired tokens periodically
	go func() {
		time.Sleep(m.config.TokenExpiration)
		m.cacheMu.Lock()
		defer m.cacheMu.Unlock()
		for t, exp := range m.tokenCache {
			if time.Now().After(exp) {
				delete(m.tokenCache, t)
			}
		}
	}()
}

// Middleware returns the CSRF middleware handler
func (m *CSRFMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Skip safe methods - just generate and return token in header
		if m.isSafeMethod(r.Method) {
			token, _ := m.generateToken()
			m.storeToken(token)
			w.Header().Set("X-CSRF-Token", token)
			next.ServeHTTP(w, r)
			return
		}

		// Validate Origin/Referer headers for state-changing requests
		if m.config.OriginValidation {
			origin := r.Header.Get("Origin")
			referer := r.Header.Get("Referer")

			// For state-changing methods, require origin or referer validation
			if origin == "" && referer == "" {
				// Allow same-origin requests (browsers don't send Origin for same-origin POST)
				// But we should still require CSRF token
			} else if !m.isValidOrigin(r, origin, referer) {
				http.Error(w, "Invalid origin", http.StatusForbidden)
				return
			}
		}

		// Check CSRF token from header or form
		token := r.Header.Get("X-CSRF-Token")
		if token == "" {
			token = r.FormValue("csrf_token")
		}

		if token == "" || !m.validateToken(token) {
			m.logger.Warn("CSRF: Invalid or missing token",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path))
			http.Error(w, "Invalid CSRF token", http.StatusForbidden)
			return
		}

		// Token valid - rotate it (one-time use)
		newToken, _ := m.generateToken()
		m.storeToken(newToken)
		w.Header().Set("X-CSRF-Token", newToken)

		next.ServeHTTP(w, r)
	})
}

// GenerateTokenForUser generates a CSRF token for a specific user context
// This can be called to include tokens in responses
func (m *CSRFMiddleware) GenerateTokenForUser() (string, error) {
	token, err := m.generateToken()
	if err != nil {
		return "", err
	}
	m.storeToken(token)
	return token, nil
}

// CleanupExpiredTokens removes expired tokens from the cache
func (m *CSRFMiddleware) CleanupExpiredTokens() {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	now := time.Now()
	for token, expiry := range m.tokenCache {
		if now.After(expiry) {
			delete(m.tokenCache, token)
		}
	}
}

// GetTokenCount returns the number of active tokens (for monitoring)
func (m *CSRFMiddleware) GetTokenCount() int {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()
	return len(m.tokenCache)
}

var _ = fmt.Sprintf // ensure fmt is used
