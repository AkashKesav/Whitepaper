// Package graph provides the DGraph client for the Knowledge Graph.
package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dgraph-io/dgo/v240"
	"github.com/dgraph-io/dgo/v240/protos/api"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client wraps the DGraph client with connection pooling and helper methods
type Client struct {
	conn   *grpc.ClientConn
	dg     *dgo.Dgraph
	logger *zap.Logger
	mu     sync.RWMutex
}

// ClientConfig holds configuration for the DGraph client
type ClientConfig struct {
	Address        string
	MaxRetries     int
	RetryInterval  time.Duration
	RequestTimeout time.Duration
}

// DefaultClientConfig returns sensible defaults
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Address:        "localhost:9080",
		MaxRetries:     5,
		RetryInterval:  2 * time.Second,
		RequestTimeout: 30 * time.Second,
	}
}

// NewClient creates a new DGraph client with connection pooling
func NewClient(ctx context.Context, cfg ClientConfig, logger *zap.Logger) (*Client, error) {
	var conn *grpc.ClientConn
	var err error

	// Retry connection with backoff
	for i := 0; i < cfg.MaxRetries; i++ {
		conn, err = grpc.DialContext(ctx, cfg.Address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err == nil {
			break
		}
		logger.Warn("Failed to connect to DGraph, retrying...",
			zap.Int("attempt", i+1),
			zap.Error(err))
		time.Sleep(cfg.RetryInterval)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to DGraph after %d attempts: %w", cfg.MaxRetries, err)
	}

	dg := dgo.NewDgraphClient(api.NewDgraphClient(conn))

	client := &Client{
		conn:   conn,
		dg:     dg,
		logger: logger,
	}

	// Initialize schema
	if err := client.initSchema(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	logger.Info("DGraph client connected successfully", zap.String("address", cfg.Address))
	return client, nil
}

// initSchema sets up the DGraph schema for the Knowledge Graph
func (c *Client) initSchema(ctx context.Context) error {
	schema := `
		# Node types
		type User {
			name
			description
			attributes
			created_at
			updated_at
			last_accessed
			activation
			access_count
		}

		type Entity {
			name
			description
			attributes
			created_at
			updated_at
			last_accessed
			activation
			access_count
			entity_type
			tags
		}

		type Event {
			name
			description
			attributes
			created_at
			updated_at
			occurred_at
			sentiment
		}

		type Insight {
			name
			description
			insight_type
			summary
			action_suggestion
			source_nodes
			created_at
			confidence
		}

		type Pattern {
			name
			description
			pattern_type
			trigger_nodes
			frequency
			confidence_score
			predicted_action
			created_at
		}

		type Fact {
			name
			description
			fact_value
			created_at
			valid_from
			valid_until
			status
		}

		# Predicates with indexes
		name: string @index(exact, term, fulltext) .
		description: string @index(fulltext) .
		attributes: [string] .
		tags: [string] @index(term) .
		entity_type: string @index(exact) .
		
		# Temporal predicates
		created_at: datetime @index(hour) .
		updated_at: datetime @index(hour) .
		last_accessed: datetime @index(hour) .
		occurred_at: datetime @index(hour) .
		valid_from: datetime .
		valid_until: datetime .
		
		# Activation and prioritization
		activation: float @index(float) .
		access_count: int @index(int) .
		traversal_cost: float .
		
		# Insight/Pattern specific
		insight_type: string @index(exact) .
		pattern_type: string @index(exact) .
		summary: string @index(fulltext) .
		action_suggestion: string .
		predicted_action: string .
		frequency: int @index(int) .
		confidence: float @index(float) .
		confidence_score: float @index(float) .
		fact_value: string .
		status: string @index(exact) .
		sentiment: string @index(exact) .
		
		# Source tracking
		source_conversation_id: string @index(exact) .
		source_nodes: [uid] .
		trigger_nodes: [uid] .
		
		# Relationship predicates (edges)
		partner_is: uid @reverse .
		family_member: [uid] @reverse .
		friend_of: [uid] @reverse .
		has_manager: uid @reverse .
		works_on: [uid] @reverse .
		works_at: uid @reverse .
		colleague: [uid] @reverse .
		likes: [uid] @reverse .
		dislikes: [uid] @reverse .
		is_allergic_to: [uid] @reverse .
		prefers: [uid] @reverse .
		has_interest: [uid] @reverse .
		caused_by: [uid] @reverse .
		blocked_by: [uid] @reverse .
		results_in: [uid] @reverse .
		contradicts: [uid] @reverse .
		occurred_on: uid @reverse .
		scheduled_at: datetime .
		derived_from: [uid] @reverse .
		synthesized_from: [uid] @reverse .
		supersedes: uid @reverse .
		knows: [uid] @reverse .
		
		# Edge metadata predicates
		edge_status: string @index(exact) .
		edge_created_at: datetime .
		edge_activation: float .
		edge_confidence: float .
	`

	op := &api.Operation{Schema: schema}
	if err := c.dg.Alter(ctx, op); err != nil {
		return fmt.Errorf("failed to alter schema: %w", err)
	}

	c.logger.Info("DGraph schema initialized successfully")
	return nil
}

// Close closes the DGraph connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// NewTxn creates a new transaction
func (c *Client) NewTxn() *dgo.Txn {
	return c.dg.NewTxn()
}

// NewReadOnlyTxn creates a new read-only transaction
func (c *Client) NewReadOnlyTxn() *dgo.Txn {
	return c.dg.NewReadOnlyTxn()
}

// CreateNode creates a new node in the graph using NQuad format for reliability
func (c *Client) CreateNode(ctx context.Context, node *Node) (string, error) {
	node.CreatedAt = time.Now()
	node.UpdatedAt = time.Now()
	node.LastAccessed = time.Now()

	if node.Activation == 0 {
		node.Activation = 0.5
	}

	// Generate a unique blank node ID
	blankNode := fmt.Sprintf("_:node_%d", time.Now().UnixNano())

	// Build NQuads for reliable mutation
	var nquads strings.Builder

	// Type (required)
	nquads.WriteString(fmt.Sprintf(`%s <dgraph.type> "%s" .
`, blankNode, node.GetType()))

	// Name (required)
	nquads.WriteString(fmt.Sprintf(`%s <name> %q .
`, blankNode, node.Name))

	// Activation
	nquads.WriteString(fmt.Sprintf(`%s <activation> "%f"^^<xs:double> .
`, blankNode, node.Activation))

	// Confidence
	nquads.WriteString(fmt.Sprintf(`%s <confidence> "%f"^^<xs:double> .
`, blankNode, node.Confidence))

	// Timestamps
	nquads.WriteString(fmt.Sprintf(`%s <created_at> "%s"^^<xs:dateTime> .
`, blankNode, node.CreatedAt.Format(time.RFC3339)))
	nquads.WriteString(fmt.Sprintf(`%s <updated_at> "%s"^^<xs:dateTime> .
`, blankNode, node.UpdatedAt.Format(time.RFC3339)))
	nquads.WriteString(fmt.Sprintf(`%s <last_accessed> "%s"^^<xs:dateTime> .
`, blankNode, node.LastAccessed.Format(time.RFC3339)))

	// Optional fields
	if node.Description != "" {
		nquads.WriteString(fmt.Sprintf(`%s <description> %q .
`, blankNode, node.Description))
	}
	if node.SourceConversationID != "" {
		nquads.WriteString(fmt.Sprintf(`%s <source_conversation_id> %q .
`, blankNode, node.SourceConversationID))
	}

	for _, tag := range node.Tags {
		nquads.WriteString(fmt.Sprintf(`%s <tags> %q .
`, blankNode, tag))
	}

	c.logger.Debug("Creating node with NQuads",
		zap.String("name", node.Name),
		zap.String("type", string(node.GetType())),
		zap.String("nquads", nquads.String()))

	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquads.String()),
		CommitNow: true,
	}

	resp, err := txn.Mutate(ctx, mu)
	if err != nil {
		c.logger.Error("CreateNode mutation failed",
			zap.String("name", node.Name),
			zap.Error(err))
		return "", fmt.Errorf("failed to create node '%s': %w", node.Name, err)
	}

	// Extract the blank node ID suffix to find the UID
	blankNodeKey := blankNode[2:] // Remove "_:" prefix
	if uid, ok := resp.Uids[blankNodeKey]; ok {
		c.logger.Info("Created node successfully",
			zap.String("uid", uid),
			zap.String("name", node.Name),
			zap.String("type", string(node.GetType())))
		return uid, nil
	}

	// If no UID returned, list what we got
	c.logger.Error("No UID returned for node",
		zap.String("name", node.Name),
		zap.Any("returned_uids", resp.Uids))
	return "", fmt.Errorf("no UID returned for node '%s'", node.Name)
}

