// Package agent provides admin API handlers for user and system management.
package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
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

	// Operational Excellence Metrics
	PreCortexStats  *PreCortexMetrics    `json:"precortex_stats,omitempty"`
	ReflectionStats *ReflectionMetrics   `json:"reflection_stats,omitempty"`
	SystemHealth    *SystemHealthMetrics `json:"system_health,omitempty"`
	IngestionStats  *IngestionMetrics    `json:"ingestion_stats,omitempty"`
}

// PreCortexMetrics represents Pre-Cortex cache and routing metrics
type PreCortexMetrics struct {
	TotalRequests   int64   `json:"total_requests"`
	CachedResponses int64   `json:"cached_responses"`
	ReflexResponses int64   `json:"reflex_responses"`
	LLMPassthrough  int64   `json:"llm_passthrough"`
	CacheHitRate    float64 `json:"cache_hit_rate"`
	ReflexRate      float64 `json:"reflex_rate"`
	LLMRate         float64 `json:"llm_rate"`
}

// ReflectionMetrics represents reflection engine statistics
type ReflectionMetrics struct {
	CycleCount       int64  `json:"cycle_count"`
	LastCycleTime    string `json:"last_cycle_time"`
	AvgCycleDuration int64  `json:"avg_cycle_duration_ms,omitempty"`
}

// SystemHealthMetrics represents service health status
type SystemHealthMetrics struct {
	ServicesUp     map[string]bool    `json:"services_up"`
	ServiceLatency map[string]float64 `json:"service_latency_ms,omitempty"`
}

