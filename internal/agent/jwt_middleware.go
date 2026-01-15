// Package agent provides JWT authentication middleware.
package agent

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const UserIDContextKey contextKey = "user_id"
const UserRoleContextKey contextKey = "user_role"

// TokenType represents the type of JWT token
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// TokenConfig holds token duration configuration
type TokenConfig struct {
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
	Issuer               string
}

// DefaultTokenConfig returns secure default token durations
func DefaultTokenConfig() TokenConfig {
	// Support environment variable overrides
	accessDur := 15 * time.Minute // Default: 15 minutes for access tokens
	if dur := os.Getenv("JWT_ACCESS_DURATION"); dur != "" {
		if parsed, err := time.ParseDuration(dur); err == nil {
			accessDur = parsed
		}
	}

	refreshDur := 7 * 24 * time.Hour // Default: 7 days for refresh tokens
	if dur := os.Getenv("JWT_REFRESH_DURATION"); dur != "" {
		if parsed, err := time.ParseDuration(dur); err == nil {
			refreshDur = parsed
		}
	}

	return TokenConfig{
		AccessTokenDuration:  accessDur,
		RefreshTokenDuration: refreshDur,
		Issuer:               "reflective-memory-kernel",
	}
}

// TokenPair represents both access and refresh tokens
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// JWTMiddleware validates JWT tokens and extracts user ID
type JWTMiddleware struct {
	secretKey []byte
	logger    *zap.Logger
}

// NewJWTMiddleware creates a new JWT middleware
// DEVELOPMENT: Uses default secret if JWT_SECRET is not set (logs warning)
// SECURITY: For production, always set JWT_SECRET to a secure 32+ character value
func NewJWTMiddleware(logger *zap.Logger) (*JWTMiddleware, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// Fallback for development - Crypto uses same default
		secret = "default-dev-secret-change-in-production-32chars"
		logger.Warn("Using default JWT secret - set JWT_SECRET in production for security")
	}
	if len(secret) < 32 {
		// Pad short secrets to meet minimum length (development only)
		secret = secret + "xxxxxxxxxxxxxxxxxxxxxxxxxxxx"
		logger.Warn("JWT_SECRET too short, using padded value - set proper JWT_SECRET in production")
	}
	return &JWTMiddleware{
		secretKey: []byte(secret),
		logger:    logger,
	}, nil
}

// Middleware wraps an http.Handler with JWT validation
// SECURITY: Public paths are explicitly defined; all other paths require authentication
func (m *JWTMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Define public paths that don't require authentication
		publicPaths := map[string]bool{
			"/api/login":      true,
			"/api/register":   true,
			"/health":         true,
			"/api/health":     true,
		}

		path := r.URL.Path

		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			// Only allow anonymous access to public paths
			if publicPaths[path] {
				ctx := context.WithValue(r.Context(), UserIDContextKey, "anonymous")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			// All other paths require authentication
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Parse and validate token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Validate signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return m.secretKey, nil
		})

		if err != nil || !token.Valid {
			m.logger.Warn("Invalid JWT token", zap.Error(err))
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Extract user_id from claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "Invalid token claims", http.StatusUnauthorized)
			return
		}

		// Try standard "sub" claim first, then fallback to "user_id"
		userID, ok := claims["sub"].(string)
		if !ok || userID == "" {
			userID, _ = claims["user_id"].(string)
		}
		if userID == "" {
			http.Error(w, "Token missing user identifier", http.StatusUnauthorized)
			return
		}

		m.logger.Debug("Authenticated user", zap.String("user_id", userID))

		// Extract role from claims (default to "user" if not present)
		role, _ := claims["role"].(string)
		if role == "" {
			role = "user"
		}

		// Add user_id and role to context
		ctx := context.WithValue(r.Context(), UserIDContextKey, userID)
		ctx = context.WithValue(ctx, UserRoleContextKey, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserID extracts user ID from request context
func GetUserID(ctx context.Context) string {
	if id, ok := ctx.Value(UserIDContextKey).(string); ok {
		return id
	}
	return "anonymous"
}

// GetUserRole extracts user role from request context
func GetUserRole(ctx context.Context) string {
	if role, ok := ctx.Value(UserRoleContextKey).(string); ok {
		return role
	}
	return "user"
}

// GetUserIDFromRequest is a helper for WebSocket handlers
func GetUserIDFromRequest(r *http.Request) string {
	return GetUserID(r.Context())
}

// getJWTSecret returns the JWT secret from environment, with fallback for development
func getJWTSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "default-dev-secret-change-in-production-32chars"
	}
	if len(secret) < 32 {
		return secret + "xxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	}
	return secret
}

