// Package graph provides query utilities for the Knowledge Graph.
package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// QueryBuilder provides fluent interface for building DGraph queries
type QueryBuilder struct {
	client *Client
}

// NewQueryBuilder creates a new query builder
func NewQueryBuilder(client *Client) *QueryBuilder {
	return &QueryBuilder{client: client}
}

// GetUserGraph retrieves the full knowledge graph for a user
// SECURITY: Requires namespace parameter to prevent cross-tenant data access
func (q *QueryBuilder) GetUserGraph(ctx context.Context, userUID, namespace string, maxDepth int) ([]byte, error) {
	query := fmt.Sprintf(`query UserGraph($uid: string, $namespace: string) {
		user(func: uid($uid)) @filter(eq(namespace, $namespace)) @recurse(depth: %d) {
			uid
			dgraph.type
			name
			description
			activation
			access_count
			last_accessed

			partner_is
			family_member
			friend_of
			has_manager
			works_on
			works_at
			colleague
			likes
			dislikes
			is_allergic_to
			prefers
			has_interest
			caused_by
			blocked_by
			results_in
		}
	}`, maxDepth)

	return q.client.Query(ctx, query, map[string]string{
		"$uid":       userUID,
		"$namespace": namespace,
	})
}

// GetHighActivationNodes retrieves nodes above a certain activation threshold
func (q *QueryBuilder) GetHighActivationNodes(ctx context.Context, namespace string, threshold float64, limit int) ([]Node, error) {
	query := fmt.Sprintf(`query HighActivation($threshold: float, $limit: int, $namespace: string) {
		nodes(func: ge(activation, $threshold), orderdesc: activation, first: $limit) @filter(eq(namespace, $namespace)) {
			uid
			dgraph.type
			name
			description
			activation
			access_count
			last_accessed
			created_at
		}
	}`)

	vars := map[string]string{
		"$threshold": fmt.Sprintf("%f", threshold),
		"$limit":     fmt.Sprintf("%d", limit),
		"$namespace": namespace,
	}

	resp, err := q.client.Query(ctx, query, vars)
	if err != nil {
		return nil, err
	}

	var result struct {
		Nodes []Node `json:"nodes"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Nodes, nil
}

// GetDecayedNodes retrieves nodes that haven't been accessed recently
func (q *QueryBuilder) GetDecayedNodes(ctx context.Context, namespace string, staleThreshold time.Duration) ([]Node, error) {
	cutoffTime := time.Now().Add(-staleThreshold)

	query := `query DecayedNodes($cutoff: string, $namespace: string) {
		nodes(func: lt(last_accessed, $cutoff)) @filter(gt(activation, 0.01) AND eq(namespace, $namespace)) {
			uid
			dgraph.type
			name
			activation
			last_accessed
		}
	}`

	vars := map[string]string{
		"$cutoff":    cutoffTime.Format(time.RFC3339),
		"$namespace": namespace,
	}

	resp, err := q.client.Query(ctx, query, vars)
	if err != nil {
		return nil, err
	}

	var result struct {
		Nodes []Node `json:"nodes"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Nodes, nil
}

// FindPotentialContradictions finds nodes that might have functional edge conflicts
// SECURITY: Requires namespace parameter to prevent cross-tenant data access
// For system-wide operations (e.g., background curation), namespace can be empty
func (q *QueryBuilder) FindPotentialContradictions(ctx context.Context, namespace string, edgeType EdgeType) ([]Contradiction, error) {
	predicateName := edgeTypeToPredicateName(edgeType)

	var query string
	var vars map[string]string

	// SECURITY: Only skip namespace filter for system operations
	if namespace == "" {
		// System-wide operation (no namespace filter)
		// This should ONLY be used by background reflection processes
		query = fmt.Sprintf(`{
			contradictions(func: has(%s)) @normalize {
				uid: uid
				name: name
				namespace: namespace
				edges: count(%s)
			}
		}`, predicateName, predicateName)
		vars = nil
	} else {
		// User-facing operation with namespace filtering
		query = fmt.Sprintf(`query Contradictions($namespace: string) {
			contradictions(func: has(%s)) @filter(eq(namespace, $namespace)) @normalize {
				uid: uid
				name: name
				namespace: namespace
				edges: count(%s)
			}
		}`, predicateName, predicateName)
		vars = map[string]string{"$namespace": namespace}
	}

	resp, err := q.client.Query(ctx, query, vars)
	if err != nil {
		return nil, err
	}

	var result struct {
		Contradictions []struct {
			UID       string `json:"uid"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
			Edges     int    `json:"edges"`
		} `json:"contradictions"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	var contradictions []Contradiction
	for _, c := range result.Contradictions {
		if c.Edges > 1 {
			contradictions = append(contradictions, Contradiction{
				NodeUID1:   c.UID,
				EdgeType:   edgeType,
				DetectedAt: time.Now(),
			})
		}
	}

	return contradictions, nil
}

// FindRelatedNodes finds nodes connected to a given node up to a certain depth
// SECURITY: Requires namespace parameter to prevent cross-tenant data access
func (q *QueryBuilder) FindRelatedNodes(ctx context.Context, nodeUID, namespace string, depth int) ([]byte, error) {
	query := fmt.Sprintf(`query RelatedNodes($uid: string, $namespace: string) {
		related(func: uid($uid)) @filter(eq(namespace, $namespace)) @recurse(depth: %d, loop: false) {
			uid
			dgraph.type
			name
			activation

			~partner_is
			~family_member
			~friend_of
			~has_manager
			~works_on
			~works_at
			~likes
			~dislikes
			~is_allergic_to
			~caused_by
			~blocked_by
		}
	}`, depth)

	return q.client.Query(ctx, query, map[string]string{
		"$uid":       nodeUID,
		"$namespace": namespace,
	})
}

// FindPathBetweenNodes finds the shortest path between two nodes
// SECURITY: Requires namespace parameter to prevent cross-tenant data access
func (q *QueryBuilder) FindPathBetweenNodes(ctx context.Context, fromUID, toUID, namespace string) ([]byte, error) {
	query := `query ShortestPath($from: string, $to: string, $namespace: string) {
		path as shortest(from: uid($from), to: uid($to)) {
			partner_is
			family_member
			friend_of
			has_manager
			works_on
			works_at
			likes
			is_allergic_to
			caused_by
			blocked_by
		}

		path_nodes(func: uid(path)) @filter(eq(namespace, $namespace)) {
			uid
			name
			dgraph.type
			activation
		}
	}`

	return q.client.Query(ctx, query, map[string]string{
		"$from":      fromUID,
		"$to":        toUID,
		"$namespace": namespace,
	})
}

// SearchByText performs full-text search across node names and descriptions, scoped to the namespace
func (q *QueryBuilder) SearchByText(ctx context.Context, namespace string, searchText string, limit int) ([]Node, error) {
	query := `query TextSearch($text: string, $limit: int, $namespace: string) {
		results(func: anyoftext(name, $text), first: $limit) @filter(eq(namespace, $namespace)) {
			uid
			dgraph.type
			name
			description
			activation
			last_accessed
		}
		
		desc_results(func: anyoftext(description, $text), first: $limit) @filter(eq(namespace, $namespace)) {
			uid
			dgraph.type
			name
			description
			activation
			last_accessed
		}

		tag_results(func: anyofterms(tags, $text), first: $limit) @filter(eq(namespace, $namespace)) {
			uid
			dgraph.type
			name
			description
			tags
			activation
			last_accessed
		}
	}`

	vars := map[string]string{
		"$text":      searchText,
		"$limit":     fmt.Sprintf("%d", limit),
		"$namespace": namespace,
	}

	resp, err := q.client.Query(ctx, query, vars)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results     []Node `json:"results"`
		DescResults []Node `json:"desc_results"`
		TagResults  []Node `json:"tag_results"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	// Merge and deduplicate results
	seen := make(map[string]bool)
	var merged []Node
	for _, node := range append(result.Results, append(result.DescResults, result.TagResults...)...) {
		if !seen[node.UID] {
			seen[node.UID] = true
			merged = append(merged, node)
		}
	}

	return merged, nil
}

// GetInsights retrieves all insights for a namespace
func (q *QueryBuilder) GetInsights(ctx context.Context, namespace string, limit int) ([]Insight, error) {
	query := fmt.Sprintf(`query GetInsights($limit: int, $namespace: string) {
		insights(func: type(Insight), orderdesc: created_at, first: $limit) @filter(eq(namespace, $namespace)) {
			uid
			name
			description
			insight_type
			summary
			action_suggestion
			created_at
			confidence
			source_nodes {
				uid
				name
			}
		}
	}`)

	vars := map[string]string{
		"$limit":     fmt.Sprintf("%d", limit),
		"$namespace": namespace,
	}

	resp, err := q.client.Query(ctx, query, vars)
	if err != nil {
		return nil, err
	}

	var result struct {
		Insights []Insight `json:"insights"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Insights, nil
}

// GetPatterns retrieves all patterns for a namespace
func (q *QueryBuilder) GetPatterns(ctx context.Context, namespace string, minConfidence float64, limit int) ([]Pattern, error) {
	query := `query GetPatterns($minConf: float, $limit: int, $namespace: string) {
		patterns(func: type(Pattern), orderdesc: confidence_score, first: $limit) @filter(ge(confidence_score, $minConf) AND eq(namespace, $namespace)) {
			uid
			name
			description
			pattern_type
			frequency
			confidence_score
			predicted_action
			created_at
			trigger_nodes {
				uid
				name
			}
		}
	}`

	vars := map[string]string{
		"$minConf":   fmt.Sprintf("%f", minConfidence),
		"$limit":     fmt.Sprintf("%d", limit),
		"$namespace": namespace,
	}

	resp, err := q.client.Query(ctx, query, vars)
	if err != nil {
		return nil, err
	}

	var result struct {
		Patterns []Pattern `json:"patterns"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Patterns, nil
}

// GetAllNodes retrieves all nodes with names (most reliable fallback)
func (q *QueryBuilder) GetAllNodes(ctx context.Context, namespace string, limit int) ([]Node, error) {
	query := `query AllNodes($limit: int, $namespace: string) {
		nodes(func: has(name), orderdesc: activation, first: $limit) @filter(eq(namespace, $namespace)) {
			uid
			dgraph.type
			name
			description
			activation
			access_count
			last_accessed
			created_at
		}
	}`

	vars := map[string]string{
		"$limit":     fmt.Sprintf("%d", limit),
		"$namespace": namespace,
	}

	resp, err := q.client.Query(ctx, query, vars)
	if err != nil {
		return nil, err
	}

	var result struct {
		Nodes []Node `json:"nodes"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Nodes, nil
}

// GetNodesByType retrieves all nodes of a specific type
func (q *QueryBuilder) GetNodesByType(ctx context.Context, namespace string, nodeType NodeType, limit int) ([]Node, error) {
	query := fmt.Sprintf(`query NodesByType($limit: int, $namespace: string) {
		nodes(func: type(%s), orderdesc: activation, first: $limit) @filter(eq(namespace, $namespace)) {
			uid
			dgraph.type
			name
			description
			activation
			access_count
			last_accessed
			created_at
		}
	}`, nodeType)

	vars := map[string]string{
		"$limit":     fmt.Sprintf("%d", limit),
		"$namespace": namespace,
	}

	resp, err := q.client.Query(ctx, query, vars)
	if err != nil {
		return nil, err
	}

	var result struct {
		Nodes []Node `json:"nodes"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Nodes, nil
}

// CountNodes counts total nodes by type
func (q *QueryBuilder) CountNodes(ctx context.Context, nodeType NodeType) (int, error) {
	query := fmt.Sprintf(`{
		count(func: type(%s)) {
			total: count(uid)
		}
	}`, nodeType)

	resp, err := q.client.Query(ctx, query, nil)
	if err != nil {
		return 0, err
	}

	var result struct {
		Count []struct {
			Total int `json:"total"`
		} `json:"count"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return 0, err
	}

	if len(result.Count) > 0 {
		return result.Count[0].Total, nil
	}
	return 0, nil
}

// GetUserRelatedNodes retrieves nodes connected to the user via specific relationship predicates
func (q *QueryBuilder) GetUserRelatedNodes(ctx context.Context, userID string, limit int) ([]Node, error) {
	// First, find the User node by name with correct NodeType
	userNode, err := q.client.FindNodeByName(ctx, fmt.Sprintf("user_%s", userID), userID, NodeTypeUser)
	if err != nil || userNode == nil {
		// User node not found - this is expected for new users
		// Return empty rather than error to allow fallback search
		return nil, nil
	}

	// Query to get all nodes connected via KNOWS edge (stores all ingested entities)
	query := `query UserKnowledge($uid: string, $limit: int) {
		nodes(func: uid($uid)) {
			knows (first: $limit) {
				uid
				dgraph.type
				name
				description
				activation
				last_accessed
				
				# Also get the reverse relationship targets (e.g., who this person has_manager to)
				~has_manager {
					uid
					name
				}
			}
			has_manager {
				uid
				dgraph.type
				name
				description
				activation
			}
		}
	}`

	vars := map[string]string{
		"$uid":   userNode.UID,
		"$limit": fmt.Sprintf("%d", limit),
	}

	resp, err := q.client.Query(ctx, query, vars)
	if err != nil {
		return nil, err
	}

	var result struct {
		Nodes []struct {
			Knows      []Node `json:"knows"`
			HasManager []Node `json:"has_manager"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	// Combine all related nodes
	var allNodes []Node
	if len(result.Nodes) > 0 {
		allNodes = append(allNodes, result.Nodes[0].Knows...)
		allNodes = append(allNodes, result.Nodes[0].HasManager...)
	}

	return allNodes, nil
}