// GetNode retrieves a node by UID
func (c *Client) GetNode(ctx context.Context, uid string) (*Node, error) {
	query := `query Node($uid: string) {
		node(func: uid($uid)) {
			uid
			dgraph.type
			name
			description
			attributes
			created_at
			updated_at
			last_accessed
			activation
			access_count
			source_conversation_id
			confidence
		}
	}`

	vars := map[string]string{"$uid": uid}
	resp, err := c.dg.NewReadOnlyTxn().QueryWithVars(ctx, query, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to query node: %w", err)
	}

	var result struct {
		Node []Node `json:"node"`
	}
	if err := json.Unmarshal(resp.Json, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node: %w", err)
	}

	if len(result.Node) == 0 {
		return nil, fmt.Errorf("node not found: %s", uid)
	}

	return &result.Node[0], nil
}

// UpdateNodeActivation updates a node's activation level
func (c *Client) UpdateNodeActivation(ctx context.Context, uid string, activation float64) error {
	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	update := map[string]interface{}{
		"uid":           uid,
		"activation":    activation,
		"last_accessed": time.Now(),
	}

	updateJSON, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal update: %w", err)
	}

	mu := &api.Mutation{
		SetJson:   updateJSON,
		CommitNow: true,
	}

	_, err = txn.Mutate(ctx, mu)
	if err != nil {
		return fmt.Errorf("failed to update activation: %w", err)
	}

	return nil
}

