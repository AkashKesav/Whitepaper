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
	"strings"
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
// SIMPLIFIED: Directly queries user's knowledge and formats it without external AI call
func (h *ConsultationHandler) Handle(ctx context.Context, req *graph.ConsultationRequest) (*graph.ConsultationResponse, error) {
	startTime := time.Now()
	h.logger.Info("=== CONSULTATION START ===",
		zap.String("user_id", req.UserID),
		zap.String("query", req.Query))

	response := &graph.ConsultationResponse{
		RequestID: uuid.New().String(),
	}

	// STEP 1: Get facts matching the query terms
	facts, err := h.getUserKnowledge(ctx, req.UserID, req.Query)
	if err != nil {
		h.logger.Warn("Failed to get user knowledge", zap.Error(err))
	}
	response.RelevantFacts = facts

	h.logger.Info("Retrieved user knowledge",
		zap.Int("facts_count", len(facts)))

	// STEP 2: Format facts directly into a brief (no external AI call)
	var brief strings.Builder
	if len(facts) > 0 {
		brief.WriteString("Based on what you've told me:\n")
		for i, fact := range facts {
			if i >= 10 {
				brief.WriteString(fmt.Sprintf("... and %d more items.\n", len(facts)-10))
				break
			}
			nodeType := fact.GetType()
			brief.WriteString(fmt.Sprintf("- %s", fact.Name))
			if fact.Description != "" {
				brief.WriteString(fmt.Sprintf(": %s", fact.Description))
			}
			if len(fact.Tags) > 0 {
				brief.WriteString(fmt.Sprintf(" [%s]", strings.Join(fact.Tags, ", ")))
			}
			brief.WriteString(fmt.Sprintf(" (%s)\n", nodeType))
		}
		response.Confidence = 0.9
	} else {
		brief.WriteString("I don't have any stored information about you yet.")
		response.Confidence = 0.3
	}

	response.SynthesizedBrief = brief.String()

	h.logger.Info("=== CONSULTATION COMPLETE ===",
		zap.String("brief", response.SynthesizedBrief),
		zap.Int("facts", len(facts)),
		zap.Duration("latency", time.Since(startTime)))

	return response, nil
}

// getUserKnowledge retrieves ALL stored facts and lets the LLM find relevant ones
func (h *ConsultationHandler) getUserKnowledge(ctx context.Context, userID string, queryText string) ([]graph.Node, error) {
	h.logger.Info("Fetching all knowledge for context", zap.String("query", queryText))

	// Simple approach: fetch ALL non-User nodes, ordered by most recent first
	query := `{
		all_facts(func: has(name), first: 50, orderdesc: created_at) @filter(NOT type(User)) {
			uid
			dgraph.type
			name
			description
			tags
		}
	}`

	resp, err := h.graphClient.Query(ctx, query, nil)
	if err != nil {
		h.logger.Error("Query failed", zap.Error(err))
		return nil, err
	}

	h.logger.Info("Raw DGraph response", zap.String("json", string(resp)))

	var result struct {
		AllFacts []graph.Node `json:"all_facts"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		h.logger.Error("Failed to unmarshal nodes", zap.Error(err))
		return nil, err
	}

	h.logger.Info("Fetched all knowledge", zap.Int("count", len(result.AllFacts)))
	for i, node := range result.AllFacts {
		h.logger.Info("Node found",
			zap.Int("index", i),
			zap.String("name", node.Name),
			zap.String("description", node.Description))
	}

	return result.AllFacts, nil
}

// textSearchFallback provides fallback text-based search if semantic search fails
func (h *ConsultationHandler) textSearchFallback(ctx context.Context, queryText string) ([]graph.Node, error) {
	cleanedQuery := h.cleanQuery(queryText)
	h.logger.Info("Fallback to text search", zap.String("query", cleanedQuery))

	query := fmt.Sprintf(`{
		desc_match(func: anyoftext(description, %q), first: 10) @filter(NOT type(User)) {
			uid
			dgraph.type
			name
			description
			tags
		}
	}`, cleanedQuery)

	resp, err := h.graphClient.Query(ctx, query, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		DescMatch []graph.Node `json:"desc_match"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.DescMatch, nil
}

// getString safely extracts a string from a map
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// cleanQuery removes common stop words to focus search on keywords
func (h *ConsultationHandler) cleanQuery(query string) string {
	stopWords := map[string]bool{
		"what": true, "is": true, "my": true, "the": true, "a": true, "an": true,
		"of": true, "for": true, "in": true, "on": true, "at": true, "to": true,
		"do": true, "does": true, "did": true, "can": true, "could": true,
		"who": true, "where": true, "when": true, "why": true, "how": true,
		"tell": true, "me": true, "about": true, "know": true,
	}

	words := strings.Fields(strings.ToLower(query))
	var keywords []string
	for _, w := range words {
		// Strip punctuation
		w = strings.Trim(w, "?!.,\"'")
		if !stopWords[w] && len(w) > 1 {
			keywords = append(keywords, w)
		}
	}
	return strings.Join(keywords, " ")
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

	h.logger.Debug("Finding relevant facts",
		zap.String("user_id", req.UserID),
		zap.String("query", req.Query),
		zap.Int("max_results", maxResults))

	// Search by text
	nodes, err := h.queryBuilder.SearchByText(ctx, req.Query, maxResults)
	if err != nil {
		h.logger.Warn("Text search failed", zap.Error(err))
	}
	h.logger.Debug("Text search results", zap.Int("count", len(nodes)))

	// CRITICAL: Also get nodes connected to the User via relationship edges
	// This is essential for queries like "Who is my manager?" where text search won't match "Bob"
	userRelated, err := h.queryBuilder.GetUserRelatedNodes(ctx, req.UserID, maxResults)
	if err != nil {
		h.logger.Warn("Failed to get user related nodes", zap.Error(err), zap.String("user_id", req.UserID))
	} else {
		h.logger.Debug("User related nodes", zap.Int("count", len(userRelated)))
		// Merge user-related nodes with text search results
		seen := make(map[string]bool)
		for _, n := range nodes {
			seen[n.UID] = true
		}
		for _, n := range userRelated {
			if !seen[n.UID] {
				nodes = append(nodes, n)
				seen[n.UID] = true
			}
		}
	}

	// Also get high-activation nodes as they represent core knowledge
	highActivation, err := h.queryBuilder.GetHighActivationNodes(ctx, req.UserID, 0.3, 10)
	if err != nil {
		h.logger.Warn("Failed to get high activation nodes", zap.Error(err))
	} else {
		h.logger.Debug("High activation nodes", zap.Int("count", len(highActivation)))
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

	// CRITICAL FALLBACK: If no nodes found, get ALL nodes with names (most reliable)
	if len(nodes) == 0 {
		h.logger.Debug("No nodes found via specific queries, using GetAllNodes fallback")
		allNodes, err := h.queryBuilder.GetAllNodes(ctx, maxResults)
		if err != nil {
			h.logger.Warn("Failed to get all nodes", zap.Error(err))
		} else {
			// Filter out User nodes, keep only Entity/Fact/Event nodes
			for _, n := range allNodes {
				if n.GetType() != graph.NodeTypeUser && n.Name != "" {
					nodes = append(nodes, n)
				}
			}
			h.logger.Debug("Fallback nodes", zap.Int("count", len(nodes)))
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
		brief += fmt.Sprintf("- %s: %s", fact.Name, fact.Description)
		if len(fact.Tags) > 0 {
			brief += fmt.Sprintf(" [Tags: %v]", fact.Tags)
		}
		brief += "\n"
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
