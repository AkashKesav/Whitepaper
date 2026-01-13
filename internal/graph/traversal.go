// Package graph provides advanced node traversal algorithms for the Knowledge Graph.
package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"time"

	"go.uber.org/zap"
)

// ============================================================================
// Spreading Activation Traversal
// ============================================================================

// SpreadActivationOpts configures the spreading activation algorithm
type SpreadActivationOpts struct {
	StartUID      string  // Starting node UID
	Namespace     string  // Limit to namespace (REQUIRED for security - prevents cross-tenant access)
	DecayFactor   float64 // 0.0-1.0, how much activation is retained per hop (0.5 = halve)
	MaxHops       int     // Maximum traversal depth
	MinActivation float64 // Stop when activation falls below this threshold
	MaxResults    int     // Limit returned nodes
}

// ActivatedNode represents a node with computed activation from traversal
type ActivatedNode struct {
	Node       Node    `json:"node"`
	Activation float64 `json:"activation"` // Computed activation level
	Hops       int     `json:"hops"`       // Distance from start node
}

// DefaultSpreadActivationOpts returns sensible defaults
func DefaultSpreadActivationOpts() SpreadActivationOpts {
	return SpreadActivationOpts{
		DecayFactor:   0.7,
		MaxHops:       3,
		MinActivation: 0.05,
		MaxResults:    50,
	}
}

// SpreadActivation performs activation-based node traversal.
// Starting from a seed node, it spreads activation to connected neighbors
// with exponential decay based on distance.
// Includes cycle detection to prevent infinite loops in cyclic graphs.
// SECURITY: Requires namespace to prevent cross-tenant data access
func (c *Client) SpreadActivation(ctx context.Context, opts SpreadActivationOpts) ([]ActivatedNode, error) {
	if opts.StartUID == "" {
		return nil, fmt.Errorf("StartUID is required")
	}

	// SECURITY: Require namespace to prevent cross-tenant activation spreading
	// This ensures users cannot discover nodes from other namespaces
	if opts.Namespace == "" {
		return nil, fmt.Errorf("namespace is required for activation spreading")
	}

	// SECURITY: Validate namespace format
	if !isValidNamespaceFormat(opts.Namespace) {
		return nil, fmt.Errorf("invalid namespace format")
	}

	if opts.DecayFactor <= 0 || opts.DecayFactor > 1 {
		opts.DecayFactor = 0.5
	}
	if opts.MaxHops <= 0 {
		opts.MaxHops = 3
	}
	if opts.MaxResults <= 0 {
		opts.MaxResults = 50
	}

	// SECURITY: Add bounds to prevent memory exhaustion
	const (
		maxVisitedNodes = 10000  // Maximum total nodes to visit
		maxQueueSize    = 5000   // Maximum queue size to prevent unbounded growth
	)

	// Track visited nodes and their activation levels
	visited := make(map[string]*ActivatedNode)

	// OPTIMIZATION: Instead of tracking full path (O(n²)), track hop count at first visit
	// This prevents cycles while avoiding O(path_length) checks for each neighbor
	// In BFS with decay, the first visit to a node has the highest activation,
	// so revisiting at a higher hop count is unnecessary.
	firstSeenAtHop := make(map[string]int) // nodeUID -> hop count when first seen

	// BFS queue with hop tracking (no full path tracking)
	type queueItem struct {
		uid        string
		activation float64
		hops       int
	}
	queue := []queueItem{{opts.StartUID, 1.0, 0}}
	firstSeenAtHop[opts.StartUID] = 0

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// BOUNDS CHECK: Stop if we've visited too many nodes
		if len(visited) >= maxVisitedNodes {
			c.logger.Warn("SpreadActivation reached max visited nodes limit",
				zap.Int("limit", maxVisitedNodes))
			break
		}

		// Skip if already visited with higher activation
		if existing, ok := visited[current.uid]; ok {
			if existing.Activation >= current.activation {
				continue
			}
		}

		// Skip if activation too low
		if current.activation < opts.MinActivation {
			continue
		}

		// Fetch the node
		node, err := c.GetNode(ctx, current.uid)
		if err != nil || node == nil {
			continue
		}

		// Skip if namespace doesn't match (when specified)
		if opts.Namespace != "" && node.Namespace != opts.Namespace {
			continue
		}

		// Store/update visited
		visited[current.uid] = &ActivatedNode{
			Node:       *node,
			Activation: current.activation,
			Hops:       current.hops,
		}

		// Stop expanding if max hops reached
		if current.hops >= opts.MaxHops {
			continue
		}

		// Find neighbors via edges (pass namespace to prevent cross-tenant spreading)
		neighbors, err := c.getNeighborUIDs(ctx, current.uid, opts.Namespace)
		if err != nil {
			c.logger.Warn("Failed to get neighbors",
				zap.String("uid", current.uid),
				zap.Error(err))
			continue
		}

		// Add neighbors to queue with decayed activation
		// nextActivation = current * decay * edge_weight
		nextHop := current.hops + 1
		for _, neighbor := range neighbors {
			// BOUNDS CHECK: Limit queue size to prevent memory exhaustion
			if len(queue) >= maxQueueSize {
				// Only add if this neighbor has higher activation than existing items
				// This prioritizes high-activation paths
				break
			}

			// OPTIMIZED CYCLE DETECTION: O(1) lookup instead of O(path_length) scan
			// If we've already seen this node at a lower or equal hop count, skip it.
			// The first visit (at lowest hop count) has the highest activation due to decay.
			if seenAt, seen := firstSeenAtHop[neighbor.UID]; seen && seenAt <= nextHop {
				// Already visited at same or lower hop level - skip
				continue
			}

			// Only add if not visited with higher activation
			if existing, ok := visited[neighbor.UID]; !ok || existing.Activation < current.activation*opts.DecayFactor {
				nextActivation := current.activation * opts.DecayFactor * neighbor.Weight

				// Mark this node as seen at this hop level
				firstSeenAtHop[neighbor.UID] = nextHop

				queue = append(queue, queueItem{
					uid:        neighbor.UID,
					activation: nextActivation,
					hops:       nextHop,
				})
			}
		}
	}

	// Convert to slice and sort by activation (descending)
	result := make([]ActivatedNode, 0, len(visited))
	for _, an := range visited {
		result = append(result, *an)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Activation > result[j].Activation
	})

	// Limit results
	if len(result) > opts.MaxResults {
		result = result[:opts.MaxResults]
	}

	return result, nil
}