// IncrementAccessCount increments a node's access count and boosts activation
func (c *Client) IncrementAccessCount(ctx context.Context, uid string, config ActivationConfig) error {
	node, err := c.GetNode(ctx, uid)
	if err != nil {
		return err
	}

	newActivation := node.Activation + config.BoostPerAccess
	if newActivation > config.MaxActivation {
		newActivation = config.MaxActivation
	}

	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	update := map[string]interface{}{
		"uid":           uid,
		"activation":    newActivation,
		"access_count":  node.AccessCount + 1,
		"last_accessed": time.Now(),
	}

	updateJSON, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal update: %w", err)
	}

	mu := &api.Mutation{
		SetJson:   updateJSON,
		CommitNow: true,
	}

	_, err = txn.Mutate(ctx, mu)
	return err
}

// AddTags appends new tags to an existing node
func (c *Client) AddTags(ctx context.Context, uid string, tags []string) error {
	if len(tags) == 0 {
		return nil
	}

	var nquads strings.Builder
	for _, tag := range tags {
		nquads.WriteString(fmt.Sprintf(`<%s> <tags> %q .
`, uid, tag))
	}

	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquads.String()),
		CommitNow: true,
	}

	_, err := txn.Mutate(ctx, mu)
	if err != nil {
		return fmt.Errorf("failed to add tags: %w", err)
	}
	return nil
}

// UpdateDescription updates the description of an existing node
func (c *Client) UpdateDescription(ctx context.Context, uid string, description string) error {
	if description == "" {
		return nil
	}

	nquad := fmt.Sprintf(`<%s> <description> %q .`, uid, description)

	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquad),
		CommitNow: true,
	}

	_, err := txn.Mutate(ctx, mu)
	if err != nil {
		return fmt.Errorf("failed to update description: %w", err)
	}
	return nil
}

// FindNodeByName finds a node by its name and type
func (c *Client) FindNodeByName(ctx context.Context, name string, nodeType NodeType) (*Node, error) {
	query := fmt.Sprintf(`query FindNode($name: string) {
		node(func: eq(name, $name)) @filter(type(%s)) {
			uid
			dgraph.type
			name
			description
			attributes
			created_at
			updated_at
			last_accessed
			activation
			access_count
		}
	}`, nodeType)

	vars := map[string]string{"$name": name}
	resp, err := c.dg.NewReadOnlyTxn().QueryWithVars(ctx, query, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to query node: %w", err)
	}

	var result struct {
		Node []Node `json:"node"`
	}
	if err := json.Unmarshal(resp.Json, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node: %w", err)
	}

	if len(result.Node) == 0 {
		return nil, nil // Not found, not an error
	}

	return &result.Node[0], nil
}

