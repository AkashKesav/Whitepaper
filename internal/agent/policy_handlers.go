package agent

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/reflective-memory-kernel/internal/policy"
	"go.uber.org/zap"
)

// SetupPolicyRoutes configures policy management routes
func (s *Server) SetupPolicyRoutes(r *mux.Router) {
	// Policy routes are admin-only for now
	adminMiddleware := NewAdminMiddleware(s.logger)

	// Mount under /api/admin/policies
	policyRouter := r.PathPrefix("/api/admin/policies").Subrouter()
	policyRouter.Use(NewJWTMiddleware(s.logger).Middleware)
	policyRouter.Use(adminMiddleware.Middleware)

	policyRouter.HandleFunc("", s.handleListPolicies).Methods("GET", "OPTIONS")
	policyRouter.HandleFunc("", s.handleCreatePolicy).Methods("POST", "OPTIONS")
	policyRouter.HandleFunc("/{id}", s.handleDeletePolicy).Methods("DELETE", "OPTIONS")

	// Audit logs
	auditRouter := r.PathPrefix("/api/admin/audit").Subrouter()
	auditRouter.Use(NewJWTMiddleware(s.logger).Middleware)
	auditRouter.Use(adminMiddleware.Middleware)
	auditRouter.HandleFunc("", s.handleGetAuditLogs).Methods("GET", "OPTIONS")

	// Rate limits (User facing? For now make it admin or protected)
	// Let's put rate-limits under /api/rate-limits (protected, self-check)
	// or /api/admin/rate-limits (check others)
	// The handler `handleGetRateLimits` takes a user_id param, so likely Admin checking on a user.
	rlRouter := r.PathPrefix("/api/admin/rate-limits").Subrouter()
	rlRouter.Use(NewJWTMiddleware(s.logger).Middleware)
	rlRouter.Use(adminMiddleware.Middleware)
	rlRouter.HandleFunc("", s.handleGetRateLimits).Methods("GET", "OPTIONS")

	s.logger.Info("Policy routes registered")
}

// handleListPolicies returns all policies
// GET /api/policies
func (s *Server) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	if s.agent.PolicyManager == nil || s.agent.PolicyManager.Store == nil {
		http.Error(w, "Policy store not available", http.StatusServiceUnavailable)
		return
	}

	policies, err := s.agent.PolicyManager.Store.LoadAllPolicies(r.Context())
	if err != nil {
		s.logger.Error("Failed to load policies", zap.Error(err))
		http.Error(w, "Failed to load policies", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"policies": policies,
	})
}

// handleCreatePolicy creates a new policy
// POST /api/policies
func (s *Server) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	if s.agent.PolicyManager == nil || s.agent.PolicyManager.Store == nil {
		http.Error(w, "Policy store not available", http.StatusServiceUnavailable)
		return
	}

	var p policy.Policy
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Default Namespace if missing
	namespace := "system"

	// Save policy
	id, err := s.agent.PolicyManager.Store.SavePolicy(r.Context(), namespace, p, "admin")
	if err != nil {
		s.logger.Error("Failed to save policy", zap.Error(err))
		http.Error(w, "Failed to save policy", http.StatusInternalServerError)
		return
	}

	// Reload policies into engine for this namespace
	s.agent.PolicyManager.LoadPolicies(r.Context(), namespace)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id, "policy_id": p.ID})
}

// handleDeletePolicy deletes a policy
// DELETE /api/policies/{id}
func (s *Server) handleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	if s.agent.PolicyManager == nil || s.agent.PolicyManager.Store == nil {
		http.Error(w, "Policy store not available", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if err := s.agent.PolicyManager.Store.DeletePolicy(r.Context(), id); err != nil {
		s.logger.Error("Failed to delete policy", zap.Error(err))
		http.Error(w, "Failed to delete policy", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "id": id})
}

// handleGetAuditLogs returns audit logs
// GET /api/audit
func (s *Server) handleGetAuditLogs(w http.ResponseWriter, r *http.Request) {
	if s.agent.PolicyManager == nil || s.agent.PolicyManager.AuditLogger == nil {
		http.Error(w, "Audit logger not available", http.StatusServiceUnavailable)
		return
	}

	// Query params
	userID := r.URL.Query().Get("user_id")
	namespace := r.URL.Query().Get("namespace")
	eventType := r.URL.Query().Get("event_type")

	logs, err := s.agent.PolicyManager.AuditLogger.QueryAuditLogs(r.Context(), userID, namespace, policy.AuditEventType(eventType), 100)
	if err != nil {
		s.logger.Error("Failed to query audit logs", zap.Error(err))
		http.Error(w, "Failed to query logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs": logs,
	})
}

// handleGetRateLimits returns rate limit status
// GET /api/rate-limits
func (s *Server) handleGetRateLimits(w http.ResponseWriter, r *http.Request) {
	if s.agent.PolicyManager == nil || s.agent.PolicyManager.RateLimiter == nil {
		http.Error(w, "Rate limiter not available", http.StatusServiceUnavailable)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	tier := policy.RateLimitTier(r.URL.Query().Get("tier"))
	if tier == "" {
		tier = policy.TierFree
	}

	status, err := s.agent.PolicyManager.RateLimiter.GetStatus(r.Context(), userID, tier)
	if err != nil {
		s.logger.Error("Failed to get rate limits", zap.Error(err))
		http.Error(w, "Failed to get rate limits", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": status,
	})
}
