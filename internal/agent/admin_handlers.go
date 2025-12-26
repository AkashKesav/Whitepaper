// Package agent provides admin API handlers for user and system management.
package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// AdminUser represents a user in the admin user list
type AdminUser struct {
	Username  string `json:"username"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at,omitempty"`
}

// AdminUserListResponse is the response for listing users
type AdminUserListResponse struct {
	Users []AdminUser `json:"users"`
	Total int         `json:"total"`
}

// UpdateRoleRequest is the request to update a user's role
type UpdateRoleRequest struct {
	Role string `json:"role"` // "admin" or "user"
}

// AdminSystemStats represents detailed system statistics
type AdminSystemStats struct {
	Uptime            string                 `json:"uptime"`
	TotalUsers        int                    `json:"total_users"`
	TotalAdmins       int                    `json:"total_admins"`
	ActiveConnections int                    `json:"active_connections"`
	KernelStats       map[string]interface{} `json:"kernel_stats,omitempty"`
	CacheStats        map[string]interface{} `json:"cache_stats,omitempty"`
	Timestamp         string                 `json:"timestamp"`
}

// ActivityLogEntry represents a single activity log entry
type ActivityLogEntry struct {
	Timestamp string `json:"timestamp"`
	UserID    string `json:"user_id"`
	Action    string `json:"action"`
	Details   string `json:"details,omitempty"`
}

// SetupAdminRoutes configures admin-only routes
func (s *Server) SetupAdminRoutes(r *mux.Router) {
	// Create admin middleware
	adminMiddleware := NewAdminMiddleware(s.logger)

	// Admin routes (under /api/admin, requires JWT + admin role)
	adminRouter := r.PathPrefix("/api/admin").Subrouter()
	adminRouter.Use(NewJWTMiddleware(s.logger).Middleware)
	adminRouter.Use(adminMiddleware.Middleware)

	// User management
	adminRouter.HandleFunc("/users", s.handleAdminListUsers).Methods("GET", "OPTIONS")
	adminRouter.HandleFunc("/users/{username}", s.handleAdminGetUser).Methods("GET", "OPTIONS")
	adminRouter.HandleFunc("/users/{username}/role", s.handleAdminUpdateUserRole).Methods("PUT", "OPTIONS")
	adminRouter.HandleFunc("/users/{username}", s.handleAdminDeleteUser).Methods("DELETE", "OPTIONS")

	// System management
	adminRouter.HandleFunc("/system/stats", s.handleAdminSystemStats).Methods("GET", "OPTIONS")
	adminRouter.HandleFunc("/system/reflection", s.handleAdminTriggerReflection).Methods("POST", "OPTIONS")

	// Group management
	adminRouter.HandleFunc("/groups", s.handleAdminListAllGroups).Methods("GET", "OPTIONS")
	adminRouter.HandleFunc("/groups/{id}", s.handleAdminDeleteGroup).Methods("DELETE", "OPTIONS")

	// Activity log
	adminRouter.HandleFunc("/activity", s.handleAdminActivityLog).Methods("GET", "OPTIONS")

	s.logger.Info("Admin routes registered")
}

// handleAdminListUsers lists all registered users
func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get all user keys from Redis
	keys, err := s.agent.RedisClient.Keys(ctx, "user:*").Result()
	if err != nil {
		s.logger.Error("Failed to list users", zap.Error(err))
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}

	users := make([]AdminUser, 0, len(keys))
	adminCount := 0

	for _, key := range keys {
		// Skip role keys
		if strings.HasPrefix(key, "user_role:") {
			continue
		}

		username := strings.TrimPrefix(key, "user:")

		// Get role for this user
		role, err := s.agent.RedisClient.Get(ctx, "user_role:"+username).Result()
		if err != nil {
			role = "user" // Default role
		}

		if role == "admin" {
			adminCount++
		}

		users = append(users, AdminUser{
			Username: username,
			Role:     role,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AdminUserListResponse{
		Users: users,
		Total: len(users),
	})
}

// handleAdminGetUser gets details for a specific user
func (s *Server) handleAdminGetUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	username := vars["username"]

	// Check if user exists
	exists, err := s.agent.RedisClient.Exists(ctx, "user:"+username).Result()
	if err != nil || exists == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Get role
	role, err := s.agent.RedisClient.Get(ctx, "user_role:"+username).Result()
	if err != nil {
		role = "user"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AdminUser{
		Username: username,
		Role:     role,
	})
}

// handleAdminUpdateUserRole updates a user's role
func (s *Server) handleAdminUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	username := vars["username"]
	adminUser := GetUserID(ctx)

	// Prevent admin from demoting themselves
	if username == adminUser {
		http.Error(w, "Cannot modify your own role", http.StatusBadRequest)
		return
	}

	var req UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate role
	if req.Role != "admin" && req.Role != "user" {
		http.Error(w, "Role must be 'admin' or 'user'", http.StatusBadRequest)
		return
	}

	// Check if user exists
	exists, err := s.agent.RedisClient.Exists(ctx, "user:"+username).Result()
	if err != nil || exists == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Update role
	if err := s.agent.RedisClient.Set(ctx, "user_role:"+username, req.Role, 0).Err(); err != nil {
		s.logger.Error("Failed to update user role", zap.Error(err))
		http.Error(w, "Failed to update role", http.StatusInternalServerError)
		return
	}

	s.logger.Info("User role updated",
		zap.String("admin", adminUser),
		zap.String("target_user", username),
		zap.String("new_role", req.Role))

	// Log activity
	s.logActivity(ctx, adminUser, "role_update", "Changed "+username+" role to "+req.Role)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "updated",
		"username": username,
		"role":     req.Role,
	})
}

