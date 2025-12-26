// Package agent provides HTTP handlers for graph traversal APIs.
package agent

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/reflective-memory-kernel/internal/graph"
	"go.uber.org/zap"
)

// ============================================================================
// Spreading Activation API
// ============================================================================

// SpreadActivationRequest represents the request for spreading activation traversal
type SpreadActivationRequest struct {
	StartUID      string  `json:"start_uid"`
	StartName     string  `json:"start_name"` // Alternative: find by name
	Namespace     string  `json:"namespace"`
	DecayFactor   float64 `json:"decay_factor"`   // 0.0-1.0, default 0.5
	MaxHops       int     `json:"max_hops"`       // default 3
	MinActivation float64 `json:"min_activation"` // default 0.1
	MaxResults    int     `json:"max_results"`    // default 50
}

func (s *Server) handleSpreadActivation(w http.ResponseWriter, r *http.Request) {
	var req SpreadActivationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get start UID from name if needed
	startUID := req.StartUID
	if startUID == "" && req.StartName != "" {
		node, err := s.agent.mkClient.FindNodeByName(r.Context(), req.StartName, graph.NodeTypeEntity)
		if err != nil || node == nil {
			http.Error(w, "Start node not found", http.StatusNotFound)
			return
		}
		startUID = node.UID
	}
	if startUID == "" {
		http.Error(w, "start_uid or start_name is required", http.StatusBadRequest)
		return
	}

	opts := graph.SpreadActivationOpts{
		StartUID:      startUID,
		Namespace:     req.Namespace,
		DecayFactor:   req.DecayFactor,
		MaxHops:       req.MaxHops,
		MinActivation: req.MinActivation,
		MaxResults:    req.MaxResults,
	}

	result, err := s.agent.mkClient.SpreadActivation(r.Context(), opts)
	if err != nil {
		s.logger.Error("Spread activation failed", zap.Error(err))
		http.Error(w, "Traversal failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes":       result,
		"total_count": len(result),
	})
}

// ============================================================================
// Community Traversal API
// ============================================================================

// CommunityTraversalRequest represents the request for community-based traversal
type CommunityTraversalRequest struct {
	EntityName string `json:"entity_name"`
	Namespace  string `json:"namespace"`
	MaxResults int    `json:"max_results"` // default 100
}

func (s *Server) handleCommunityTraversal(w http.ResponseWriter, r *http.Request) {
	var req CommunityTraversalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.EntityName == "" {
		http.Error(w, "entity_name is required", http.StatusBadRequest)
		return
	}

	opts := graph.CommunityTraversalOpts{
		EntityName: req.EntityName,
		Namespace:  req.Namespace,
		MaxResults: req.MaxResults,
	}

	result, err := s.agent.mkClient.TraverseViaCommunity(r.Context(), opts)
	if err != nil {
		s.logger.Error("Community traversal failed", zap.Error(err))
		http.Error(w, "Traversal failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// ============================================================================
// Temporal Decay Query API
// ============================================================================

// TemporalQueryRequest represents the request for temporal decay queries
type TemporalQueryRequest struct {
	Namespace     string  `json:"namespace"`
	MinActivation float64 `json:"min_activation"` // default 0.1
	RecencyDays   int     `json:"recency_days"`   // default 7
	RecencyWeight float64 `json:"recency_weight"` // 0.0-1.0, default 0.3
	MaxResults    int     `json:"max_results"`    // default 50
}

func (s *Server) handleTemporalQuery(w http.ResponseWriter, r *http.Request) {
	var req TemporalQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Convert recency days to duration
	recencyCutoff := time.Duration(req.RecencyDays) * 24 * time.Hour
	if recencyCutoff == 0 {
		recencyCutoff = 7 * 24 * time.Hour // Default 7 days
	}

	opts := graph.TemporalQueryOpts{
		Namespace:     req.Namespace,
		MinActivation: req.MinActivation,
		RecencyCutoff: recencyCutoff,
		RecencyWeight: req.RecencyWeight,
		MaxResults:    req.MaxResults,
	}

	result, err := s.agent.mkClient.QueryWithTemporalDecay(r.Context(), opts)
	if err != nil {
		s.logger.Error("Temporal query failed", zap.Error(err))
		http.Error(w, "Query failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes":       result,
		"total_count": len(result),
	})
}

// ============================================================================
// Node Expansion API
// ============================================================================

// ExpandNodeRequest represents the request for multi-hop expansion
type ExpandNodeRequest struct {
	StartUID   string   `json:"start_uid"`
	StartName  string   `json:"start_name"`  // Alternative: find by name
	EdgeTypes  []string `json:"edge_types"`  // Optional filter
	MaxHops    int      `json:"max_hops"`    // default 2
	MaxResults int      `json:"max_results"` // default 100
}

func (s *Server) handleExpandNode(w http.ResponseWriter, r *http.Request) {
	var req ExpandNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get start UID from name if needed
	startUID := req.StartUID
	if startUID == "" && req.StartName != "" {
		node, err := s.agent.mkClient.FindNodeByName(r.Context(), req.StartName, graph.NodeTypeEntity)
		if err != nil || node == nil {
			http.Error(w, "Start node not found", http.StatusNotFound)
			return
		}
		startUID = node.UID
	}
	if startUID == "" {
		http.Error(w, "start_uid or start_name is required", http.StatusBadRequest)
		return
	}

	opts := graph.ExpandOpts{
		StartUID:   startUID,
		EdgeTypes:  req.EdgeTypes,
		MaxHops:    req.MaxHops,
		MaxResults: req.MaxResults,
	}

	result, err := s.agent.mkClient.ExpandFromNode(r.Context(), opts)
	if err != nil {
		s.logger.Error("Node expansion failed", zap.Error(err))
		http.Error(w, "Expansion failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