// IngestionMetrics represents document ingestion pipeline statistics
type IngestionMetrics struct {
	TotalProcessed       int64   `json:"total_processed"`
	TotalErrors          int64   `json:"total_errors"`
	TotalEntitiesCreated int64   `json:"total_entities_created"`
	SuccessRate          float64 `json:"success_rate"`
	AvgDurationMs        float64 `json:"avg_duration_ms"`
	LastProcessedAt      string  `json:"last_processed_at,omitempty"`
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
	adminRouter.HandleFunc("/users/search", s.handleAdminSearchUsers).Methods("GET", "OPTIONS")
	adminRouter.HandleFunc("/users/{username}", s.handleAdminGetUser).Methods("GET", "OPTIONS")
	adminRouter.HandleFunc("/users/{username}/details", s.handleAdminGetUserDetails).Methods("GET", "OPTIONS")
	adminRouter.HandleFunc("/users/{username}/role", s.handleAdminUpdateUserRole).Methods("PUT", "OPTIONS")
	adminRouter.HandleFunc("/users/{username}", s.handleAdminDeleteUser).Methods("DELETE", "OPTIONS")

	// Trial management
	adminRouter.HandleFunc("/users/{username}/trial", s.handleAdminExtendTrial).Methods("POST", "OPTIONS")

	// System management
	adminRouter.HandleFunc("/system/stats", s.handleAdminSystemStats).Methods("GET", "OPTIONS")
	adminRouter.HandleFunc("/system/reflection", s.handleAdminTriggerReflection).Methods("POST", "OPTIONS")

	// Group management
	adminRouter.HandleFunc("/groups", s.handleAdminListAllGroups).Methods("GET", "OPTIONS")
	adminRouter.HandleFunc("/groups/{id}", s.handleAdminDeleteGroup).Methods("DELETE", "OPTIONS")

	// Activity log
	adminRouter.HandleFunc("/activity", s.handleAdminActivityLog).Methods("GET", "OPTIONS")

	// Dashboard overview
	adminRouter.HandleFunc("/dashboard", s.handleAdminDashboard).Methods("GET", "OPTIONS")

	// Export endpoints
	adminRouter.HandleFunc("/export/users", s.handleAdminExportUsers).Methods("GET", "OPTIONS")
	adminRouter.HandleFunc("/export/activity", s.handleAdminExportActivity).Methods("GET", "OPTIONS")

	// Batch operations
	adminRouter.HandleFunc("/batch/role", s.handleAdminBatchUpdateRole).Methods("POST", "OPTIONS")
	adminRouter.HandleFunc("/batch/delete", s.handleAdminBatchDelete).Methods("POST", "OPTIONS")

	// System maintenance
	adminRouter.HandleFunc("/system/cache/clear", s.handleAdminClearCache).Methods("POST", "OPTIONS")
	adminRouter.HandleFunc("/system/info", s.handleAdminSystemInfo).Methods("GET", "OPTIONS")

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
	var ingestionStats *IngestionMetrics
	var reflectionStats *ReflectionMetrics

	if s.agent.mkClient != nil {
		kernelStats, _ = s.agent.mkClient.GetStats(ctx)

		// Extract ingestion stats from kernel stats
		if ingestion, ok := kernelStats["ingestion"].(map[string]interface{}); ok {
			var totalProcessed, totalErrors, totalEntities int64
			var avgDuration float64
			var lastProcessed string

			if v, ok := ingestion["total_processed"].(float64); ok {
				totalProcessed = int64(v)
			}
			if v, ok := ingestion["total_errors"].(float64); ok {
				totalErrors = int64(v)
			}
			if v, ok := ingestion["total_entities_created"].(float64); ok {
				totalEntities = int64(v)
			}
			if v, ok := ingestion["avg_duration_ms"].(float64); ok {
				avgDuration = v
			}
			if v, ok := ingestion["last_processed_at"].(string); ok {
				lastProcessed = v
			}

			successRate := float64(0)
			if totalProcessed > 0 {
				successRate = float64(totalProcessed-totalErrors) / float64(totalProcessed) * 100
			}

			ingestionStats = &IngestionMetrics{
				TotalProcessed:       totalProcessed,
				TotalErrors:          totalErrors,
				TotalEntitiesCreated: totalEntities,
				SuccessRate:          successRate,
				AvgDurationMs:        avgDuration,
				LastProcessedAt:      lastProcessed,
			}
		}
	}

	// Get Pre-Cortex stats if available
	var preCortexStats *PreCortexMetrics
	if s.agent.preCortex != nil {
		total, cached, reflex, llm, hitRate := s.agent.preCortex.Stats()
		reflexRate := float64(0)
		llmRate := float64(0)
		if total > 0 {
			reflexRate = float64(reflex) / float64(total) * 100
			llmRate = float64(llm) / float64(total) * 100
		}
		preCortexStats = &PreCortexMetrics{
			TotalRequests:   total,
			CachedResponses: cached,
			ReflexResponses: reflex,
			LLMPassthrough:  llm,
			CacheHitRate:    hitRate * 100, // Convert to percentage
			ReflexRate:      reflexRate,
			LLMRate:         llmRate,
		}
	}

	// Get system health - check service connectivity
	systemHealth := &SystemHealthMetrics{
		ServicesUp: map[string]bool{
			"redis": s.agent.RedisClient != nil,
			"nats":  s.agent.natsConn != nil && s.agent.natsConn.IsConnected(),
		},
	}
	if s.agent.mkClient != nil {
		systemHealth.ServicesUp["memory_kernel"] = true
	}

	stats := AdminSystemStats{
		TotalUsers:      totalUsers,
		TotalAdmins:     totalAdmins,
		KernelStats:     kernelStats,
		Timestamp:       time.Now().Format(time.RFC3339),
		PreCortexStats:  preCortexStats,
		ReflectionStats: reflectionStats,
		SystemHealth:    systemHealth,
		IngestionStats:  ingestionStats,
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

// AdminGroup represents a group for admin views
type AdminGroup struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	OwnerID     string   `json:"owner_id"`
	MemberCount int      `json:"member_count"`
	Members     []string `json:"members,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty"`
}

// handleAdminListAllGroups lists all groups in the system
func (s *Server) handleAdminListAllGroups(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get all group keys from Redis
	groupKeys, err := s.agent.RedisClient.Keys(ctx, "group:*").Result()
	if err != nil {
		s.logger.Error("Failed to list groups", zap.Error(err))
		http.Error(w, "Failed to list groups", http.StatusInternalServerError)
		return
	}

	groups := make([]AdminGroup, 0)

	for _, key := range groupKeys {
		// Skip member keys and other sub-keys
		if strings.Contains(key, ":members") || strings.Contains(key, ":settings") {
			continue
		}

		groupID := strings.TrimPrefix(key, "group:")

		// Get group data
		groupData, err := s.agent.RedisClient.HGetAll(ctx, key).Result()
		if err != nil {
			continue
		}

		// Get member count
		memberKey := "group:" + groupID + ":members"
		memberCount, _ := s.agent.RedisClient.SCard(ctx, memberKey).Result()

		// Get members list
		members, _ := s.agent.RedisClient.SMembers(ctx, memberKey).Result()

		groups = append(groups, AdminGroup{
			ID:          groupID,
			Name:        groupData["name"],
			Description: groupData["description"],
			OwnerID:     groupData["owner_id"],
			MemberCount: int(memberCount),
			Members:     members,
			CreatedAt:   groupData["created_at"],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"groups": groups,
		"total":  len(groups),
	})
}

// handleAdminDeleteGroup deletes a group and its related data
func (s *Server) handleAdminDeleteGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	groupID := vars["id"]
	adminUser := GetUserID(ctx)

	// Check if group exists
	exists, err := s.agent.RedisClient.Exists(ctx, "group:"+groupID).Result()
	if err != nil || exists == 0 {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}

	// Delete group and related keys
	pipe := s.agent.RedisClient.Pipeline()
	pipe.Del(ctx, "group:"+groupID)
	pipe.Del(ctx, "group:"+groupID+":members")
	pipe.Del(ctx, "group:"+groupID+":settings")
	_, err = pipe.Exec(ctx)

	if err != nil {
		s.logger.Error("Failed to delete group", zap.Error(err), zap.String("group_id", groupID))
		http.Error(w, "Failed to delete group", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Admin deleted group",
		zap.String("admin", adminUser),
		zap.String("group_id", groupID))

	s.logActivity(ctx, adminUser, "group_delete", "Deleted group: "+groupID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "deleted",
		"group_id": groupID,
	})
}

// handleAdminActivityLog returns recent admin activity with optional filtering
func (s *Server) handleAdminActivityLog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters for filtering
	actionFilter := r.URL.Query().Get("action")
	userFilter := r.URL.Query().Get("user")
	limitStr := r.URL.Query().Get("limit")

	limit := 50 // default limit
	if limitStr != "" {
		if parsed, err := json.Number(limitStr).Int64(); err == nil && parsed > 0 && parsed <= 100 {
			limit = int(parsed)
		}
	}

	// Get activity log from Redis (stored as a list)
	activities, err := s.agent.RedisClient.LRange(ctx, "admin_activity_log", 0, 99).Result()
	if err != nil {
		s.logger.Error("Failed to get activity log", zap.Error(err))
		// Return empty log instead of error
		activities = []string{}
	}

	entries := make([]ActivityLogEntry, 0)
	for _, activity := range activities {
		var entry ActivityLogEntry
		if err := json.Unmarshal([]byte(activity), &entry); err == nil {
			// Apply filters
			if actionFilter != "" && entry.Action != actionFilter {
				continue
			}
			if userFilter != "" && entry.UserID != userFilter {
				continue
			}
			entries = append(entries, entry)
			if len(entries) >= limit {
				break
			}
		}
	}

	// Get unique action types for filter dropdown
	actionTypes := make(map[string]bool)
	for _, activity := range activities {
		var entry ActivityLogEntry
		if err := json.Unmarshal([]byte(activity), &entry); err == nil {
			actionTypes[entry.Action] = true
		}
	}

	uniqueActions := make([]string, 0, len(actionTypes))
	for action := range actionTypes {
		uniqueActions = append(uniqueActions, action)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"activities":   entries,
		"total":        len(entries),
		"action_types": uniqueActions,
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

// handleAdminSearchUsers searches and filters users
func (s *Server) handleAdminSearchUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	query := strings.ToLower(r.URL.Query().Get("q"))
	roleFilter := r.URL.Query().Get("role") // "admin", "user", or empty for all

	// Get all user keys from Redis
	keys, err := s.agent.RedisClient.Keys(ctx, "user:*").Result()
	if err != nil {
		s.logger.Error("Failed to search users", zap.Error(err))
		http.Error(w, "Failed to search users", http.StatusInternalServerError)
		return
	}

	users := make([]AdminUser, 0)

	for _, key := range keys {
		if strings.HasPrefix(key, "user_role:") {
			continue
		}

		username := strings.TrimPrefix(key, "user:")

		// Filter by search query
		if query != "" && !strings.Contains(strings.ToLower(username), query) {
			continue
		}

		// Get role
		role, err := s.agent.RedisClient.Get(ctx, "user_role:"+username).Result()
		if err != nil {
			role = "user"
		}

		// Filter by role
		if roleFilter != "" && role != roleFilter {
			continue
		}

		users = append(users, AdminUser{
			Username: username,
			Role:     role,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users": users,
		"total": len(users),
		"query": query,
		"role":  roleFilter,
	})
}

// UserDetails represents detailed user information
type UserDetails struct {
	Username       string                 `json:"username"`
	Role           string                 `json:"role"`
	CreatedAt      string                 `json:"created_at,omitempty"`
	LastActive     string                 `json:"last_active,omitempty"`
	TrialExpiresAt string                 `json:"trial_expires_at,omitempty"`
	IsTrial        bool                   `json:"is_trial"`
	MemoryStats    map[string]interface{} `json:"memory_stats,omitempty"`
	GroupCount     int                    `json:"group_count"`
}

// handleAdminGetUserDetails gets comprehensive user details
func (s *Server) handleAdminGetUserDetails(w http.ResponseWriter, r *http.Request) {
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

	// Get trial expiration
	trialExpires, _ := s.agent.RedisClient.Get(ctx, "user_trial:"+username).Result()
	isTrial := trialExpires != ""

	// Get last active time
	lastActive, _ := s.agent.RedisClient.Get(ctx, "user_last_active:"+username).Result()

	// Get created at time
	createdAt, _ := s.agent.RedisClient.Get(ctx, "user_created:"+username).Result()

	// Get memory stats from kernel if available
	var memoryStats map[string]interface{}
	if s.agent.mkClient != nil {
		// Get user-specific stats
		memoryStats, _ = s.agent.mkClient.GetStats(ctx)
	}

	// Count user's groups
	groupCount := 0
	groupKeys, _ := s.agent.RedisClient.Keys(ctx, "group_member:"+username+":*").Result()
	groupCount = len(groupKeys)

	details := UserDetails{
		Username:       username,
		Role:           role,
		CreatedAt:      createdAt,
		LastActive:     lastActive,
		TrialExpiresAt: trialExpires,
		IsTrial:        isTrial,
		MemoryStats:    memoryStats,
		GroupCount:     groupCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(details)
}

// ExtendTrialRequest represents a request to extend a user's trial
type ExtendTrialRequest struct {
	Days int `json:"days"` // Number of days to extend
}

// handleAdminExtendTrial extends a user's trial period
func (s *Server) handleAdminExtendTrial(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	username := vars["username"]
	adminUser := GetUserID(ctx)

	// Parse request
	var req ExtendTrialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate days
	if req.Days < 1 || req.Days > 365 {
		http.Error(w, "Days must be between 1 and 365", http.StatusBadRequest)
		return
	}

	// Check if user exists
	exists, err := s.agent.RedisClient.Exists(ctx, "user:"+username).Result()
	if err != nil || exists == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Get current trial expiration or use current time
	currentExpiry := time.Now()
	currentExpiryStr, err := s.agent.RedisClient.Get(ctx, "user_trial:"+username).Result()
	if err == nil && currentExpiryStr != "" {
		parsed, err := time.Parse(time.RFC3339, currentExpiryStr)
		if err == nil && parsed.After(time.Now()) {
			currentExpiry = parsed
		}
	}

	// Calculate new expiry
	newExpiry := currentExpiry.AddDate(0, 0, req.Days)

	// Save new expiry
	err = s.agent.RedisClient.Set(ctx, "user_trial:"+username, newExpiry.Format(time.RFC3339), 0).Err()
	if err != nil {
		s.logger.Error("Failed to extend trial", zap.Error(err))
		http.Error(w, "Failed to extend trial", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Trial extended",
		zap.String("admin", adminUser),
		zap.String("user", username),
		zap.Int("days", req.Days),
		zap.Time("new_expiry", newExpiry))

	// Log activity
	s.logActivity(ctx, adminUser, "trial_extend", "Extended "+username+"'s trial by "+string(rune(req.Days))+" days")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "extended",
		"username":   username,
		"days_added": req.Days,
		"expires_at": newExpiry.Format(time.RFC3339),
	})
}

// DashboardOverview represents comprehensive admin dashboard data
type DashboardOverview struct {
	UserStats     *UserStatsOverview `json:"user_stats"`
	RecentActions []ActivityLogEntry `json:"recent_actions"`
	SystemHealth  map[string]bool    `json:"system_health"`
	QuickActions  []QuickAction      `json:"quick_actions"`
	Timestamp     string             `json:"timestamp"`
}

// UserStatsOverview provides user-related stats
type UserStatsOverview struct {
	TotalUsers   int `json:"total_users"`
	TotalAdmins  int `json:"total_admins"`
	ActiveTrials int `json:"active_trials"`
	NewThisWeek  int `json:"new_this_week"`
}

// QuickAction represents an available quick action
type QuickAction struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Icon  string `json:"icon"`
}

// handleAdminDashboard returns comprehensive dashboard overview
func (s *Server) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Count users
	userKeys, _ := s.agent.RedisClient.Keys(ctx, "user:*").Result()
	totalUsers := 0
	totalAdmins := 0
	activeTrials := 0

	oneWeekAgo := time.Now().AddDate(0, 0, -7)

	for _, key := range userKeys {
		if strings.HasPrefix(key, "user_role:") || strings.HasPrefix(key, "user_trial:") ||
			strings.HasPrefix(key, "user_created:") || strings.HasPrefix(key, "user_last_active:") {
			continue
		}

		totalUsers++
		username := strings.TrimPrefix(key, "user:")

		// Check role
		role, _ := s.agent.RedisClient.Get(ctx, "user_role:"+username).Result()
		if role == "admin" {
			totalAdmins++
		}

		// Check trial status
		trialExpiry, err := s.agent.RedisClient.Get(ctx, "user_trial:"+username).Result()
		if err == nil && trialExpiry != "" {
			expiry, _ := time.Parse(time.RFC3339, trialExpiry)
			if expiry.After(time.Now()) {
				activeTrials++
			}
		}
	}

	// Count new users this week (simplified - would need created_at tracking)
	newThisWeek := 0
	for _, key := range userKeys {
		if strings.HasPrefix(key, "user_created:") {
			continue
		}
		username := strings.TrimPrefix(key, "user:")
		createdAt, err := s.agent.RedisClient.Get(ctx, "user_created:"+username).Result()
		if err == nil && createdAt != "" {
			created, _ := time.Parse(time.RFC3339, createdAt)
			if created.After(oneWeekAgo) {
				newThisWeek++
			}
		}
	}

	// Get recent activity (last 5)
	activities, _ := s.agent.RedisClient.LRange(ctx, "admin_activity_log", 0, 4).Result()
	recentActions := make([]ActivityLogEntry, 0, len(activities))
	for _, activity := range activities {
		var entry ActivityLogEntry
		if err := json.Unmarshal([]byte(activity), &entry); err == nil {
			recentActions = append(recentActions, entry)
		}
	}

	// System health
	systemHealth := map[string]bool{
		"redis": s.agent.RedisClient != nil,
		"nats":  s.agent.natsConn != nil && s.agent.natsConn.IsConnected(),
	}
	if s.agent.mkClient != nil {
		systemHealth["memory_kernel"] = true
	}
	if s.agent.preCortex != nil {
		systemHealth["precortex"] = true
	}

	// Quick actions
	quickActions := []QuickAction{
		{ID: "trigger_reflection", Label: "Trigger Reflection", Icon: "refresh"},
		{ID: "export_users", Label: "Export Users", Icon: "download"},
		{ID: "clear_cache", Label: "Clear Cache", Icon: "trash"},
	}

	overview := DashboardOverview{
		UserStats: &UserStatsOverview{
			TotalUsers:   totalUsers,
			TotalAdmins:  totalAdmins,
			ActiveTrials: activeTrials,
			NewThisWeek:  newThisWeek,
		},
		RecentActions: recentActions,
		SystemHealth:  systemHealth,
		QuickActions:  quickActions,
		Timestamp:     time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(overview)
}

// ExportUser represents a user for export
type ExportUser struct {
	Username  string `json:"username" csv:"username"`
	Role      string `json:"role" csv:"role"`
	IsTrial   bool   `json:"is_trial" csv:"is_trial"`
	CreatedAt string `json:"created_at" csv:"created_at"`
}

// handleAdminExportUsers exports users as JSON or CSV
func (s *Server) handleAdminExportUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	format := r.URL.Query().Get("format") // "json" or "csv"
	if format == "" {
		format = "json"
	}

	// Get all users
	keys, err := s.agent.RedisClient.Keys(ctx, "user:*").Result()
	if err != nil {
		s.logger.Error("Failed to export users", zap.Error(err))
		http.Error(w, "Failed to export users", http.StatusInternalServerError)
		return
	}

	users := make([]ExportUser, 0)
	for _, key := range keys {
		if strings.HasPrefix(key, "user_role:") || strings.HasPrefix(key, "user_trial:") ||
			strings.HasPrefix(key, "user_created:") || strings.HasPrefix(key, "user_last_active:") {
			continue
		}

		username := strings.TrimPrefix(key, "user:")
		role, _ := s.agent.RedisClient.Get(ctx, "user_role:"+username).Result()
		if role == "" {
			role = "user"
		}
		trialExpiry, _ := s.agent.RedisClient.Get(ctx, "user_trial:"+username).Result()
		createdAt, _ := s.agent.RedisClient.Get(ctx, "user_created:"+username).Result()

		users = append(users, ExportUser{
			Username:  username,
			Role:      role,
			IsTrial:   trialExpiry != "",
			CreatedAt: createdAt,
		})
	}

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=users_export.csv")
		w.Write([]byte("username,role,is_trial,created_at\n"))
		for _, u := range users {
			line := u.Username + "," + u.Role + "," +
				func() string {
					if u.IsTrial {
						return "true"
					} else {
						return "false"
					}
				}() + "," +
				u.CreatedAt + "\n"
			w.Write([]byte(line))
		}
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=users_export.json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"users":       users,
			"total":       len(users),
			"exported_at": time.Now().Format(time.RFC3339),
		})
	}
}

