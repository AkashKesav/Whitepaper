package agent

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

type AffiliateDTO struct {
	Code           string  `json:"code"`
	User           string  `json:"user"`
	CommissionRate float64 `json:"commission_rate"`
	TotalEarnings  float64 `json:"total_earnings"`
	Active         bool    `json:"active"`
}

// handleListAffiliates returns all affiliates
// GET /api/admin/affiliates
func (s *Server) handleListAffiliates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Fetch Affiliates from Redis
	val, err := s.agent.RedisClient.HGetAll(ctx, "affiliate:partners").Result()
	if err != nil {
		s.logger.Error("Failed to fetch affiliates", zap.Error(err))
		http.Error(w, "Failed to fetch data", http.StatusInternalServerError)
		return
	}

	affiliates := []AffiliateDTO{}
	for _, jsonAff := range val {
		var aff AffiliateDTO
		if err := json.Unmarshal([]byte(jsonAff), &aff); err == nil {
			affiliates = append(affiliates, aff)
		}
	}

	// 2. Seed Defaults if empty (MVP)
	if len(affiliates) == 0 {
		defaults := []AffiliateDTO{
			{Code: "ALICE20", User: "alice", CommissionRate: 0.20, TotalEarnings: 150.00, Active: true},
			{Code: "BOB10", User: "bob", CommissionRate: 0.10, TotalEarnings: 0.00, Active: false},
		}
		for _, d := range defaults {
			data, _ := json.Marshal(d)
			s.agent.RedisClient.HSet(ctx, "affiliate:partners", d.Code, data)
			affiliates = append(affiliates, d)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"affiliates": affiliates,
	})
}

// handleCreateAffiliate creates a new affiliate
// POST /api/admin/affiliates
func (s *Server) handleCreateAffiliate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var da AffiliateDTO
	if err := json.NewDecoder(r.Body).Decode(&da); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if da.Code == "" || da.User == "" {
		http.Error(w, "Code and User are required", http.StatusBadRequest)
		return
	}

	// Persist to Redis
	data, err := json.Marshal(da)
	if err != nil {
		http.Error(w, "Failed to marshal data", http.StatusInternalServerError)
		return
	}

	if err := s.agent.RedisClient.HSet(ctx, "affiliate:partners", da.Code, data).Err(); err != nil {
		s.logger.Error("Failed to create affiliate", zap.Error(err))
		http.Error(w, "Failed to create affiliate", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Affiliate created", zap.String("code", da.Code))
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(da)
}

// handleDeleteAffiliate deletes an affiliate
// DELETE /api/admin/affiliates/{code}
func (s *Server) handleDeleteAffiliate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	code := vars["code"]

	if err := s.agent.RedisClient.HDel(ctx, "affiliate:partners", code).Err(); err != nil {
		s.logger.Error("Failed to delete affiliate", zap.Error(err))
		http.Error(w, "Failed to delete affiliate", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Affiliate deleted", zap.String("code", code))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "code": code})
}