// GenerateToken creates a new JWT token for a user
// DEVELOPMENT: Uses default secret if JWT_SECRET is not set
// SECURITY: For production, always set JWT_SECRET to a secure 32+ character value
func GenerateToken(username, role string) (string, error) {
	secret := getJWTSecret()

	// Create claims
	claims := jwt.MapClaims{
		"sub":  username,
		"role": role,
		"exp":  jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		"iat":  jwt.NewNumericDate(time.Now()),
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign and get the complete encoded token as a string
	return token.SignedString([]byte(secret))
}

// HashPassword hashes a plain text password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword compares a hashed password with a plain text password
func CheckPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// GenerateTokenPair creates both access and refresh tokens for a user
// DEVELOPMENT: Uses default secret if JWT_SECRET is not set
// SECURITY: Uses short-lived access tokens (15 min) and longer-lived refresh tokens (7 days)
func GenerateTokenPair(username, role string) (*TokenPair, error) {
	secret := getJWTSecret()

	config := DefaultTokenConfig()
	now := time.Now()

	// Access Token - short-lived for security
	accessClaims := jwt.MapClaims{
		"type": TokenTypeAccess,
		"sub":  username,
		"role":  role,
		"exp":  jwt.NewNumericDate(now.Add(config.AccessTokenDuration)),
		"iat":  jwt.NewNumericDate(now),
		"iss":  config.Issuer,
		"jti":  uuid.New().String(),
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessSigned, err := accessToken.SignedString([]byte(secret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// Refresh Token - longer-lived for getting new access tokens
	refreshClaims := jwt.MapClaims{
		"type": TokenTypeRefresh,
		"sub":  username,
		"role":  role,
		"exp":  jwt.NewNumericDate(now.Add(config.RefreshTokenDuration)),
		"iat":  jwt.NewNumericDate(now),
		"iss":  config.Issuer,
		"jti":  uuid.New().String(),
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshSigned, err := refreshToken.SignedString([]byte(secret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessSigned,
		RefreshToken: refreshSigned,
		ExpiresAt:    now.Add(config.AccessTokenDuration),
		TokenType:    "Bearer",
	}, nil
}

// RefreshAccessToken validates a refresh token and issues a new token pair
// DEVELOPMENT: Uses default secret if JWT_SECRET is not set
// SECURITY: Only accepts refresh tokens, not access tokens
func RefreshAccessToken(refreshToken string) (*TokenPair, error) {
	secret := getJWTSecret()

	// Parse and validate refresh token
	token, err := jwt.Parse(refreshToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid or expired refresh token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Verify this is a refresh token
	tokenType, _ := claims["type"].(string)
	if tokenType != string(TokenTypeRefresh) {
		return nil, fmt.Errorf("expected refresh token, got %s", tokenType)
	}

	// Extract user info
	username, _ := claims["sub"].(string)
	if username == "" {
		return nil, fmt.Errorf("refresh token missing subject")
	}

	role, _ := claims["role"].(string)
	if role == "" {
		role = "user"
	}

	// Issue new token pair
	return GenerateTokenPair(username, role)
}