// handleAdminExportActivity exports activity log
func (s *Server) handleAdminExportActivity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	activities, _ := s.agent.RedisClient.LRange(ctx, "admin_activity_log", 0, 999).Result()
	entries := make([]ActivityLogEntry, 0, len(activities))
	for _, activity := range activities {
		var entry ActivityLogEntry
		if err := json.Unmarshal([]byte(activity), &entry); err == nil {
			entries = append(entries, entry)
		}
	}

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=activity_export.csv")
		w.Write([]byte("timestamp,user_id,action,details\n"))
		for _, a := range entries {
			line := a.Timestamp + "," + a.UserID + "," + a.Action + ",\"" + a.Details + "\"\n"
			w.Write([]byte(line))
		}
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=activity_export.json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"activities":  entries,
			"total":       len(entries),
			"exported_at": time.Now().Format(time.RFC3339),
		})
	}
}

// BatchRoleRequest represents a batch role update request
type BatchRoleRequest struct {
	Usernames []string `json:"usernames"`
	Role      string   `json:"role"` // "admin" or "user"
}

// handleAdminBatchUpdateRole updates role for multiple users
func (s *Server) handleAdminBatchUpdateRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	adminUser := GetUserID(ctx)

	var req BatchRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Role != "admin" && req.Role != "user" {
		http.Error(w, "Role must be 'admin' or 'user'", http.StatusBadRequest)
		return
	}

	if len(req.Usernames) == 0 {
		http.Error(w, "No usernames provided", http.StatusBadRequest)
		return
	}

	if len(req.Usernames) > 50 {
		http.Error(w, "Maximum 50 users per batch", http.StatusBadRequest)
		return
	}

	updated := 0
	failed := 0

	for _, username := range req.Usernames {
		// Skip self
		if username == adminUser {
			failed++
			continue
		}

		// Check if user exists
		exists, _ := s.agent.RedisClient.Exists(ctx, "user:"+username).Result()
		if exists == 0 {
			failed++
			continue
		}

		// Update role
		err := s.agent.RedisClient.Set(ctx, "user_role:"+username, req.Role, 0).Err()
		if err != nil {
			failed++
		} else {
			updated++
		}
	}

	s.logger.Info("Batch role update",
		zap.String("admin", adminUser),
		zap.Int("updated", updated),
		zap.Int("failed", failed),
		zap.String("role", req.Role))

	s.logActivity(ctx, adminUser, "batch_role_update", "Updated "+string(rune(updated))+" users to "+req.Role)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "completed",
		"updated": updated,
		"failed":  failed,
		"role":    req.Role,
	})
}

