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
	"github.com/reflective-memory-kernel/internal/policy"
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

	// Policy Manager
	policyManager *policy.PolicyManager
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
	policyManager *policy.PolicyManager,
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
		policyManager: policyManager,
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
	// STEP 1.5: Policy Enforcement (Filter Facts)
	// Even if we found the facts, we must verify the user is allowed to see them.
	// This enforces ABAC (Clearance) and RBAC (Policies) at the data retrieval layer.
	var allowedFacts []graph.Node
	if h.policyManager != nil {
		// CRITICAL: Load policies from DGraph before evaluation
		// Without this, the engine has no policies to check against!
		if err := h.policyManager.LoadPolicies(ctx, namespace); err != nil {
			h.logger.Warn("Failed to load policies from store", zap.Error(err))
		}

		// Build UserContext (fetch groups, clearance, etc.)
		userCtx, err := h.buildUserContext(ctx, req.UserID)
		if err != nil {
			h.logger.Error("Failed to build user context for policy check", zap.Error(err))
			// Fail safe: if we can't verify context, we assume minimal access (or deny all?)
			// For now, we'll log and continue with minimal context (public only)
			userCtx = policy.UserContext{UserID: req.UserID}
		}

		for _, fact := range facts {
			// Evaluate "READ" action on this resource
			effect, err := h.policyManager.Evaluate(ctx, userCtx, &fact, policy.ActionRead)
			if err != nil {
				h.logger.Warn("Policy evaluation error", zap.Error(err), zap.String("node", fact.UID))
				continue // Skip on error
			}

			if effect == policy.EffectAllow {
				allowedFacts = append(allowedFacts, fact)
			} else {
				h.logger.Info("Data access denied by policy",
					zap.String("user", req.UserID),
					zap.String("node", fact.UID),
					zap.String("type", string(fact.GetType())))
			}
		}
		facts = allowedFacts // Update facts with filtered list
	}

	response.RelevantFacts = facts

	h.logger.Info("Retrieved user knowledge (after policy filter)",
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

	// STEP 3: Async Activation Boost (Active Synthesis) - DISABLED
	// This was causing a feedback loop where ALL retrieved nodes got boosted,
	// even when just viewing the dashboard. Activation should only be boosted
	// when entities are explicitly mentioned (handled in ingestion deduplication).
	//
	// if len(response.RelevantFacts) > 0 {
	// 	factsToBoost := make([]graph.Node, len(response.RelevantFacts))
	// 	copy(factsToBoost, response.RelevantFacts)
	// 	go func(nodes []graph.Node) {
	// 		boostCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// 		defer cancel()
	// 		config := graph.DefaultActivationConfig()
	// 		for _, node := range nodes {
	// 			if err := h.graphClient.IncrementAccessCount(boostCtx, node.UID, config); err != nil {
	// 				h.logger.Debug("Failed to boost activation", zap.String("uid", node.UID), zap.Error(err))
	// 			}
	// 		}
	// 	}(factsToBoost)
	// }

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
		// Skip User and Group nodes by checking dgraph.type (not name!)
		nodeType := node.GetType()
		if nodeType == graph.NodeTypeUser || nodeType == graph.NodeTypeGroup {
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
		// Skip generic "Batch Summary" nodes - return the actual entities
		if node.Name == "Batch Summary" {
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
			uids, scores, payloads, err := h.vectorIndex.Search(ctx, namespace, queryVec, 20)
			if err != nil {
				h.logger.Warn("Vector search failed", zap.Error(err))
			} else if len(uids) > 0 {
				h.logger.Info("Vector search found candidates",
					zap.Int("count", len(uids)),
					zap.Float32("top_score", scores[0]))

				// Process Vector Results (Hybrid)
				var entityUIDs []string

				for i, uid := range uids {
					payload := payloads[i]

					// If this is a chunk with text, create a synthetic node
					if text, ok := payload["text"].(string); ok && text != "" {
						// Create synthetic Fact node from snippet
						snippetNode := graph.Node{
							UID:         uid,
							Name:        "Relevant Excerpt",
							Description: text, // The chunk text is the content
							DType:       []string{string(graph.NodeTypeFact)},
							Activation:  1.0, // High priority from vector match
							Confidence:  float64(scores[i]),
							Tags:        []string{"vector-result", "snippet"},
						}

						// Add metadata if available
						if page, ok := payload["page_number"].(float64); ok {
							snippetNode.Attributes = map[string]string{
								"page": fmt.Sprintf("%.0f", page),
							}
						}

						if !seen[uid] {
							seen[uid] = true
							merged = append(merged, snippetNode)
						}
					} else {
						// It's a graph node (Entity), queue for lookup
						entityUIDs = append(entityUIDs, uid)
					}
				}

				// Fetch full node data for Entity matches
				if len(entityUIDs) > 0 {
					vectorNodes, err := h.graphClient.GetNodesByUIDs(ctx, entityUIDs)
					if err != nil {
						h.logger.Warn("Failed to fetch vector search results", zap.Error(err))
					} else {
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
	}

	// STEP 1.5: SPREADING ACTIVATION (Multi-Hop Expansion)
	// For seed nodes found via semantic search, spread activation to neighbors
	// This enables multi-hop reasoning like "Who is my boss's wife?"
	if len(merged) > 0 && h.graphClient != nil {
		h.logger.Debug("Spreading activation from seed nodes", zap.Int("seeds", len(merged)))

		// Use top 3 seeds to avoid explosion
		seedCount := 3
		if len(merged) < seedCount {
			seedCount = len(merged)
		}

		for i := 0; i < seedCount; i++ {
			seed := merged[i]
			if seed.UID == "" {
				continue
			}

			opts := graph.SpreadActivationOpts{
				StartUID:      seed.UID,
				Namespace:     namespace,
				DecayFactor:   0.6, // Retain 60% per hop
				MaxHops:       2,   // 2 hops for relationship traversal
				MinActivation: 0.2, // Stop when activation < 20%
				MaxResults:    10,
			}

			expanded, err := h.graphClient.SpreadActivation(ctx, opts)
			if err != nil {
				h.logger.Warn("Spreading activation failed", zap.Error(err), zap.String("seed", seed.UID))
				continue
			}

			for _, an := range expanded {
				if !seen[an.Node.UID] && isValidNode(an.Node) {
					seen[an.Node.UID] = true
					// Preserve the computed activation from traversal
					node := an.Node
					node.Activation = an.Activation
					merged = append(merged, node)
				}
			}
		}

		h.logger.Info("Spreading activation complete",
			zap.Int("total_nodes", len(merged)))
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

// buildUserContext requires fetching user details (groups, clearance) from DGraph
func (h *ConsultationHandler) buildUserContext(ctx context.Context, userID string) (policy.UserContext, error) {
	// 1. Get Groups
	graphGroups, err := h.graphClient.ListUserGroups(ctx, userID)
	if err != nil {
		return policy.UserContext{}, fmt.Errorf("failed to list groups: %w", err)
	}
	var groups []string
	for _, g := range graphGroups {
		groups = append(groups, g.Name)
	}

	// 2. Get Clearance (using specific query)
	// Use a lightweight query to get just the clearance level
	query := `query UserClearance($id: string) {
		u(func: eq(username, $id)) {
			clearance
		}
	}`
	resp, err := h.graphClient.Query(ctx, query, map[string]string{"$id": userID})
	if err != nil {
		return policy.UserContext{}, fmt.Errorf("failed to query clearance: %w", err)
	}

	var result struct {
		U []struct {
			Clearance int `json:"clearance"`
		} `json:"u"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return policy.UserContext{}, fmt.Errorf("failed to parse clearance: %w", err)
	}

	clearance := 0
	if len(result.U) > 0 {
		clearance = result.U[0].Clearance
	}

	return policy.UserContext{
		UserID:    userID,
		Groups:    groups,
		Clearance: clearance,
	}, nil
}
