// Package kernel provides the consultation handler for the Memory Kernel.
// This implements Phase 3 of the three-phase loop: answering queries from
// the Front-End Agent with pre-synthesized, relevant information.
package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/graph"
)

// ConsultationHandler handles consultation requests from the Front-End Agent
type ConsultationHandler struct {
	graphClient   *graph.Client
	queryBuilder  *graph.QueryBuilder
	redisClient   *redis.Client
	aiServicesURL string
	logger        *zap.Logger
}

// NewConsultationHandler creates a new consultation handler
func NewConsultationHandler(
	graphClient *graph.Client,
	queryBuilder *graph.QueryBuilder,
	redisClient *redis.Client,
	aiServicesURL string,
	logger *zap.Logger,
) *ConsultationHandler {
	return &ConsultationHandler{
		graphClient:   graphClient,
		queryBuilder:  queryBuilder,
		redisClient:   redisClient,
		aiServicesURL: aiServicesURL,
		logger:        logger,
	}
}

// Handle processes a consultation request and returns a synthesized response
func (h *ConsultationHandler) Handle(ctx context.Context, req *graph.ConsultationRequest) (*graph.ConsultationResponse, error) {
	startTime := time.Now()
	h.logger.Debug("Handling consultation request",
		zap.String("user_id", req.UserID),
		zap.String("query", req.Query))

	response := &graph.ConsultationResponse{
		RequestID: uuid.New().String(),
	}

	// Step 1: Check Redis cache for recent similar queries
	cachedBrief, err := h.checkCache(ctx, req)
	if err == nil && cachedBrief != "" {
		response.SynthesizedBrief = cachedBrief
		h.logger.Debug("Cache hit for consultation", zap.Duration("latency", time.Since(startTime)))
		return response, nil
	}

	// Step 2: Search the knowledge graph for relevant facts
	relevantFacts, err := h.findRelevantFacts(ctx, req)
	if err != nil {
		h.logger.Warn("Failed to find relevant facts", zap.Error(err))
	}
	response.RelevantFacts = relevantFacts

	// Step 3: Get high-priority insights
	if req.IncludeInsights {
		insights, err := h.getRelevantInsights(ctx, req)
		if err != nil {
			h.logger.Warn("Failed to get insights", zap.Error(err))
		}
		response.Insights = insights
	}

	// Step 4: Check for matching patterns (proactive assistance)
	patterns, alerts := h.checkPatterns(ctx, req)
	response.Patterns = patterns
	response.ProactiveAlerts = alerts

	// Step 5: Synthesize a brief using the AI service
	synthesizedBrief, confidence, err := h.synthesizeBrief(ctx, req, response)
	if err != nil {
		h.logger.Warn("Failed to synthesize brief", zap.Error(err))
		// Create a fallback brief from raw facts
		synthesizedBrief = h.createFallbackBrief(response)
		confidence = 0.5
	}
	response.SynthesizedBrief = synthesizedBrief
	response.Confidence = confidence

	// Step 6: Cache the response
	if err := h.cacheResponse(ctx, req, response); err != nil {
		h.logger.Warn("Failed to cache response", zap.Error(err))
	}

	// Step 7: Update activation on accessed nodes
	h.updateAccessedNodes(ctx, response)

	h.logger.Info("Consultation completed",
		zap.String("request_id", response.RequestID),
		zap.Int("facts", len(response.RelevantFacts)),
		zap.Int("insights", len(response.Insights)),
		zap.Int("alerts", len(response.ProactiveAlerts)),
		zap.Float64("confidence", response.Confidence),
		zap.Duration("latency", time.Since(startTime)))

	return response, nil
}

// checkCache checks Redis for a cached response
func (h *ConsultationHandler) checkCache(ctx context.Context, req *graph.ConsultationRequest) (string, error) {
	// Create a cache key from the query
	key := fmt.Sprintf("consultation:%s:%s", req.UserID, hashQuery(req.Query))

	cached, err := h.redisClient.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}

	return cached, nil
}

// hashQuery creates a simple hash of a query for caching
func hashQuery(query string) string {
	// Simple hash - in production, use a proper hash function
	h := 0
	for _, c := range query {
		h = 31*h + int(c)
	}
	return fmt.Sprintf("%x", h&0x7fffffff)
}

