<<<<<<< HEAD
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

	"github.com/reflective-memory-kernel/internal/ai/local"
	"github.com/reflective-memory-kernel/internal/graph"
)

// ConsultationHandler handles consultation requests from the Front-End Agent
type ConsultationHandler struct {
	graphClient   *graph.Client
	queryBuilder  *graph.QueryBuilder
	redisClient   *redis.Client
	aiServicesURL string
	logger        *zap.Logger

	// Hybrid RAG components
	embedder    local.LocalEmbedder
	vectorIndex *VectorIndex
}

// isUUIDLike checks if a string looks like a UUID (8-4-4-4-12 pattern)
func isUUIDLike(s string) bool {
	// Quick length check - UUIDs are 36 chars with dashes or 32 without
	if len(s) == 36 {
		_, err := uuid.Parse(s)
		return err == nil
	}
	return false
}

// NewConsultationHandler creates a new consultation handler
func NewConsultationHandler(
	graphClient *graph.Client,
	queryBuilder *graph.QueryBuilder,
	redisClient *redis.Client,
	vectorIndex *VectorIndex,
	embedder local.LocalEmbedder,
	aiServicesURL string,
	logger *zap.Logger,
) *ConsultationHandler {
	return &ConsultationHandler{
		graphClient:   graphClient,
		queryBuilder:  queryBuilder,
		redisClient:   redisClient,
		aiServicesURL: aiServicesURL,
		logger:        logger,
		embedder:      embedder,
		vectorIndex:   vectorIndex,
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

	// Step 0: Determine Namespace
	namespace := req.Namespace
	if namespace == "" {
		namespace = fmt.Sprintf("user_%s", req.UserID)
	}

	// PERMISSION CHECK: For group namespaces, verify user is a member
	if strings.HasPrefix(namespace, "group_") {
		isMember, err := h.graphClient.IsWorkspaceMember(ctx, namespace, req.UserID)
		if err != nil {
			h.logger.Error("Failed to check workspace membership", zap.Error(err))
			return nil, fmt.Errorf("permission check failed: %w", err)
		}
		if !isMember {
			h.logger.Warn("Access denied: user is not a workspace member",
				zap.String("user", req.UserID),
				zap.String("workspace", namespace))
			return nil, fmt.Errorf("access denied: not a member of workspace %s", namespace)
		}
		h.logger.Debug("Workspace access verified", zap.String("namespace", namespace))
	}

	// STEP 0.5: Check Speculative Cache (Time Travel)
	var facts []graph.Node
	var err error

	cachedFacts, cacheErr := h.checkSpeculationCache(ctx, req.UserID, req.Query)
	if cacheErr == nil && cachedFacts != nil {
		h.logger.Info("Hit speculative cache (Time Travel successful)", zap.Int("facts", len(cachedFacts)))
		facts = cachedFacts
	} else {
		// STEP 1: Get facts matching the query terms (Cache Miss)
		facts, err = h.getUserKnowledge(ctx, namespace, req.Query)
		if err != nil {
			h.logger.Warn("Failed to get user knowledge", zap.Error(err))
		}
	}
	response.RelevantFacts = facts

	h.logger.Info("Retrieved user knowledge",
		zap.String("namespace", namespace),
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

// getUserKnowledge retrieves stored facts using Hybrid RAG approach:
// 1. Vector search for semantically similar nodes (NEW - Hybrid RAG)
// 2. High activation nodes (frequently accessed)
// 3. Recent nodes (newly added)
// This ensures semantic relevance, importance, AND freshness are all considered
func (h *ConsultationHandler) getUserKnowledge(ctx context.Context, namespace string, queryText string) ([]graph.Node, error) {
	h.logger.Info("Fetching knowledge with Hybrid RAG approach", zap.String("query", queryText))

	seen := make(map[string]bool)
	var merged []graph.Node

	// Helper to check if node should be included
	isValidNode := func(node graph.Node) bool {
		if node.Name == "" {
			return false
		}
		// Skip User nodes
		if node.Name == "User" {
			return false
		}
		// Skip Conversation_ nodes (these are conversation metadata, not facts)
		if len(node.Name) > 13 && node.Name[:13] == "Conversation_" {
			return false
		}
		// Skip user_xxx IDs (user identifiers, not knowledge)
		if len(node.Name) > 5 && node.Name[:5] == "user_" {
			return false
		}
		// Skip UUID-like names (8-4-4-4-12 pattern or just long hex strings)
		if isUUIDLike(node.Name) {
			return false
		}
		return true
	}

	// STEP 1: Vector search for semantically similar nodes (Hybrid RAG)
	if h.embedder != nil && h.vectorIndex != nil {
		queryVec, err := h.embedder.Embed(queryText)
		if err != nil {
			h.logger.Warn("Failed to embed query for vector search", zap.Error(err))
		} else if len(queryVec) > 0 {
			uids, scores, err := h.vectorIndex.Search(ctx, namespace, queryVec, 20)
			if err != nil {
				h.logger.Warn("Vector search failed", zap.Error(err))
			} else if len(uids) > 0 {
				// Fetch full node data for vector search results
				vectorNodes, err := h.graphClient.GetNodesByUIDs(ctx, uids)
				if err != nil {
					h.logger.Warn("Failed to fetch vector search results", zap.Error(err))
				} else {
					h.logger.Info("Vector search found candidates",
						zap.Int("count", len(vectorNodes)),
						zap.Float32("top_score", scores[0]))

					// Add vector results first (highest priority - semantic match)
					for _, node := range vectorNodes {
						if !seen[node.UID] && isValidNode(node) {
							seen[node.UID] = true
							merged = append(merged, node)
						}
					}
				}
			}
		}
	}

	// STEP 2: Get nodes by activation and recency (existing logic)
	query := `query HybridKnowledge($namespace: string) {
		by_activation(func: has(name), first: 50, orderdesc: activation) @filter(eq(namespace, $namespace)) {
			uid
			dgraph.type
			name
			description
			tags
			activation
			created_at
		}
		by_recency(func: has(name), first: 50, orderdesc: created_at) @filter(eq(namespace, $namespace)) {
			uid
			dgraph.type
			name
			description
			tags
			activation
			created_at
		}
	}`

	resp, err := h.graphClient.Query(ctx, query, map[string]string{"$namespace": namespace})
	if err != nil {
		h.logger.Error("Query failed", zap.Error(err))
		return merged, err // Return vector results if we have them
	}

	var result struct {
		ByActivation []graph.Node `json:"by_activation"`
		ByRecency    []graph.Node `json:"by_recency"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		h.logger.Error("Failed to unmarshal nodes", zap.Error(err))
		return merged, err
	}

	// Add high-activation nodes (after vector results)
	for _, node := range result.ByActivation {
		if !seen[node.UID] && isValidNode(node) {
			seen[node.UID] = true
			merged = append(merged, node)
		}
	}

	// Add recent nodes that weren't already included
	for _, node := range result.ByRecency {
		if !seen[node.UID] && isValidNode(node) {
			seen[node.UID] = true
			merged = append(merged, node)
		}
	}

	vectorCount := 0
	if h.embedder != nil && h.vectorIndex != nil {
		for _, node := range merged {
			if seen[node.UID] {
				vectorCount++
				break // Count once for logging if vector was used
			}
		}
	}

	h.logger.Info("Fetched Hybrid RAG knowledge",
		zap.Int("by_activation", len(result.ByActivation)),
		zap.Int("by_recency", len(result.ByRecency)),
		zap.Int("merged_filtered", len(merged)),
		zap.Bool("vector_search_used", h.embedder != nil && h.vectorIndex != nil))

	return merged, nil
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
	namespace := req.Namespace
	if namespace == "" {
		namespace = fmt.Sprintf("user_%s", req.UserID)
	}
	nodes, err := h.queryBuilder.SearchByText(ctx, namespace, req.Query, maxResults)
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
	highActivation, err := h.queryBuilder.GetHighActivationNodes(ctx, namespace, 0.3, 10)
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
		allNodes, err := h.queryBuilder.GetAllNodes(ctx, namespace, maxResults)
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
	// Get recent insights
	namespace := req.Namespace
	if namespace == "" {
		namespace = fmt.Sprintf("user_%s", req.UserID)
	}
	insights, err := h.queryBuilder.GetInsights(ctx, namespace, 5)
	if err != nil {
		return nil, err
	}

	// TODO: Filter insights by relevance to the query
	// For now, return all recent insights
	return insights, nil
}

// checkPatterns checks for patterns that might be relevant (proactive assistance)
func (h *ConsultationHandler) checkPatterns(ctx context.Context, req *graph.ConsultationRequest) ([]graph.Pattern, []string) {
	namespace := req.Namespace
	if namespace == "" {
		namespace = fmt.Sprintf("user_%s", req.UserID)
	}
	patterns, err := h.queryBuilder.GetPatterns(ctx, namespace, 0.7, 5)
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

// isQueryRelevant checks if a node is semantically relevant to the query
func isQueryRelevant(nodeName string, query string) bool {
	queryLower := strings.ToLower(query)
	nameLower := strings.ToLower(nodeName)

	// Direct substring match
	if strings.Contains(queryLower, nameLower) {
		return true
	}

	// Word-level match (handles "basketball" in "I love basketball")
	queryWords := strings.Fields(queryLower)
	nameWords := strings.Fields(nameLower)

	for _, nameWord := range nameWords {
		for _, queryWord := range queryWords {
			if nameWord == queryWord {
				return true
			}
		}
	}

	return false
}

// updateAccessedNodes boosts activation only for query-relevant nodes
func (h *ConsultationHandler) updateAccessedNodes(ctx context.Context, query string, resp *graph.ConsultationResponse) {
	config := graph.DefaultActivationConfig()

	for _, node := range resp.RelevantFacts {
		// ONLY boost if node is relevant to the query
		if !isQueryRelevant(node.Name, query) {
			continue
		}

		if err := h.graphClient.IncrementAccessCount(ctx, node.UID, config); err != nil {
			h.logger.Warn("Failed to update node activation",
				zap.String("uid", node.UID),
				zap.Error(err))
		} else {
			h.logger.Debug("Boosted relevant node",
				zap.String("name", node.Name),
				zap.String("query", query))
		}
	}
}

// Speculate performs a pre-fetch for a partial query and caches the result
func (h *ConsultationHandler) Speculate(ctx context.Context, req *graph.ConsultationRequest) error {
	if len(req.Query) < 5 {
		// Too short to speculate
		return nil
	}

	h.logger.Debug("Speculating on partial query", zap.String("query", req.Query))

	namespace := req.Namespace
	if namespace == "" {
		namespace = fmt.Sprintf("user_%s", req.UserID)
	}

	// Just perform text search for speed (Hot Path)
	// We don't do full hybrid search or getting user-related nodes to save resources
	facts, err := h.queryBuilder.SearchByText(ctx, namespace, req.Query, 5)
	if err != nil {
		return err
	}

	if len(facts) > 0 {
		h.logger.Debug("Speculation found facts", zap.Int("count", len(facts)))
		return h.saveSpeculation(ctx, req.UserID, req.Query, facts)
	}

	return nil
}

// saveSpeculation saves the speculation result to Redis
func (h *ConsultationHandler) saveSpeculation(ctx context.Context, userID, query string, facts []graph.Node) error {
	key := fmt.Sprintf("speculation:%s:latest", userID)

	data := struct {
		Query string       `json:"query"`
		Facts []graph.Node `json:"facts"`
	}{
		Query: query,
		Facts: facts,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Cache for 10 seconds (typing context is fleeing)
	return h.redisClient.Set(ctx, key, jsonData, 10*time.Second).Err()
}

// checkSpeculationCache checks if we have a valid speculation for the current query
func (h *ConsultationHandler) checkSpeculationCache(ctx context.Context, userID, currentQuery string) ([]graph.Node, error) {
	key := fmt.Sprintf("speculation:%s:latest", userID)

	val, err := h.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Miss
		}
		return nil, err
	}

	var cached struct {
		Query string       `json:"query"`
		Facts []graph.Node `json:"facts"`
	}
	if err := json.Unmarshal([]byte(val), &cached); err != nil {
		return nil, err
	}

	// Check if cached query is relevant to current query
	// Ideally, cached query ("Hello w") should be a prefix of current ("Hello world")
	// Or vice-versa if user deleted chars, but usually we care about forward typing.
	// We also accept if they are very close.

	// transform to lower
	curr := strings.ToLower(currentQuery)
	prev := strings.ToLower(cached.Query)

	if strings.HasPrefix(curr, prev) {
		// Hit!
		return cached.Facts, nil
	}

	return nil, nil
}
=======
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
	"github.com/reflective-memory-kernel/internal/memory"
	"github.com/reflective-memory-kernel/internal/policy"
)

// ConsultationHandler handles consultation requests from the Front-End Agent
type ConsultationHandler struct {
	graphClient   *graph.Client
	queryBuilder  *graph.QueryBuilder
	redisClient   *redis.Client
	hotCache      *memory.HotCache
	policyEngine  policy.Engine
	aiServicesURL string
	logger        *zap.Logger
}

// isUUIDLike checks if a string looks like a UUID (8-4-4-4-12 pattern)
func isUUIDLike(s string) bool {
	// Quick length check - UUIDs are 36 chars with dashes or 32 without
	if len(s) == 36 {
		_, err := uuid.Parse(s)
		return err == nil
	}
	return false
}

// NewConsultationHandler creates a new consultation handler
func NewConsultationHandler(
	graphClient *graph.Client,
	queryBuilder *graph.QueryBuilder,
	redisClient *redis.Client,
	hotCache *memory.HotCache,
	policyEngine policy.Engine,
	aiServicesURL string,
	logger *zap.Logger,
) *ConsultationHandler {
	return &ConsultationHandler{
		graphClient:   graphClient,
		queryBuilder:  queryBuilder,
		redisClient:   redisClient,
		hotCache:      hotCache,
		policyEngine:  policyEngine,
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

	// STEP 1.5: Check Hot Cache for recent context (Instant Recall)
	if h.hotCache != nil {
		hotMsgs, err := h.hotCache.Search(req.UserID, req.Query, 3, 0.5)
		if err == nil && len(hotMsgs) > 0 {
			h.logger.Info("Hot cache hit", zap.Int("count", len(hotMsgs)))
			for _, result := range hotMsgs {
				// Add as high-confidence fact
				facts = append(facts, graph.Node{
					UID:         "hot-cache-item", // dummy
					Name:        "Recent Conversation",
					Description: fmt.Sprintf("User: %s\nAI: %s", result.Message.Query, result.Message.Response),
					Tags:        []string{"recent_memory", "hot_cache"},
					Activation:  1.0,
				})
			}
		}
	}

	// STEP 1.6: Apply Policy Filtering (ABAC/RBAC)
	if h.policyEngine != nil {
		// Construct user context (fetch groups if needed, for now assume basic user context)
		// In production, fetch this from Identity Provider or Cache
		userCtx := policy.UserContext{
			UserID:    req.UserID,
			Groups:    []string{}, // TODO: Fetch user groups
			Clearance: 1,          // Default to Internal clearance
		}

		allowedFacts := make([]graph.Node, 0, len(facts))
		for _, fact := range facts {
			// Skip check for dummy hot-cache items or perform specific check
			if fact.UID == "hot-cache-item" {
				allowedFacts = append(allowedFacts, fact)
				continue
			}

			effect, err := h.policyEngine.Evaluate(ctx, userCtx, &fact, policy.ActionRead)
			if err == nil && effect == policy.EffectAllow {
				allowedFacts = append(allowedFacts, fact)
			} else {
				h.logger.Debug("Policy denied access to fact",
					zap.String("uid", fact.UID),
					zap.String("user", req.UserID))
			}
		}
		facts = allowedFacts
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

// getUserKnowledge retrieves stored facts using hybrid approach:
// - High activation nodes (frequently accessed)
// - Recent nodes (newly added within last hour)
// This ensures both relevant AND fresh memories are considered
func (h *ConsultationHandler) getUserKnowledge(ctx context.Context, userID string, queryText string) ([]graph.Node, error) {
	h.logger.Info("Fetching knowledge with hybrid approach", zap.String("query", queryText))

	// Hybrid query: top 50 by activation + top 50 by recency
	namespace := fmt.Sprintf("user_%s", userID)

	query := `query HybridKnowledge($namespace: string) {
		by_activation(func: has(name), first: 50, orderdesc: activation) @filter(eq(namespace, $namespace)) {
			uid
			dgraph.type
			name
			description
			tags
			activation
			created_at
		}
		by_recency(func: has(name), first: 50, orderdesc: created_at) @filter(eq(namespace, $namespace)) {
			uid
			dgraph.type
			name
			description
			tags
			activation
			created_at
		}
	}`

	resp, err := h.graphClient.Query(ctx, query, map[string]string{"$namespace": namespace})
	if err != nil {
		h.logger.Error("Query failed", zap.Error(err))
		return nil, err
	}

	var result struct {
		ByActivation []graph.Node `json:"by_activation"`
		ByRecency    []graph.Node `json:"by_recency"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		h.logger.Error("Failed to unmarshal nodes", zap.Error(err))
		return nil, err
	}

	// Merge and deduplicate, filter out User nodes and Conversation nodes
	seen := make(map[string]bool)
	var merged []graph.Node

	// Helper to check if node should be included
	isValidNode := func(node graph.Node) bool {
		if node.Name == "" {
			return false
		}
		// Skip User nodes
		if node.Name == "User" {
			return false
		}
		// Skip Conversation_ nodes (these are conversation metadata, not facts)
		if len(node.Name) > 13 && node.Name[:13] == "Conversation_" {
			return false
		}
		// Skip user_xxx IDs (user identifiers, not knowledge)
		if len(node.Name) > 5 && node.Name[:5] == "user_" {
			return false
		}
		// Skip UUID-like names (8-4-4-4-12 pattern or just long hex strings)
		if isUUIDLike(node.Name) {
			return false
		}
		return true
	}

	// Add high-activation first (priority)
	for _, node := range result.ByActivation {
		if !seen[node.UID] && isValidNode(node) {
			seen[node.UID] = true
			merged = append(merged, node)
		}
	}

	// Add recent nodes that weren't already included
	for _, node := range result.ByRecency {
		if !seen[node.UID] && isValidNode(node) {
			seen[node.UID] = true
			merged = append(merged, node)
		}
	}

	h.logger.Info("Fetched hybrid knowledge",
		zap.Int("by_activation", len(result.ByActivation)),
		zap.Int("by_recency", len(result.ByRecency)),
		zap.Int("merged_filtered", len(merged)))

	return merged, nil
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
	namespace := fmt.Sprintf("user_%s", req.UserID)
	nodes, err := h.queryBuilder.SearchByText(ctx, namespace, req.Query, maxResults)
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
	highActivation, err := h.queryBuilder.GetHighActivationNodes(ctx, namespace, 0.3, 10)
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
		allNodes, err := h.queryBuilder.GetAllNodes(ctx, namespace, maxResults)
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
	// Get recent insights
	namespace := fmt.Sprintf("user_%s", req.UserID)
	insights, err := h.queryBuilder.GetInsights(ctx, namespace, 5)
	if err != nil {
		return nil, err
	}

	// TODO: Filter insights by relevance to the query
	// For now, return all recent insights
	return insights, nil
}

// checkPatterns checks for patterns that might be relevant (proactive assistance)
func (h *ConsultationHandler) checkPatterns(ctx context.Context, req *graph.ConsultationRequest) ([]graph.Pattern, []string) {
	namespace := fmt.Sprintf("user_%s", req.UserID)
	patterns, err := h.queryBuilder.GetPatterns(ctx, namespace, 0.7, 5)
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

// isQueryRelevant checks if a node is semantically relevant to the query
func isQueryRelevant(nodeName string, query string) bool {
	queryLower := strings.ToLower(query)
	nameLower := strings.ToLower(nodeName)

	// Direct substring match
	if strings.Contains(queryLower, nameLower) {
		return true
	}

	// Word-level match (handles "basketball" in "I love basketball")
	queryWords := strings.Fields(queryLower)
	nameWords := strings.Fields(nameLower)

	for _, nameWord := range nameWords {
		for _, queryWord := range queryWords {
			if nameWord == queryWord {
				return true
			}
		}
	}

	return false
}

// updateAccessedNodes boosts activation only for query-relevant nodes
func (h *ConsultationHandler) updateAccessedNodes(ctx context.Context, query string, resp *graph.ConsultationResponse) {
	config := graph.DefaultActivationConfig()

	for _, node := range resp.RelevantFacts {
		// ONLY boost if node is relevant to the query
		if !isQueryRelevant(node.Name, query) {
			continue
		}

		if err := h.graphClient.IncrementAccessCount(ctx, node.UID, config); err != nil {
			h.logger.Warn("Failed to update node activation",
				zap.String("uid", node.UID),
				zap.Error(err))
		} else {
			h.logger.Debug("Boosted relevant node",
				zap.String("name", node.Name),
				zap.String("query", query))
		}
	}
}
>>>>>>> 5f37bd4 (Major update: API timeout fixes, Vector-Native ingestion, Frontend integration)