// BatchDeleteRequest represents a batch delete request
type BatchDeleteRequest struct {
	Usernames []string `json:"usernames"`
}

// handleAdminBatchDelete deletes multiple users
func (s *Server) handleAdminBatchDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	adminUser := GetUserID(ctx)

	var req BatchDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Usernames) == 0 {
		http.Error(w, "No usernames provided", http.StatusBadRequest)
		return
	}

	if len(req.Usernames) > 50 {
		http.Error(w, "Maximum 50 users per batch", http.StatusBadRequest)
		return
	}

	deleted := 0
	failed := 0

	for _, username := range req.Usernames {
		// Skip self
		if username == adminUser {
			failed++
			continue
		}

		// Check if user exists
		exists, _ := s.agent.RedisClient.Exists(ctx, "user:"+username).Result()
		if exists == 0 {
			failed++
			continue
		}

		// Delete user
		pipe := s.agent.RedisClient.Pipeline()
		pipe.Del(ctx, "user:"+username)
		pipe.Del(ctx, "user_role:"+username)
		pipe.Del(ctx, "user_trial:"+username)
		pipe.Del(ctx, "user_created:"+username)
		pipe.Del(ctx, "user_last_active:"+username)
		_, err := pipe.Exec(ctx)
		if err != nil {
			failed++
		} else {
			deleted++
		}
	}

	s.logger.Info("Batch delete",
		zap.String("admin", adminUser),
		zap.Int("deleted", deleted),
		zap.Int("failed", failed))

	s.logActivity(ctx, adminUser, "batch_delete", "Deleted "+string(rune(deleted))+" users")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "completed",
		"deleted": deleted,
		"failed":  failed,
	})
}

