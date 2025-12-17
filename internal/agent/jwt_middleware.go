// Package agent provides JWT authentication middleware.
package agent

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const UserIDContextKey contextKey = "user_id"

// JWTMiddleware validates JWT tokens and extracts user ID
type JWTMiddleware struct {
	secretKey []byte
	logger    *zap.Logger
}

// NewJWTMiddleware creates a new JWT middleware
func NewJWTMiddleware(logger *zap.Logger) *JWTMiddleware {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-secret-change-in-production" // Default for development
	}
	return &JWTMiddleware{
		secretKey: []byte(secret),
		logger:    logger,
	}
}

// Middleware wraps an http.Handler with JWT validation
func (m *JWTMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			// Allow unauthenticated access with anonymous user
			ctx := context.WithValue(r.Context(), UserIDContextKey, "anonymous")
			next.ServeHTTP(w, r.WithContext(ctx))
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

		// Add user_id to context
		ctx := context.WithValue(r.Context(), UserIDContextKey, userID)
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

// GetUserIDFromRequest is a helper for WebSocket handlers
func GetUserIDFromRequest(r *http.Request) string {
	return GetUserID(r.Context())
}

// GenerateToken creates a new JWT token for a user
func GenerateToken(username string) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-secret-change-in-production"
	}

	// Create claims
	claims := jwt.MapClaims{
		"sub": username,
		"exp": jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		"iat": jwt.NewNumericDate(time.Now()),
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