// GetNodesByNames fetches multiple nodes by name in a single query
func (c *Client) GetNodesByNames(ctx context.Context, names []string) (map[string]*Node, error) {
	if len(names) == 0 {
		return make(map[string]*Node), nil
	}

	// Build filter string: eq(name, "n1") OR eq(name, "n2") ...
	var filters []string
	for _, name := range names {
		filters = append(filters, fmt.Sprintf("eq(name, %q)", name))
	}
	filterStr := strings.Join(filters, " OR ")

	query := fmt.Sprintf(`query FindNodes {
		nodes(func: has(name)) @filter(%s) {
			uid
			dgraph.type
			name
			description
			attributes
			created_at
			updated_at
			last_accessed
			activation
			access_count
			tags
		}
	}`, filterStr)

	resp, err := c.dg.NewReadOnlyTxn().Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes batch: %w", err)
	}

	var result struct {
		Nodes []Node `json:"nodes"`
	}
	if err := json.Unmarshal(resp.Json, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal nodes: %w", err)
	}

	nodeMap := make(map[string]*Node)
	for i := range result.Nodes {
		node := &result.Nodes[i]
		nodeMap[node.Name] = node
	}

	// Also check for User nodes if requested (special case)
	// Simple approach: Iterate names, check if "User" type exists in result
	// Note: The above query filters by type(Entity), we might need type(User) too if mixed
	// For now, robustly assume this is primarily for Entity nodes as per ingestion logic

	return nodeMap, nil
}

// CreateEdge creates a relationship between two nodes
func (c *Client) CreateEdge(ctx context.Context, fromUID, toUID string, edgeType EdgeType, status EdgeStatus) error {
	predicateName := edgeTypeToPredicateName(edgeType)

	// Check for functional constraint
	if FunctionalEdges[edgeType] {
		if err := c.archiveExistingFunctionalEdge(ctx, fromUID, edgeType); err != nil {
			return fmt.Errorf("failed to archive existing edge: %w", err)
		}
	}

	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	nquad := fmt.Sprintf(`<%s> <%s> <%s> .`, fromUID, predicateName, toUID)

	mu := &api.Mutation{
		SetNquads: []byte(nquad),
		CommitNow: true,
	}

	_, err := txn.Mutate(ctx, mu)
	if err != nil {
		return fmt.Errorf("failed to create edge: %w", err)
	}

	c.logger.Debug("Created edge",
		zap.String("from", fromUID),
		zap.String("to", toUID),
		zap.String("type", string(edgeType)))

	return nil
}