// handleAdminDeleteUser deletes a user
func (s *Server) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	username := vars["username"]
	adminUser := GetUserID(ctx)

	// Prevent admin from deleting themselves
	if username == adminUser {
		http.Error(w, "Cannot delete your own account", http.StatusBadRequest)
		return
	}

	// Check if user exists
	exists, err := s.agent.RedisClient.Exists(ctx, "user:"+username).Result()
	if err != nil || exists == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Delete user data from Redis
	pipe := s.agent.RedisClient.Pipeline()
	pipe.Del(ctx, "user:"+username)
	pipe.Del(ctx, "user_role:"+username)
	_, err = pipe.Exec(ctx)
	if err != nil {
		s.logger.Error("Failed to delete user", zap.Error(err))
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	s.logger.Info("User deleted",
		zap.String("admin", adminUser),
		zap.String("deleted_user", username))

	// Log activity
	s.logActivity(ctx, adminUser, "user_delete", "Deleted user "+username)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "deleted",
		"username": username,
	})
}

// handleAdminSystemStats returns comprehensive system statistics
func (s *Server) handleAdminSystemStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Count users and admins
	userKeys, _ := s.agent.RedisClient.Keys(ctx, "user:*").Result()
	totalUsers := 0
	totalAdmins := 0

	for _, key := range userKeys {
		if !strings.HasPrefix(key, "user_role:") {
			totalUsers++
			username := strings.TrimPrefix(key, "user:")
			role, _ := s.agent.RedisClient.Get(ctx, "user_role:"+username).Result()
			if role == "admin" {
				totalAdmins++
			}
		}
	}

	// Get kernel stats if available
	var kernelStats map[string]interface{}
	if s.agent.mkClient != nil {
		kernelStats, _ = s.agent.mkClient.GetStats(ctx)
	}

	stats := AdminSystemStats{
		TotalUsers:  totalUsers,
		TotalAdmins: totalAdmins,
		KernelStats: kernelStats,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleAdminTriggerReflection manually triggers a reflection cycle
func (s *Server) handleAdminTriggerReflection(w http.ResponseWriter, r *http.Request) {
	adminUser := GetUserID(r.Context())

	if s.agent.mkClient == nil {
		http.Error(w, "Memory kernel not available", http.StatusServiceUnavailable)
		return
	}

	err := s.agent.mkClient.TriggerReflection(r.Context())
	if err != nil {
		s.logger.Error("Failed to trigger reflection", zap.Error(err))
		http.Error(w, "Failed to trigger reflection", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Reflection triggered by admin", zap.String("admin", adminUser))
	s.logActivity(r.Context(), adminUser, "reflection_trigger", "Manually triggered reflection cycle")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "triggered",
		"message": "Reflection cycle started",
	})
}

// handleAdminListAllGroups lists all groups in the system
func (s *Server) handleAdminListAllGroups(w http.ResponseWriter, r *http.Request) {
	// For now, use a placeholder since we'd need a separate admin query for all groups
	// The kernel's ListGroups is scoped to a user
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"groups":  []interface{}{},
		"message": "All groups listing requires DGraph admin query",
	})
}

// handleAdminDeleteGroup deletes a group
func (s *Server) handleAdminDeleteGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID := vars["id"]
	adminUser := GetUserID(r.Context())

	// For now, return placeholder - would need kernel support for admin delete
	s.logger.Info("Admin group delete requested",
		zap.String("admin", adminUser),
		zap.String("group_id", groupID))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "not_implemented",
		"message": "Group deletion requires kernel support",
	})
}

// handleAdminActivityLog returns recent admin activity
func (s *Server) handleAdminActivityLog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get activity log from Redis (stored as a list)
	activities, err := s.agent.RedisClient.LRange(ctx, "admin_activity_log", 0, 49).Result()
	if err != nil {
		s.logger.Error("Failed to get activity log", zap.Error(err))
		// Return empty log instead of error
		activities = []string{}
	}

	entries := make([]ActivityLogEntry, 0, len(activities))
	for _, activity := range activities {
		var entry ActivityLogEntry
		if err := json.Unmarshal([]byte(activity), &entry); err == nil {
			entries = append(entries, entry)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"activities": entries,
		"total":      len(entries),
	})
}

// logActivity logs an admin activity to Redis
func (s *Server) logActivity(ctx interface{}, userID, action, details string) {
	entry := ActivityLogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		UserID:    userID,
		Action:    action,
		Details:   details,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	// Use background context for logging
	bgCtx := context.Background()
	s.agent.RedisClient.LPush(bgCtx, "admin_activity_log", string(data))
	// Keep only last 100 entries
	s.agent.RedisClient.LTrim(bgCtx, "admin_activity_log", 0, 99)
}
