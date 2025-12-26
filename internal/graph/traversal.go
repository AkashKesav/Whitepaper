// Package graph provides advanced node traversal algorithms for the Knowledge Graph.
package graph

import (
	"context"
	"encoding/json"
	"fmt"
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
	Namespace     string  // Limit to namespace (empty = all)
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
		DecayFactor:   0.5,
		MaxHops:       3,
		MinActivation: 0.1,
		MaxResults:    50,
	}
}

// SpreadActivation performs activation-based node traversal.
// Starting from a seed node, it spreads activation to connected neighbors
// with exponential decay based on distance.
func (c *Client) SpreadActivation(ctx context.Context, opts SpreadActivationOpts) ([]ActivatedNode, error) {
	if opts.StartUID == "" {
		return nil, fmt.Errorf("StartUID is required")
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

	// Track visited nodes and their activation levels
	visited := make(map[string]*ActivatedNode)

	// BFS queue: (uid, current_activation, hop_count)
	type queueItem struct {
		uid        string
		activation float64
		hops       int
	}
	queue := []queueItem{{opts.StartUID, 1.0, 0}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

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

		// Find neighbors via edges
		neighbors, err := c.getNeighborUIDs(ctx, current.uid)
		if err != nil {
			c.logger.Warn("Failed to get neighbors",
				zap.String("uid", current.uid),
				zap.Error(err))
			continue
		}

		// Add neighbors to queue with decayed activation
		nextActivation := current.activation * opts.DecayFactor
		for _, neighborUID := range neighbors {
			if _, visited := visited[neighborUID]; !visited {
				queue = append(queue, queueItem{
					uid:        neighborUID,
					activation: nextActivation,
					hops:       current.hops + 1,
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

// getNeighborUIDs finds all connected nodes via edges
func (c *Client) getNeighborUIDs(ctx context.Context, uid string) ([]string, error) {
	query := fmt.Sprintf(`query Neighbors {
		node(func: uid(%s)) {
			related_to { uid }
			has_attribute { uid }
			produced_by { uid }
			group_has_member { uid }
		}
	}`, uid)

	resp, err := c.Query(ctx, query, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Node []struct {
			RelatedTo []struct {
				UID string `json:"uid"`
			} `json:"related_to"`
			HasAttribute []struct {
				UID string `json:"uid"`
			} `json:"has_attribute"`
			ProducedBy []struct {
				UID string `json:"uid"`
			} `json:"produced_by"`
			GroupHasMember []struct {
				UID string `json:"uid"`
			} `json:"group_has_member"`
		} `json:"node"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	// Collect all neighbor UIDs
	neighbors := make([]string, 0)
	if len(result.Node) > 0 {
		for _, r := range result.Node[0].RelatedTo {
			neighbors = append(neighbors, r.UID)
		}
		for _, r := range result.Node[0].HasAttribute {
			neighbors = append(neighbors, r.UID)
		}
		for _, r := range result.Node[0].ProducedBy {
			neighbors = append(neighbors, r.UID)
		}
		for _, r := range result.Node[0].GroupHasMember {
			neighbors = append(neighbors, r.UID)
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
	seedNode, err := c.FindNodeByName(ctx, opts.EntityName, NodeTypeEntity)
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
func (c *Client) GetSampleNodes(ctx context.Context, limit int) ([]Node, error) {
	if limit <= 0 {
		limit = 50
	}

	query := fmt.Sprintf(`query SampleNodes {
		nodes(func: has(name), first: %d) {
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

	resp, err := c.Query(ctx, query, nil)
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