// archiveExistingFunctionalEdge archives any existing "current" edge for functional relationships
func (c *Client) archiveExistingFunctionalEdge(ctx context.Context, fromUID string, edgeType EdgeType) error {
	predicateName := edgeTypeToPredicateName(edgeType)

	// Query for existing edges
	query := fmt.Sprintf(`query ExistingEdge($uid: string) {
		node(func: uid($uid)) {
			%s {
				uid
			}
		}
	}`, predicateName)

	vars := map[string]string{"$uid": fromUID}
	resp, err := c.dg.NewReadOnlyTxn().QueryWithVars(ctx, query, vars)
	if err != nil {
		return err
	}

	var result map[string][]map[string]interface{}
	if err := json.Unmarshal(resp.Json, &result); err != nil {
		return err
	}

	// Delete existing edges if found
	if nodes, ok := result["node"]; ok && len(nodes) > 0 {
		txn := c.dg.NewTxn()
		defer txn.Discard(ctx)

		for _, existing := range nodes {
			if edges, ok := existing[predicateName].([]interface{}); ok {
				for _, edge := range edges {
					if edgeMap, ok := edge.(map[string]interface{}); ok {
						if existingUID, ok := edgeMap["uid"].(string); ok {
							nquad := fmt.Sprintf(`<%s> <%s> <%s> .`, fromUID, predicateName, existingUID)
							mu := &api.Mutation{
								DelNquads: []byte(nquad),
								CommitNow: true,
							}
							if _, err := txn.Mutate(ctx, mu); err != nil {
								c.logger.Warn("Failed to delete existing edge",
									zap.String("from", fromUID),
									zap.String("to", existingUID),
									zap.Error(err))
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// CreateNodes batch creates multiple nodes in a single mutation
func (c *Client) CreateNodes(ctx context.Context, nodes []*Node) (map[string]string, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	var nquads strings.Builder
	// Map from temporary ID to node Name to resolve UIDs later
	tempIDToName := make(map[string]string)

	for i, node := range nodes {
		node.CreatedAt = time.Now()
		node.UpdatedAt = time.Now()
		node.LastAccessed = time.Now()
		if node.Activation == 0 {
			node.Activation = 0.5
		}

		// Use a unique blank node for this batch
		blankNode := fmt.Sprintf("_:node_%d_%d", time.Now().UnixNano(), i)
		tempIDToName[blankNode[2:]] = node.Name // Store without "_:"

		// Type
		for _, dtype := range node.DType {
			nquads.WriteString(fmt.Sprintf(`%s <dgraph.type> "%s" .
`, blankNode, dtype))
		}

		// Name
		nquads.WriteString(fmt.Sprintf(`%s <name> %q .
`, blankNode, node.Name))

		// Metadata
		nquads.WriteString(fmt.Sprintf(`%s <activation> "%f"^^<xs:double> .
`, blankNode, node.Activation))
		nquads.WriteString(fmt.Sprintf(`%s <confidence> "%f"^^<xs:double> .
`, blankNode, node.Confidence))
		nquads.WriteString(fmt.Sprintf(`%s <created_at> "%s"^^<xs:dateTime> .
`, blankNode, node.CreatedAt.Format(time.RFC3339)))

		// Description
		if node.Description != "" {
			nquads.WriteString(fmt.Sprintf(`%s <description> %q .
`, blankNode, node.Description))
		}

		// Tags
		for _, tag := range node.Tags {
			nquads.WriteString(fmt.Sprintf(`%s <tags> %q .
`, blankNode, tag))
		}
	}

	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquads.String()),
		CommitNow: true,
	}

	resp, err := txn.Mutate(ctx, mu)
	if err != nil {
		return nil, fmt.Errorf("batch create nodes failed: %w", err)
	}

	// Map returned UIDs back to names
	nameToUID := make(map[string]string)
	for blankID, realUID := range resp.Uids {
		if name, ok := tempIDToName[blankID]; ok {
			nameToUID[name] = realUID
		}
	}

	return nameToUID, nil
}

// EdgeInput represents a single edge to be created in a batch
type EdgeInput struct {
	FromUID string
	ToUID   string
	Type    EdgeType
	Status  EdgeStatus
}

// CreateEdges batch creates multiple edges in a single mutation
func (c *Client) CreateEdges(ctx context.Context, edges []EdgeInput) error {
	if len(edges) == 0 {
		return nil
	}

	var nquads strings.Builder
	for _, edge := range edges {
		predicateName := edgeTypeToPredicateName(edge.Type)
		nquads.WriteString(fmt.Sprintf(`<%s> <%s> <%s> .
`, edge.FromUID, predicateName, edge.ToUID))
	}

	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquads.String()),
		CommitNow: true,
	}

	_, err := txn.Mutate(ctx, mu)
	if err != nil {
		return fmt.Errorf("batch create edges failed: %w", err)
	}

	return nil
}

// edgeTypeToPredicateName converts EdgeType to DGraph predicate name
func edgeTypeToPredicateName(edgeType EdgeType) string {
	mapping := map[EdgeType]string{
		EdgeTypePartnerIs:    "partner_is",
		EdgeTypeFamilyMember: "family_member",
		EdgeTypeFriendOf:     "friend_of",
		EdgeTypeHasManager:   "has_manager",
		EdgeTypeWorksOn:      "works_on",
		EdgeTypeWorksAt:      "works_at",
		EdgeTypeColleague:    "colleague",
		EdgeTypeLikes:        "likes",
		EdgeTypeDislikes:     "dislikes",
		EdgeTypeIsAllergic:   "is_allergic_to",
		EdgeTypePrefers:      "prefers",
		EdgeTypeHasInterest:  "has_interest",
		EdgeTypeCausedBy:     "caused_by",
		EdgeTypeBlockedBy:    "blocked_by",
		EdgeTypeResultsIn:    "results_in",
		EdgeTypeContradicts:  "contradicts",
		EdgeTypeOccurredOn:   "occurred_on",
		EdgeTypeDerivedFrom:  "derived_from",
		EdgeTypeSynthesized:  "synthesized_from",
		EdgeTypeSupersedes:   "supersedes",
		EdgeTypeKnows:        "knows",
	}

	if pred, ok := mapping[edgeType]; ok {
		return pred
	}
	return string(edgeType)
}

// Query executes a raw DGraph query
func (c *Client) Query(ctx context.Context, query string, vars map[string]string) ([]byte, error) {
	resp, err := c.dg.NewReadOnlyTxn().QueryWithVars(ctx, query, vars)
	if err != nil {
		return nil, err
	}
	return resp.Json, nil
}

// Mutate executes a raw DGraph mutation
func (c *Client) Mutate(ctx context.Context, mutation *api.Mutation) (*api.Response, error) {
	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)
	return txn.Mutate(ctx, mutation)
}
