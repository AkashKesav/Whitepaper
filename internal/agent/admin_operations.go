package agent

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

type CampaignDTO struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	Status         string    `json:"status"`
	TargetAudience string    `json:"target_audience"`
	ConversionRate float64   `json:"conversion_rate"`
	CreatedAt      time.Time `json:"created_at"`
}

// handleListCampaigns returns all marketing campaigns
// GET /api/admin/operations/campaigns
func (s *Server) handleListCampaigns(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Fetch Campaigns from Redis
	val, err := s.agent.RedisClient.HGetAll(ctx, "operations:campaigns").Result()
	if err != nil {
		s.logger.Error("Failed to fetch campaigns", zap.Error(err))
		http.Error(w, "Failed to fetch data", http.StatusInternalServerError)
		return
	}

	campaigns := []CampaignDTO{}
	for _, jsonCmp := range val {
		var cmp CampaignDTO
		if err := json.Unmarshal([]byte(jsonCmp), &cmp); err == nil {
			campaigns = append(campaigns, cmp)
		}
	}

	// 2. Seed Default Campaigns (MVP)
	if len(campaigns) == 0 {
		defaults := []CampaignDTO{
			{ID: "cmp_1", Name: "Welcome Series", Type: "email", Status: "active", TargetAudience: "new_users", ConversionRate: 0.45, CreatedAt: time.Now().Add(-720 * time.Hour)},
			{ID: "cmp_2", Name: "Pro Upgrade", Type: "in-app", Status: "draft", TargetAudience: "active_users", ConversionRate: 0.0, CreatedAt: time.Now()},
		}
		for _, d := range defaults {
			data, _ := json.Marshal(d)
			s.agent.RedisClient.HSet(ctx, "operations:campaigns", d.ID, data)
			campaigns = append(campaigns, d)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"campaigns": campaigns,
	})
}

// handleCreateCampaign creates a new campaign
// POST /api/admin/operations/campaigns
func (s *Server) handleCreateCampaign(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var dc CampaignDTO
	if err := json.NewDecoder(r.Body).Decode(&dc); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if dc.ID == "" || dc.Name == "" {
		http.Error(w, "ID and Name are required", http.StatusBadRequest)
		return
	}

	// Persist to Redis
	data, err := json.Marshal(dc)
	if err != nil {
		http.Error(w, "Failed to marshal data", http.StatusInternalServerError)
		return
	}

	if err := s.agent.RedisClient.HSet(ctx, "operations:campaigns", dc.ID, data).Err(); err != nil {
		s.logger.Error("Failed to create campaign", zap.Error(err))
		http.Error(w, "Failed to create campaign", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Campaign created", zap.String("id", dc.ID))
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(dc)
}

// handleDeleteCampaign deletes a campaign
// DELETE /api/admin/operations/campaigns/{id}
func (s *Server) handleDeleteCampaign(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	if err := s.agent.RedisClient.HDel(ctx, "operations:campaigns", id).Err(); err != nil {
		s.logger.Error("Failed to delete campaign", zap.Error(err))
		http.Error(w, "Failed to delete campaign", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Campaign deleted", zap.String("id", id))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "id": id})
}