// ClearCacheRequest represents a cache clear request
type ClearCacheRequest struct {
	Type string `json:"type"` // "all", "sessions", "users", "groups"
}

// handleAdminClearCache clears specified cache data
func (s *Server) handleAdminClearCache(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	adminUser := GetUserID(ctx)

	var req ClearCacheRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Type = "sessions" // Default to sessions only
	}

	cleared := 0
	var pattern string

	switch req.Type {
	case "all":
		// Clear all cache keys (dangerous!)
		pattern = "cache:*"
	case "sessions":
		pattern = "session:*"
	case "precortex":
		pattern = "precortex:*"
	default:
		pattern = "cache:*"
	}

	// Get and delete matching keys
	keys, err := s.agent.RedisClient.Keys(ctx, pattern).Result()
	if err == nil {
		for _, key := range keys {
			if err := s.agent.RedisClient.Del(ctx, key).Err(); err == nil {
				cleared++
			}
		}
	}

	s.logger.Info("Admin cleared cache",
		zap.String("admin", adminUser),
		zap.String("type", req.Type),
		zap.Int("cleared", cleared))

	s.logActivity(ctx, adminUser, "cache_clear", "Cleared "+req.Type+" cache: "+string(rune(cleared))+" keys")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "cleared",
		"type":    req.Type,
		"cleared": cleared,
	})
}

