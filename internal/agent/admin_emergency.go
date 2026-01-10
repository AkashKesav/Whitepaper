package agent

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

type EmergencyRequestDTO struct {
	ID          string    `json:"id"`
	Reason      string    `json:"reason"`
	Duration    string    `json:"duration"`
	Status      string    `json:"status"`
	RequestedBy string    `json:"requested_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type EmergencyActionRequest struct {
	Action string `json:"action"` // approve, deny
}

// handleListEmergencyRequests returns all emergency access requests
// GET /api/admin/emergency/requests
func (s *Server) handleListEmergencyRequests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Fetch Requests from Redis
	val, err := s.agent.RedisClient.HGetAll(ctx, "emergency:requests").Result()
	if err != nil {
		s.logger.Error("Failed to fetch emergency requests", zap.Error(err))
		http.Error(w, "Failed to fetch data", http.StatusInternalServerError)
		return
	}

	requests := []EmergencyRequestDTO{}
	for _, jsonReq := range val {
		var req EmergencyRequestDTO
		if err := json.Unmarshal([]byte(jsonReq), &req); err == nil {
			requests = append(requests, req)
		}
	}

	// 2. Seed defaults if empty (MVP)
	if len(requests) == 0 {
		defaults := []EmergencyRequestDTO{
			{ID: "er_1", Reason: "Production DB Incident #402", Duration: "2h", Status: "pending", RequestedBy: "ops_lead", CreatedAt: time.Now().Add(-10 * time.Minute)},
			{ID: "er_2", Reason: "Audit Log Verification", Duration: "4h", Status: "approved", RequestedBy: "security_auditor", CreatedAt: time.Now().Add(-24 * time.Hour)},
		}
		for _, d := range defaults {
			data, _ := json.Marshal(d)
			s.agent.RedisClient.HSet(ctx, "emergency:requests", d.ID, data)
			requests = append(requests, d)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"requests": requests,
	})
}

// handleApproveEmergencyRequest approves a request
// POST /api/admin/emergency/requests/{id}/approve
func (s *Server) handleApproveEmergencyRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	requestID := vars["id"]
	ctx := r.Context()

	// 1. Get existing request
	val, err := s.agent.RedisClient.HGet(ctx, "emergency:requests", requestID).Result()
	if err != nil {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}

	// 2. Update Status
	var req EmergencyRequestDTO
	if err := json.Unmarshal([]byte(val), &req); err != nil {
		http.Error(w, "Data corruption", http.StatusInternalServerError)
		return
	}
	req.Status = "approved"

	// 3. Save back
	newData, _ := json.Marshal(req)
	s.agent.RedisClient.HSet(ctx, "emergency:requests", requestID, newData)

	s.logger.Info("Emergency Access Approved", zap.String("request_id", requestID))

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "approved", "id": requestID})
}

// handleDenyEmergencyRequest denies a request
// POST /api/admin/emergency/requests/{id}/deny
func (s *Server) handleDenyEmergencyRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	requestID := vars["id"]
	ctx := r.Context()

	// 1. Get existing request
	val, err := s.agent.RedisClient.HGet(ctx, "emergency:requests", requestID).Result()
	if err != nil {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}

	// 2. Update Status
	var req EmergencyRequestDTO
	if err := json.Unmarshal([]byte(val), &req); err != nil {
		http.Error(w, "Data corruption", http.StatusInternalServerError)
		return
	}
	req.Status = "denied"

	// 3. Save back
	newData, _ := json.Marshal(req)
	s.agent.RedisClient.HSet(ctx, "emergency:requests", requestID, newData)

	s.logger.Info("Emergency Access Denied", zap.String("request_id", requestID))

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "denied", "id": requestID})
}
