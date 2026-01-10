package agent

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

type FeatureFlagDTO struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	IsEnabled   bool   `json:"is_enabled"`
	Description string `json:"description"`
}

type ToggleFlagRequest struct {
	Key       string `json:"key"`
	IsEnabled bool   `json:"is_enabled"`
}

// handleListFlags returns all system feature flags
// GET /api/admin/system/flags
func (s *Server) handleListFlags(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Define Default Flags
	defaultFlags := []FeatureFlagDTO{
		{Key: "pre_cortex", Name: "Pre-Cortex Layer", IsEnabled: true, Description: "Semantic caching"},
		{Key: "reflection", Name: "Reflection Engine", IsEnabled: true, Description: "Background processing"},
		{Key: "vision", Name: "Vision Processing", IsEnabled: false, Description: "PDF chart extraction"},
		{Key: "workspace", Name: "Workspace Collaboration", IsEnabled: true, Description: "Team features"},
	}

	// 2. Fetch Stored Flags from Redis
	storedFlags, err := s.agent.RedisClient.HGetAll(ctx, "system:flags").Result()
	if err != nil {
		s.logger.Warn("Failed to fetch system flags from Redis, using defaults", zap.Error(err))
	}

	// 3. Merge Stored Values
	finalFlags := make([]FeatureFlagDTO, len(defaultFlags))
	for i, flag := range defaultFlags {
		finalFlags[i] = flag
		if val, ok := storedFlags[flag.Key]; ok {
			finalFlags[i].IsEnabled = (val == "true")
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"flags": finalFlags,
	})
}

// handleToggleFlag updates a feature flag
// POST /api/admin/system/flags/toggle
func (s *Server) handleToggleFlag(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req ToggleFlagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Persist to Redis
	val := "false"
	if req.IsEnabled {
		val = "true"
	}

	if err := s.agent.RedisClient.HSet(ctx, "system:flags", req.Key, val).Err(); err != nil {
		s.logger.Error("Failed to persist flag toggle", zap.Error(err))
		http.Error(w, "Failed to update flag", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Feature flag toggled",
		zap.String("key", req.Key),
		zap.Bool("new_state", req.IsEnabled),
	)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}
