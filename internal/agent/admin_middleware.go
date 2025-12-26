// Package agent provides admin middleware for protecting admin-only routes.
package agent

import (
	"net/http"

	"go.uber.org/zap"
)

// AdminMiddleware ensures only users with admin role can access protected routes
type AdminMiddleware struct {
	logger *zap.Logger
}

// NewAdminMiddleware creates a new admin middleware
func NewAdminMiddleware(logger *zap.Logger) *AdminMiddleware {
	return &AdminMiddleware{
		logger: logger,
	}
}

// Middleware checks if the user has admin role
func (m *AdminMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get user role from context (set by JWT middleware)
		role := GetUserRole(r.Context())
		userID := GetUserID(r.Context())

		if role != "admin" {
			m.logger.Warn("Admin access denied",
				zap.String("user_id", userID),
				zap.String("role", role),
				zap.String("path", r.URL.Path))
			http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
			return
		}

		m.logger.Debug("Admin access granted",
			zap.String("user_id", userID),
			zap.String("path", r.URL.Path))

		next.ServeHTTP(w, r)
	})
}