// findRelevantFacts searches the knowledge graph for facts relevant to the query
func (h *ConsultationHandler) findRelevantFacts(ctx context.Context, req *graph.ConsultationRequest) ([]graph.Node, error) {
	maxResults := req.MaxResults
	if maxResults == 0 {
		maxResults = 10
	}

	// Search by text
	nodes, err := h.queryBuilder.SearchByText(ctx, req.Query, maxResults)
	if err != nil {
		return nil, err
	}

	// Also get high-activation nodes as they represent core knowledge
	highActivation, err := h.queryBuilder.GetHighActivationNodes(ctx, req.UserID, 0.7, 5)
	if err != nil {
		h.logger.Warn("Failed to get high activation nodes", zap.Error(err))
	} else {
		// Merge, prioritizing text matches
		seen := make(map[string]bool)
		for _, n := range nodes {
			seen[n.UID] = true
		}
		for _, n := range highActivation {
			if !seen[n.UID] {
				nodes = append(nodes, n)
			}
		}
	}

	// Sort by activation (higher first) and recency
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Activation != nodes[j].Activation {
			return nodes[i].Activation > nodes[j].Activation
		}
		return nodes[i].LastAccessed.After(nodes[j].LastAccessed)
	})

	// Limit results
	if len(nodes) > maxResults {
		nodes = nodes[:maxResults]
	}

	return nodes, nil
}

// getRelevantInsights retrieves insights that may be relevant to the query
func (h *ConsultationHandler) getRelevantInsights(ctx context.Context, req *graph.ConsultationRequest) ([]graph.Insight, error) {
	insights, err := h.queryBuilder.GetInsights(ctx, 5)
	if err != nil {
		return nil, err
	}

	// TODO: Filter insights by relevance to the query
	// For now, return all recent insights
	return insights, nil
}

// checkPatterns checks for patterns that might be relevant (proactive assistance)
func (h *ConsultationHandler) checkPatterns(ctx context.Context, req *graph.ConsultationRequest) ([]graph.Pattern, []string) {
	patterns, err := h.queryBuilder.GetPatterns(ctx, 0.7, 5)
	if err != nil {
		h.logger.Warn("Failed to get patterns", zap.Error(err))
		return nil, nil
	}

	var alerts []string
	for _, pattern := range patterns {
		// Check if pattern triggers should fire based on context
		if pattern.ConfidenceScore > 0.8 && pattern.PredictedAction != "" {
			alerts = append(alerts,
				fmt.Sprintf("Based on past behavior: %s", pattern.PredictedAction))
		}
	}

	return patterns, alerts
}

// synthesizeBrief calls the AI service to create a synthesized brief
func (h *ConsultationHandler) synthesizeBrief(ctx context.Context, req *graph.ConsultationRequest, data *graph.ConsultationResponse) (string, float64, error) {
	type SynthesisRequest struct {
		Query           string          `json:"query"`
		Context         string          `json:"context,omitempty"`
		Facts           []graph.Node    `json:"facts"`
		Insights        []graph.Insight `json:"insights"`
		ProactiveAlerts []string        `json:"proactive_alerts"`
	}

	synthesisReq := SynthesisRequest{
		Query:           req.Query,
		Context:         req.Context,
		Facts:           data.RelevantFacts,
		Insights:        data.Insights,
		ProactiveAlerts: data.ProactiveAlerts,
	}

	jsonData, err := json.Marshal(synthesisReq)
	if err != nil {
		return "", 0, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		h.aiServicesURL+"/synthesize",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return "", 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("synthesis service returned status %d", resp.StatusCode)
	}

	var result struct {
		Brief      string  `json:"brief"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", 0, err
	}

	return result.Brief, result.Confidence, nil
}

// createFallbackBrief creates a simple brief from raw facts when AI synthesis fails
func (h *ConsultationHandler) createFallbackBrief(data *graph.ConsultationResponse) string {
	if len(data.RelevantFacts) == 0 {
		return "I don't have enough information to answer this question."
	}

	brief := "Based on what I know:\n"
	for i, fact := range data.RelevantFacts {
		if i >= 3 {
			brief += fmt.Sprintf("... and %d more related facts.", len(data.RelevantFacts)-3)
			break
		}
		brief += fmt.Sprintf("- %s: %s\n", fact.Name, fact.Description)
	}

	if len(data.ProactiveAlerts) > 0 {
		brief += "\nNote: " + data.ProactiveAlerts[0]
	}

	return brief
}

// cacheResponse caches the synthesized response in Redis
func (h *ConsultationHandler) cacheResponse(ctx context.Context, req *graph.ConsultationRequest, resp *graph.ConsultationResponse) error {
	key := fmt.Sprintf("consultation:%s:%s", req.UserID, hashQuery(req.Query))

	// Cache for 5 minutes
	return h.redisClient.Set(ctx, key, resp.SynthesizedBrief, 5*time.Minute).Err()
}

// updateAccessedNodes boosts activation for all accessed nodes
func (h *ConsultationHandler) updateAccessedNodes(ctx context.Context, resp *graph.ConsultationResponse) {
	config := graph.DefaultActivationConfig()

	for _, node := range resp.RelevantFacts {
		if err := h.graphClient.IncrementAccessCount(ctx, node.UID, config); err != nil {
			h.logger.Warn("Failed to update node activation",
				zap.String("uid", node.UID),
				zap.Error(err))
		}
	}
}