// SystemInfo represents detailed system information
type SystemInfo struct {
	Version       string            `json:"version"`
	Uptime        string            `json:"uptime"`
	GoVersion     string            `json:"go_version"`
	NumGoroutines int               `json:"num_goroutines"`
	MemoryUsage   map[string]uint64 `json:"memory_usage"`
	Connections   map[string]bool   `json:"connections"`
	Features      map[string]bool   `json:"features"`
}

// handleAdminSystemInfo returns detailed system information
func (s *Server) handleAdminSystemInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Check connections
	connections := map[string]bool{
		"redis": s.agent.RedisClient != nil,
	}
	if s.agent.natsConn != nil {
		connections["nats"] = s.agent.natsConn.IsConnected()
	}
	if s.agent.mkClient != nil {
		connections["memory_kernel"] = true
	}
	if s.agent.preCortex != nil {
		connections["precortex"] = true
	}

	// Check available features
	features := map[string]bool{
		"pre_cortex":    s.agent.preCortex != nil,
		"memory_kernel": s.agent.mkClient != nil,
		"nats":          s.agent.natsConn != nil,
		"groups":        true,
		"reflection":    s.agent.mkClient != nil,
	}

	// Get Redis info
	redisInfo, _ := s.agent.RedisClient.Info(ctx, "server").Result()
	_ = redisInfo // Could parse for version info

	info := SystemInfo{
		Version:       "1.0.0", // Could be injected at build time
		GoVersion:     runtime.Version(),
		NumGoroutines: runtime.NumGoroutine(),
		MemoryUsage: map[string]uint64{
			"alloc":       memStats.Alloc,
			"total_alloc": memStats.TotalAlloc,
			"sys":         memStats.Sys,
			"heap_alloc":  memStats.HeapAlloc,
			"heap_sys":    memStats.HeapSys,
		},
		Connections: connections,
		Features:    features,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}