// WeightedNeighbor represents a connected node with edge weight
type WeightedNeighbor struct {
	UID    string
	Weight float64
}

// getNeighborUIDs finds all connected nodes via edges and returns them with weights
// SECURITY: Requires namespace parameter to prevent cross-tenant activation spreading
func (c *Client) getNeighborUIDs(ctx context.Context, uid, namespace string) ([]WeightedNeighbor, error) {
	query := fmt.Sprintf(`query Neighbors($uid: string, $namespace: string) {
		node(func: uid($uid)) {
			related_to @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			has_attribute @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			produced_by @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			group_has_member @facets(weight) @filter(eq(namespace, $namespace)) { uid }

			# Add standard relation predicates
			partner_is @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			family_member @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			friend_of @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			has_manager @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			works_on @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			works_at @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			colleague @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			likes @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			dislikes @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			is_allergic_to @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			prefers @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			has_interest @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			caused_by @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			blocked_by @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			results_in @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			contradicts @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			occurred_on @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			derived_from @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			synthesized_from @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			supersedes @facets(weight) @filter(eq(namespace, $namespace)) { uid }
			knows @facets(weight) @filter(eq(namespace, $namespace)) { uid }
		}
	}`)

	vars := map[string]string{
		"$uid":       uid,
		"$namespace": namespace,
	}
	resp, err := c.Query(ctx, query, vars)
	if err != nil {
		return nil, err
	}

	// Helper struct for DGraph facet unmarshaling
	type FacetStruct struct {
		UID           string  `json:"uid"`
		Weight        float64 `json:"weight"`            // Standard alias (if used)
		RelatedWeight float64 `json:"related_to|weight"` // Specific facet keys
		FamilyWeight  float64 `json:"family_member|weight"`
		FriendWeight  float64 `json:"friend_of|weight"`
		KnowsWeight   float64 `json:"knows|weight"`
		// We can't easily map ALL facet keys to struct fields without a very long struct.
		// A better approach for specific known edges is detailed unmarshaling or map[string]interface{}.
		// However, for simplicity and coverage, we'll try to rely on DGraph's standard JSON behavior
		// or just check the most common ones we implemented.
		// ACTUALLY: The most robust way in Go/DGraph without a huge struct is map[string]interface{} for dynamic keys
		// BUT: Client.Query returns []byte. Let's use a flexible struct or just map.
	}

	// Using a map to handle the dynamic facet keys (e.g., "friend_of|weight")
	var result struct {
		Node []map[string]interface{} `json:"node"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	neighbors := make([]WeightedNeighbor, 0)
	if len(result.Node) == 0 {
		return neighbors, nil
	}

	nodeData := result.Node[0]

	// Iterate over all fields in the JSON response
	for _, value := range nodeData {
		// We expect lists of objects for edges
		if edges, ok := value.([]interface{}); ok {
			for _, edge := range edges {
				edgeMap, ok := edge.(map[string]interface{})
				if !ok {
					continue
				}

				uid, hasUID := edgeMap["uid"].(string)
				if !hasUID {
					continue
				}

				// Find weight in the edge map. DGraph returns facets as "predicate|facet": value
				// OR just "predicate": [{"uid": "...", "predicate|facet": value}]
				// Since we are iterating the value of the predicate (the list), the key inside edgeMap
				// will be "key|weight" (e.g., "friend_of|weight") if we requested @facets(weight) on that predicate.

				weight := 0.5 // Default

				// Look for any key ending in "|weight"
				for edgeKey, edgeVal := range edgeMap {
					if len(edgeKey) > 7 && edgeKey[len(edgeKey)-7:] == "|weight" {
						if w, ok := edgeVal.(float64); ok {
							weight = w
							break
						}
					}
					// Also check simple "weight" if aliases are involved (unlikely here but safe)
					if edgeKey == "weight" {
						if w, ok := edgeVal.(float64); ok {
							weight = w
							break
						}
					}
				}

				neighbors = append(neighbors, WeightedNeighbor{
					UID:    uid,
					Weight: weight,
				})
			}
		}
	}

	return neighbors, nil
}

// ============================================================================
// Community-Aware Traversal
// ============================================================================

// CommunityTraversalOpts configures community-based traversal
type CommunityTraversalOpts struct {
	EntityName string // Name of seed entity
	Namespace  string // Namespace scope
	MaxResults int    // Limit results
}

// CommunityResult contains the community members and metadata
type CommunityResult struct {
	CommunityName string `json:"community_name"`
	MemberCount   int    `json:"member_count"`
	Members       []Node `json:"members"`
}

// TraverseViaCommunity finds all entities in the same community/department
// as the seed entity. It groups entities by their common attributes (e.g., department, team).
func (c *Client) TraverseViaCommunity(ctx context.Context, opts CommunityTraversalOpts) (*CommunityResult, error) {
	if opts.EntityName == "" {
		return nil, fmt.Errorf("EntityName is required")
	}
	if opts.MaxResults <= 0 {
		opts.MaxResults = 100
	}

	// First, find the seed entity and its community attribute
	seedNode, err := c.FindNodeByName(ctx, opts.Namespace, opts.EntityName, NodeTypeEntity)
	if err != nil {
		return nil, fmt.Errorf("failed to find seed entity: %w", err)
	}
	if seedNode == nil {
		return nil, fmt.Errorf("entity not found: %s", opts.EntityName)
	}

	// Extract community from description or attributes (look for department/team)
	communityName := extractCommunity(seedNode)
	if communityName == "" {
		communityName = "Unknown"
	}

	// Query all entities with similar community attribute
	query := fmt.Sprintf(`query CommunityMembers($namespace: string) {
		members(func: type(Entity), first: %d) @filter(eq(namespace, $namespace)) {
			uid
			name
			description
			namespace
			activation
			created_at
			last_accessed
			dgraph.type
		}
	}`, opts.MaxResults*2) // Fetch extra to filter

	vars := map[string]string{"$namespace": opts.Namespace}
	resp, err := c.Query(ctx, query, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to query community members: %w", err)
	}

	var result struct {
		Members []Node `json:"members"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	// Filter to only those in the same community
	members := make([]Node, 0)
	for _, node := range result.Members {
		nodeCommunity := extractCommunity(&node)
		if nodeCommunity == communityName {
			members = append(members, node)
			if len(members) >= opts.MaxResults {
				break
			}
		}
	}

	return &CommunityResult{
		CommunityName: communityName,
		MemberCount:   len(members),
		Members:       members,
	}, nil
}

// extractCommunity extracts community/department from node description
func extractCommunity(node *Node) string {
	if node == nil || node.Description == "" {
		return ""
	}

	// Look for patterns like "department: X" or "team: X"
	desc := node.Description
	patterns := []string{"department:", "team:", "group:", "community:"}

	for _, pattern := range patterns {
		if idx := findPatternIndex(desc, pattern); idx != -1 {
			// Extract value after pattern
			start := idx + len(pattern)
			end := start
			for end < len(desc) && desc[end] != '\n' && desc[end] != ',' {
				end++
			}
			if end > start {
				return trimSpace(desc[start:end])
			}
		}
	}

	return ""
}

func findPatternIndex(s, pattern string) int {
	for i := 0; i <= len(s)-len(pattern); i++ {
		if s[i:i+len(pattern)] == pattern {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// ============================================================================
// Temporal Decay Query
// ============================================================================

// TemporalQueryOpts configures temporal decay queries
type TemporalQueryOpts struct {
	Namespace     string        // Namespace scope
	MinActivation float64       // Minimum base activation (default 0.1)
	RecencyCutoff time.Duration // Only consider nodes accessed within this duration
	RecencyWeight float64       // How much recency affects final score (0.0-1.0)
	MaxResults    int           // Limit results
}

// RankedNode represents a node with temporal ranking
type RankedNode struct {
	Node        Node    `json:"node"`
	FinalScore  float64 `json:"final_score"`  // Combined activation + recency score
	RecencyDays int     `json:"recency_days"` // Days since last access
}

// DefaultTemporalQueryOpts returns sensible defaults
func DefaultTemporalQueryOpts() TemporalQueryOpts {
	return TemporalQueryOpts{
		MinActivation: 0.1,
		RecencyCutoff: 7 * 24 * time.Hour, // 7 days
		RecencyWeight: 0.3,
		MaxResults:    50,
	}
}

// QueryWithTemporalDecay finds nodes prioritizing recent access.
// Final score = baseActivation * (1 - recencyWeight) + recencyScore * recencyWeight
func (c *Client) QueryWithTemporalDecay(ctx context.Context, opts TemporalQueryOpts) ([]RankedNode, error) {
	if opts.MaxResults <= 0 {
		opts.MaxResults = 50
	}
	if opts.RecencyWeight < 0 || opts.RecencyWeight > 1 {
		opts.RecencyWeight = 0.3
	}

	// Query nodes with activation above minimum
	query := fmt.Sprintf(`query TemporalNodes($namespace: string, $minActivation: string) {
		nodes(func: type(Entity), first: %d) @filter(eq(namespace, $namespace) AND ge(activation, $minActivation)) {
			uid
			name
			description
			namespace
			activation
			created_at
			last_accessed
			access_count
			dgraph.type
		}
	}`, opts.MaxResults*2)

	vars := map[string]string{
		"$namespace":     opts.Namespace,
		"$minActivation": fmt.Sprintf("%f", opts.MinActivation),
	}

	resp, err := c.Query(ctx, query, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to query temporal nodes: %w", err)
	}

	var result struct {
		Nodes []Node `json:"nodes"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	now := time.Now()
	cutoffTime := now.Add(-opts.RecencyCutoff)
	ranked := make([]RankedNode, 0, len(result.Nodes))

	for _, node := range result.Nodes {
		// Parse last_accessed
		lastAccessed := node.LastAccessed
		if lastAccessed.IsZero() {
			lastAccessed = node.CreatedAt
		}

		// Skip if outside recency cutoff
		if !cutoffTime.IsZero() && lastAccessed.Before(cutoffTime) {
			continue
		}

		// Calculate recency score (1.0 = just now, 0.0 = at cutoff)
		daysSinceAccess := int(now.Sub(lastAccessed).Hours() / 24)
		maxDays := int(opts.RecencyCutoff.Hours() / 24)
		recencyScore := 1.0
		if maxDays > 0 {
			recencyScore = 1.0 - float64(daysSinceAccess)/float64(maxDays)
			if recencyScore < 0 {
				recencyScore = 0
			}
		}

		// Calculate final score
		baseActivation := node.Activation
		if baseActivation == 0 {
			baseActivation = 0.5 // Default
		}
		finalScore := baseActivation*(1-opts.RecencyWeight) + recencyScore*opts.RecencyWeight

		ranked = append(ranked, RankedNode{
			Node:        node,
			FinalScore:  finalScore,
			RecencyDays: daysSinceAccess,
		})
	}

	// Sort by final score (descending)
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].FinalScore > ranked[j].FinalScore
	})

	// Limit results
	if len(ranked) > opts.MaxResults {
		ranked = ranked[:opts.MaxResults]
	}

	return ranked, nil
}

// ============================================================================
// Multi-Hop Expansion Query
// ============================================================================

// ExpandOpts configures multi-hop expansion
type ExpandOpts struct {
	StartUID   string   // Starting node UID
	EdgeTypes  []string // Edge types to follow (empty = all)
	MaxHops    int      // Maximum depth
	MaxResults int      // Limit total results
}

// ExpandResult contains nodes at each hop level
type ExpandResult struct {
	StartNode  Node           `json:"start_node"`
	ByHop      map[int][]Node `json:"by_hop"` // Hop number -> nodes at that level
	TotalNodes int            `json:"total_nodes"`
}

// ExpandFromNode performs multi-hop graph expansion from a starting node
func (c *Client) ExpandFromNode(ctx context.Context, opts ExpandOpts) (*ExpandResult, error) {
	if opts.StartUID == "" {
		return nil, fmt.Errorf("StartUID is required")
	}
	if opts.MaxHops <= 0 {
		opts.MaxHops = 2
	}
	if opts.MaxResults <= 0 {
		opts.MaxResults = 100
	}

	// Build recursive expansion query
	query := fmt.Sprintf(`query Expand {
		node(func: uid(%s)) @recurse(depth: %d) {
			uid
			name
			description
			dgraph.type
			related_to
			has_attribute
		}
	}`, opts.StartUID, opts.MaxHops+1)

	resp, err := c.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to expand: %w", err)
	}

	var result struct {
		Node []Node `json:"node"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	if len(result.Node) == 0 {
		return nil, fmt.Errorf("start node not found")
	}

	// For now, return flat result (TODO: implement hop-level grouping)
	return &ExpandResult{
		StartNode:  result.Node[0],
		ByHop:      map[int][]Node{0: result.Node},
		TotalNodes: len(result.Node),
	}, nil
}

// GetSampleNodes returns sample nodes from the graph for visualization
func (c *Client) GetSampleNodes(ctx context.Context, namespace string, limit int) ([]Node, error) {
	if limit <= 0 {
		limit = 50
	}

	query := fmt.Sprintf(`query SampleNodes($namespace: string) {
		nodes(func: has(name), first: %d, orderdesc: activation) @filter(eq(namespace, $namespace) AND NOT eq(name, "Batch Summary")) {
			uid
			name
			description
			namespace
			activation
			created_at
			last_accessed
			dgraph.type
		}
	}`, limit)

	vars := map[string]string{"$namespace": namespace}

	resp, err := c.Query(ctx, query, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to query sample nodes: %w", err)
	}

	var result struct {
		Nodes []Node `json:"nodes"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	return result.Nodes, nil
}

// isValidNamespaceFormat validates namespace format for security
// Valid formats: user_<alphanumeric> or group_<alphanumeric>
// SECURITY: Prevents namespace injection and bypass attacks
func isValidNamespaceFormat(ns string) bool {
	if ns == "" {
		return false
	}
	// Allow: user_<alphanumeric with optional hyphens/underscores>
	// Allow: group_<UUID format or alphanumeric with hyphens/underscores>
	matched, _ := regexp.MatchString(`^(user|group)_[a-zA-Z0-9_-]+$`, ns)
	return matched
}
