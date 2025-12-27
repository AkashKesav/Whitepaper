// Package graph provides the DGraph client for the Knowledge Graph.
package graph

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dgraph-io/dgo/v240"
	"github.com/dgraph-io/dgo/v240/protos/api"
	"github.com/google/uuid"
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
		RequestTimeout: 10 * time.Second, // Default 10s timeout for DGraph calls
	}
}

// timeoutInterceptor creates a gRPC unary interceptor that enforces per-call timeouts.
// This prevents slow DGraph queries from blocking the API indefinitely.
func timeoutInterceptor(timeout time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// Only add timeout if context doesn't already have a deadline
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		return invoker(ctx, method, req, reply, cc, opts...)
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
			grpc.WithUnaryInterceptor(timeoutInterceptor(cfg.RequestTimeout)),
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
		# Edges
		group_has_admin: [uid] @reverse .
		group_has_member: [uid] @reverse .
		
		# Node types
		# Group Management (V2)
		
		type Group {
			name
			description
			namespace
			created_by
			created_at
			updated_at
			group_has_admin
			group_has_member
		}

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
		namespace: string @index(exact) .
		created_by: string @index(exact) .
		
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

		# Workspace Collaboration Types
		type WorkspaceInvitation {
			workspace_id
			invitee_user_id
			role
			status
			created_at
			created_by
		}

		type ShareLink {
			workspace_id
			token
			role
			max_uses
			current_uses
			expires_at
			is_active
			created_at
			created_by
		}

		# Workspace Collaboration Predicates
		workspace_id: string @index(exact) .
		invitee_user_id: string @index(exact) .
		token: string @index(exact) .
		max_uses: int .
		current_uses: int .
		expires_at: datetime .
		is_active: bool @index(bool) .
		role: string @index(exact) .

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

	// Namespace (critical for isolation)
	if node.Namespace != "" {
		nquads.WriteString(fmt.Sprintf(`%s <namespace> %q .
`, blankNode, node.Namespace))
	}

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

// GetNodesByNames fetches multiple nodes by name in a single query, scoped to namespace
func (c *Client) GetNodesByNames(ctx context.Context, namespace string, names []string) (map[string]*Node, error) {
	if len(names) == 0 {
		return make(map[string]*Node), nil
	}

	// Build filter string: eq(name, "n1") OR eq(name, "n2") ...
	var filters []string
	for _, name := range names {
		filters = append(filters, fmt.Sprintf("eq(name, %q)", name))
	}
	filterStr := strings.Join(filters, " OR ")

	query := fmt.Sprintf(`query FindNodes($namespace: string) {
		nodes(func: has(name)) @filter((%s) AND eq(namespace, $namespace)) {
			uid
			dgraph.type
			name
			description
			attributes
			created_at
			activation
			namespace
		}
	}`, filterStr)
	resp, err := c.dg.NewReadOnlyTxn().QueryWithVars(ctx, query, map[string]string{"$namespace": namespace})
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

		// Namespace
		if node.Namespace != "" {
			nquads.WriteString(fmt.Sprintf(`%s <namespace> %q .
`, blankNode, node.Namespace))
		}

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

// GetNodesByUIDs fetches multiple nodes by their UIDs in a single query
// Used by Hybrid RAG to retrieve full node data after vector search
func (c *Client) GetNodesByUIDs(ctx context.Context, uids []string) ([]Node, error) {
	if len(uids) == 0 {
		return nil, nil
	}

	// Build UID list for query
	uidList := strings.Join(uids, ",")

	query := fmt.Sprintf(`{
		nodes(func: uid(%s)) {
			uid
			dgraph.type
			name
			description
			tags
			activation
			created_at
			namespace
		}
	}`, uidList)

	resp, err := c.dg.NewReadOnlyTxn().Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes by UIDs: %w", err)
	}

	var result struct {
		Nodes []Node `json:"nodes"`
	}
	if err := json.Unmarshal(resp.Json, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal nodes: %w", err)
	}

	return result.Nodes, nil
}

// Mutate executes a raw DGraph mutation
func (c *Client) Mutate(ctx context.Context, mutation *api.Mutation) (*api.Response, error) {
	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)
	return txn.Mutate(ctx, mutation)
}

// EnsureUserNode creates a User node in DGraph if it doesn't exist (idempotent)
func (c *Client) EnsureUserNode(ctx context.Context, username string) error {
	// Check if user already exists
	existing, err := c.FindNodeByName(ctx, username, NodeTypeUser)
	if err != nil {
		return fmt.Errorf("failed to check user existence: %w", err)
	}
	if existing != nil {
		return nil // Already exists
	}

	// Create new User node
	now := time.Now().Format(time.RFC3339)
	nquads := fmt.Sprintf(`
		_:user <dgraph.type> "User" .
		_:user <name> %q .
		_:user <namespace> %q .
		_:user <created_at> %q .
		_:user <updated_at> %q .
		_:user <activation> "0.5" .
	`, username, fmt.Sprintf("user_%s", username), now, now)

	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquads),
		CommitNow: true,
	}

	_, err = txn.Mutate(ctx, mu)
	if err != nil {
		return fmt.Errorf("failed to create user node: %w", err)
	}

	c.logger.Info("Created User node in DGraph", zap.String("username", username))
	return nil
}

// CreateGroup creates a new group (V2) with strict namespace isolation and admin hierarchy
func (c *Client) CreateGroup(ctx context.Context, name, description, ownerID string) (string, error) {
	// Find owner user
	ownerNode, err := c.FindNodeByName(ctx, ownerID, NodeTypeUser)
	if err != nil {
		return "", fmt.Errorf("failed to find owner: %w", err)
	}
	if ownerNode == nil {
		return "", fmt.Errorf("owner user %s not found", ownerID)
	}

	groupID := uuid.New().String()
	namespace := fmt.Sprintf("group_%s", groupID)

	// Create Group Node (It exists within its OWN namespace so it can be found by queries filtering for that group)
	// WAIT: A group node itself acts as the anchor. If I put it in "group_X", then to find it I need to know "group_X".
	// But I don't know "group_X" yet.
	// The Group Node must be visible to the CREATOR (user_X) and Members.
	// So Group Node should effectively be in "user_X" (created by)?
	// NO.
	// Solution: The Group Node has `namespace: "system"` or `namespace: "registry"`, OR
	// We simply use the `group_has_member` edges from the USER to find the groups.
	// The USER is in `user_X`. The User Node has `~group_has_member` edge to Group Node.
	// When we query `User { ~group_has_member { ... } }` we are traversing FROM `user_X`.
	// As long as the Group Node EXISTS, DGraph handles the traversal.
	// The `namespace` field on the Group Node is just metadata for "what namespace does this define".
	// So, we set `namespace` = `group_X` on the Group Node to indicate "I define this space".

	now := time.Now().Format(time.RFC3339)
	groupUID := "_:newgroup"

	nquads := fmt.Sprintf(`
		%s <dgraph.type> "Group" .
		%s <name> %q .
		%s <description> %q .
		%s <namespace> %q .
		%s <created_at> %q .
		%s <updated_at> %q .
		
		# Hierarchy
		%s <group_has_admin> <%s> .
		%s <group_has_member> <%s> .
	`,
		groupUID, groupUID, name,
		groupUID, description,
		groupUID, namespace,
		groupUID, now,
		groupUID, now,
		groupUID, ownerNode.UID,
		groupUID, ownerNode.UID)

	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquads),
		CommitNow: true,
	}

	_, err = txn.Mutate(ctx, mu)
	if err != nil {
		return "", fmt.Errorf("failed to create group: %w", err)
	}

	// The returned ID is the DGraph UID.
	// But the LOGICAL ID (for namespace) is the UUID we generated.
	// We should probably store the UUID as a property `group_id` for external reference?
	// For now, returning the Namespace string as the identifier to the caller is most useful.
	// But `resp.Uids["newgroup"]` returns the DGraph UID.
	// Let's return the namespace string, as that is the "ID" needed for further API calls.

	return namespace, nil
}

// AddGroupMember adds a user to a group (V2)
func (c *Client) AddGroupMember(ctx context.Context, groupNamespace, username string) error {
	// groupNamespace e.g. "group_<UUID>"
	// We need to find the Group Node by its namespace field.

	q := `query FindGroup($ns: string) {
		g(func: eq(namespace, $ns)) @filter(type(Group)) {
			uid
		}
	}`
	resp, err := c.Query(ctx, q, map[string]string{"$ns": groupNamespace})
	if err != nil {
		return fmt.Errorf("failed to query group: %w", err)
	}

	var res struct {
		G []struct {
			UID string `json:"uid"`
		} `json:"g"`
	}
	json.Unmarshal(resp, &res)
	if len(res.G) == 0 {
		return fmt.Errorf("group %s not found", groupNamespace)
	}
	groupUID := res.G[0].UID

	// Find the User
	userNode, err := c.FindNodeByName(ctx, username, NodeTypeUser)
	if err != nil || userNode == nil {
		return fmt.Errorf("user %s not found", username)
	}

	nquad := fmt.Sprintf(`<%s> <group_has_member> <%s> .`, groupUID, userNode.UID)

	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquad),
		CommitNow: true,
	}

	if _, err := txn.Mutate(ctx, mu); err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}
	return nil
}

// RemoveGroupMember removes a user from a group
func (c *Client) RemoveGroupMember(ctx context.Context, groupID, username string) error {
	// Find Group UID (groupID is namespace/logical ID usually, but here likely passed as proper UID or namespace?)
	// Wait, Kernel passes "groupID". CreateGroup returns "namespace".
	// Let's assume input is "groupNamespace" for consistency with AddGroupMember.
	// We need to resolve it.

	q := `query FindGroup($ns: string) {
		g(func: eq(namespace, $ns)) @filter(type(Group)) {
			uid
		}
	}`
	resp, err := c.Query(ctx, q, map[string]string{"$ns": groupID})
	if err != nil {
		return fmt.Errorf("failed to query group: %w", err)
	}

	var res struct {
		G []struct {
			UID string `json:"uid"`
		} `json:"g"`
	}
	json.Unmarshal(resp, &res)
	if len(res.G) == 0 {
		return fmt.Errorf("group %s not found", groupID)
	}
	groupUID := res.G[0].UID

	// Find User
	userNode, err := c.FindNodeByName(ctx, username, NodeTypeUser)
	if err != nil || userNode == nil {
		return fmt.Errorf("user %s not found", username)
	}

	// Delete Edge: <GroupUID> <group_has_member> <UserUID>
	nquad := fmt.Sprintf(`<%s> <group_has_member> <%s> .`, groupUID, userNode.UID)

	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		DelNquads: []byte(nquad),
		CommitNow: true,
	}

	if _, err := txn.Mutate(ctx, mu); err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}
	return nil
}

// DeleteGroup deletes a group (and its edges automatically due to DGraph behavior on node deletion? No, usually explicitly needed)
// For safety, we just delete the node.
func (c *Client) DeleteGroup(ctx context.Context, groupID string) error {
	// groupID is namespace
	q := `query FindGroup($ns: string) {
		g(func: eq(namespace, $ns)) @filter(type(Group)) {
			uid
		}
	}`
	resp, err := c.Query(ctx, q, map[string]string{"$ns": groupID})
	if err != nil {
		return err
	}
	var res struct {
		G []struct {
			UID string `json:"uid"`
		} `json:"g"`
	}
	json.Unmarshal(resp, &res)
	if len(res.G) == 0 {
		return nil // Already gone
	}

	d := map[string]string{"uid": res.G[0].UID}
	db, err := json.Marshal(d)
	if err != nil {
		return err
	}

	mu := &api.Mutation{
		DeleteJson: db,
		CommitNow:  true,
	}

	_, err = c.dg.NewTxn().Mutate(ctx, mu)
	return err
}

// ShareToGroup shares a conversation ID with a group
func (c *Client) ShareToGroup(ctx context.Context, conversationID, groupID string) error {
	// 1. Find Group
	q := `query FindGroup($ns: string) {
		g(func: eq(namespace, $ns)) @filter(type(Group)) {
			uid
		}
	}`
	resp, err := c.Query(ctx, q, map[string]string{"$ns": groupID})
	if err != nil {
		return err
	}
	var res struct {
		G []struct {
			UID string `json:"uid"`
		} `json:"g"`
	}
	json.Unmarshal(resp, &res)
	if len(res.G) == 0 {
		return fmt.Errorf("group %s not found", groupID)
	}
	groupUID := res.G[0].UID

	// 2. Create Shared Record
	// For "Quantum" speed, we just write a new SharedConversation node linked to the group
	blankNode := fmt.Sprintf("_:share_%d", time.Now().UnixNano())
	var nquads strings.Builder

	nquads.WriteString(fmt.Sprintf(`%s <dgraph.type> "SharedConversation" .
`, blankNode))
	nquads.WriteString(fmt.Sprintf(`%s <conversation_id> %q .
`, blankNode, conversationID))
	nquads.WriteString(fmt.Sprintf(`%s <shared_with> <%s> .
`, blankNode, groupUID))
	nquads.WriteString(fmt.Sprintf(`%s <shared_at> "%s"^^<xs:dateTime> .
`, blankNode, time.Now().Format(time.RFC3339)))

	mu := &api.Mutation{
		SetNquads: []byte(nquads.String()),
		CommitNow: true,
	}

	_, err = c.dg.NewTxn().Mutate(ctx, mu)
	return err
}

// ListUserGroups returns groups the user is a member of (V2)
// NOTE: This intentionally steps OUTSIDE the strict namespace filter for discovery.
func (c *Client) ListUserGroups(ctx context.Context, userID string) ([]Group, error) {
	userNode, err := c.FindNodeByName(ctx, userID, NodeTypeUser)
	if err != nil || userNode == nil {
		return nil, fmt.Errorf("user not found: %s", userID)
	}

	// Traverse from User -> ~group_has_member -> Group
	query := `query UserGroups($user: string) {
		groups(func: type(Group)) @filter(uid_in(group_has_member, $user)) {
			uid
			name
			description
			namespace
			created_at
			# We can also fetch members if needed
			group_has_member {
				uid
				name
			}
		}
	}`

	resp, err := c.Query(ctx, query, map[string]string{"$user": userNode.UID})
	if err != nil {
		return nil, err
	}

	var result struct {
		Groups []Group `json:"groups"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Groups, nil
}

// IsGroupAdmin checks if a user is an admin of the group
func (c *Client) IsGroupAdmin(ctx context.Context, groupNamespace, userID string) (bool, error) {
	userNode, err := c.FindNodeByName(ctx, userID, NodeTypeUser)
	if err != nil || userNode == nil {
		return false, fmt.Errorf("user not found: %s", userID)
	}

	query := `query IsAdmin($ns: string, $user: string) {
		g(func: eq(namespace, $ns)) @filter(uid_in(group_has_admin, $user)) {
			uid
		}
	}`

	resp, err := c.Query(ctx, query, map[string]string{
		"$ns":   groupNamespace,
		"$user": userNode.UID,
	})
	if err != nil {
		return false, err
	}

	var res struct {
		G []struct {
			UID string `json:"uid"`
		} `json:"g"`
	}
	if err := json.Unmarshal(resp, &res); err != nil {
		return false, err
	}

	return len(res.G) > 0, nil
}

// IngestWisdomBatch batch creates summary nodes and entities for the Wisdom Layer
// Returns the UID of the created summary node for vector indexing
func (c *Client) IngestWisdomBatch(ctx context.Context, namespace string, summary string, entities []ExtractedEntity) (string, error) {
	var nquads strings.Builder

	// 1. Create Summary Node (Unique logical fact per batch timestamp for now)
	summaryBlankID := fmt.Sprintf("summary_%d", time.Now().UnixNano())
	summaryNode := "_:" + summaryBlankID

	nquads.WriteString(fmt.Sprintf(`%s <dgraph.type> "Fact" .
`, summaryNode))
	nquads.WriteString(fmt.Sprintf(`%s <name> "Batch Summary" .
`, summaryNode))
	nquads.WriteString(fmt.Sprintf(`%s <description> %q .
`, summaryNode, summary))
	nquads.WriteString(fmt.Sprintf(`%s <fact_value> %q .
`, summaryNode, summary)) // Use fact_value as main content
	nquads.WriteString(fmt.Sprintf(`%s <namespace> %q .
`, summaryNode, namespace))
	nquads.WriteString(fmt.Sprintf(`%s <created_at> "%s"^^<xs:dateTime> .
`, summaryNode, time.Now().Format(time.RFC3339)))
	nquads.WriteString(fmt.Sprintf(`%s <status> "crystallized" .
`, summaryNode))

	// 2. Process Entities
	// We need to link them to the summary and creating them if missing
	for i, e := range entities {
		// Entity Node
		entityNode := fmt.Sprintf("_:entity_%d", i)

		nquads.WriteString(fmt.Sprintf(`%s <dgraph.type> "Entity" .
`, entityNode))
		nquads.WriteString(fmt.Sprintf(`%s <name> %q .
`, entityNode, e.Name))
		nquads.WriteString(fmt.Sprintf(`%s <namespace> %q .
`, entityNode, namespace))

		// Link Entity -> Summary (Derived From)
		nquads.WriteString(fmt.Sprintf(`%s <synthesized_from> %s .
`, entityNode, summaryNode))
	}

	c.logger.Debug("Writing Wisdom Batch", zap.String("namespace", namespace))

	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquads.String()),
		CommitNow: true,
	}

	resp, err := txn.Mutate(ctx, mu)
	if err != nil {
		return "", fmt.Errorf("failed to ingest wisdom batch: %w", err)
	}

	// Extract the UID of the created summary node
	summaryUID := ""
	if uid, ok := resp.Uids[summaryBlankID]; ok {
		summaryUID = uid
	}

	return summaryUID, nil
}

// ============================================================================
// WORKSPACE COLLABORATION FUNCTIONS
// ============================================================================

// InviteToWorkspace creates a username-based invitation to join a workspace
func (c *Client) InviteToWorkspace(ctx context.Context, workspaceNS, inviterID, inviteeUsername, role string) (*WorkspaceInvitation, error) {
	// Validate role
	if role != "admin" && role != "subuser" {
		return nil, fmt.Errorf("invalid role: %s (must be 'admin' or 'subuser')", role)
	}

	// Check if invitee exists
	inviteeNode, err := c.FindNodeByName(ctx, inviteeUsername, NodeTypeUser)
	if err != nil || inviteeNode == nil {
		return nil, fmt.Errorf("user %s not found", inviteeUsername)
	}

	// Check if already a member
	isMember, err := c.IsWorkspaceMember(ctx, workspaceNS, inviteeUsername)
	if err != nil {
		return nil, err
	}
	if isMember {
		return nil, fmt.Errorf("user %s is already a member of this workspace", inviteeUsername)
	}

	// Check for existing pending invitation
	existingInvite, err := c.findPendingInvitation(ctx, workspaceNS, inviteeUsername)
	if err == nil && existingInvite != nil {
		return nil, fmt.Errorf("pending invitation already exists for user %s", inviteeUsername)
	}

	// Create invitation
	blankNode := fmt.Sprintf("_:invite_%d", time.Now().UnixNano())
	var nquads strings.Builder

	nquads.WriteString(fmt.Sprintf(`%s <dgraph.type> "WorkspaceInvitation" .
`, blankNode))
	nquads.WriteString(fmt.Sprintf(`%s <workspace_id> %q .
`, blankNode, workspaceNS))
	nquads.WriteString(fmt.Sprintf(`%s <invitee_user_id> %q .
`, blankNode, inviteeUsername))
	nquads.WriteString(fmt.Sprintf(`%s <role> %q .
`, blankNode, role))
	nquads.WriteString(fmt.Sprintf(`%s <status> "pending" .
`, blankNode))
	nquads.WriteString(fmt.Sprintf(`%s <created_at> "%s"^^<xs:dateTime> .
`, blankNode, time.Now().Format(time.RFC3339)))
	nquads.WriteString(fmt.Sprintf(`%s <created_by> %q .
`, blankNode, inviterID))

	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquads.String()),
		CommitNow: true,
	}

	resp, err := txn.Mutate(ctx, mu)
	if err != nil {
		return nil, fmt.Errorf("failed to create invitation: %w", err)
	}

	inviteUID := ""
	blankKey := blankNode[2:]
	if uid, ok := resp.Uids[blankKey]; ok {
		inviteUID = uid
	}

	c.logger.Info("Created workspace invitation",
		zap.String("workspace", workspaceNS),
		zap.String("invitee", inviteeUsername),
		zap.String("role", role))

	return &WorkspaceInvitation{
		UID:           inviteUID,
		WorkspaceID:   workspaceNS,
		InviteeUserID: inviteeUsername,
		Role:          role,
		Status:        "pending",
		CreatedAt:     time.Now(),
		CreatedBy:     inviterID,
	}, nil
}

// findPendingInvitation finds an existing pending invitation
func (c *Client) findPendingInvitation(ctx context.Context, workspaceNS, username string) (*WorkspaceInvitation, error) {
	query := `query FindInvite($ws: string, $user: string) {
		invite(func: type(WorkspaceInvitation)) @filter(eq(workspace_id, $ws) AND eq(invitee_user_id, $user) AND eq(status, "pending")) {
			uid
			workspace_id
			invitee_user_id
			role
			status
			created_at
			created_by
		}
	}`

	resp, err := c.Query(ctx, query, map[string]string{
		"$ws":   workspaceNS,
		"$user": username,
	})
	if err != nil {
		return nil, err
	}

	var result struct {
		Invite []WorkspaceInvitation `json:"invite"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if len(result.Invite) == 0 {
		return nil, nil
	}

	return &result.Invite[0], nil
}

// IsWorkspaceMember checks if a user is a member (admin or subuser) of the workspace
func (c *Client) IsWorkspaceMember(ctx context.Context, workspaceNS, userID string) (bool, error) {
	userNode, err := c.FindNodeByName(ctx, userID, NodeTypeUser)
	if err != nil || userNode == nil {
		return false, nil
	}

	// NOTE: uid_in() requires a UID, not a string variable, so we embed the UID directly
	query := fmt.Sprintf(`query IsMember($ns: string) {
		g(func: eq(namespace, $ns)) @filter(type(Group) AND (uid_in(group_has_admin, %s) OR uid_in(group_has_member, %s))) {
			uid
		}
	}`, userNode.UID, userNode.UID)

	resp, err := c.Query(ctx, query, map[string]string{
		"$ns": workspaceNS,
	})
	if err != nil {
		return false, err
	}

	var res struct {
		G []struct {
			UID string `json:"uid"`
		} `json:"g"`
	}
	if err := json.Unmarshal(resp, &res); err != nil {
		return false, err
	}

	return len(res.G) > 0, nil
}

// AcceptInvitation accepts a pending invitation and adds user to workspace
func (c *Client) AcceptInvitation(ctx context.Context, invitationUID, userID string) error {
	// Get the invitation
	invite, err := c.getInvitation(ctx, invitationUID)
	if err != nil {
		return err
	}

	// Verify this invitation is for this user
	if invite.InviteeUserID != userID {
		return fmt.Errorf("invitation is not for user %s", userID)
	}

	if invite.Status != "pending" {
		return fmt.Errorf("invitation is not pending (status: %s)", invite.Status)
	}

	// Add user to workspace with appropriate role
	if invite.Role == "admin" {
		if err := c.addWorkspaceAdmin(ctx, invite.WorkspaceID, userID); err != nil {
			return fmt.Errorf("failed to add admin: %w", err)
		}
	} else {
		if err := c.AddGroupMember(ctx, invite.WorkspaceID, userID); err != nil {
			return fmt.Errorf("failed to add member: %w", err)
		}
	}

	// Update invitation status
	nquad := fmt.Sprintf(`<%s> <status> "accepted" .`, invitationUID)
	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquad),
		CommitNow: true,
	}

	if _, err := txn.Mutate(ctx, mu); err != nil {
		return fmt.Errorf("failed to update invitation status: %w", err)
	}

	c.logger.Info("Invitation accepted",
		zap.String("invitation", invitationUID),
		zap.String("user", userID))

	return nil
}

// addWorkspaceAdmin adds a user as admin to a workspace
func (c *Client) addWorkspaceAdmin(ctx context.Context, workspaceNS, userID string) error {
	// Find group by namespace
	q := `query FindGroup($ns: string) {
		g(func: eq(namespace, $ns)) @filter(type(Group)) {
			uid
		}
	}`
	resp, err := c.Query(ctx, q, map[string]string{"$ns": workspaceNS})
	if err != nil {
		return err
	}

	var res struct {
		G []struct {
			UID string `json:"uid"`
		} `json:"g"`
	}
	json.Unmarshal(resp, &res)
	if len(res.G) == 0 {
		return fmt.Errorf("group %s not found", workspaceNS)
	}
	groupUID := res.G[0].UID

	// Find User
	userNode, err := c.FindNodeByName(ctx, userID, NodeTypeUser)
	if err != nil || userNode == nil {
		return fmt.Errorf("user %s not found", userID)
	}

	nquad := fmt.Sprintf(`<%s> <group_has_admin> <%s> .`, groupUID, userNode.UID)

	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquad),
		CommitNow: true,
	}

	if _, err := txn.Mutate(ctx, mu); err != nil {
		return fmt.Errorf("failed to add admin: %w", err)
	}
	return nil
}

// getInvitation retrieves an invitation by UID
func (c *Client) getInvitation(ctx context.Context, invitationUID string) (*WorkspaceInvitation, error) {
	query := `query GetInvite($uid: string) {
		invite(func: uid($uid)) @filter(type(WorkspaceInvitation)) {
			uid
			workspace_id
			invitee_user_id
			role
			status
			created_at
			created_by
		}
	}`

	resp, err := c.Query(ctx, query, map[string]string{"$uid": invitationUID})
	if err != nil {
		return nil, err
	}

	var result struct {
		Invite []WorkspaceInvitation `json:"invite"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if len(result.Invite) == 0 {
		return nil, fmt.Errorf("invitation not found: %s", invitationUID)
	}

	return &result.Invite[0], nil
}

// DeclineInvitation declines a pending invitation
func (c *Client) DeclineInvitation(ctx context.Context, invitationUID, userID string) error {
	// Get the invitation
	invite, err := c.getInvitation(ctx, invitationUID)
	if err != nil {
		return err
	}

	// Verify this invitation is for this user
	if invite.InviteeUserID != userID {
		return fmt.Errorf("invitation is not for user %s", userID)
	}

	if invite.Status != "pending" {
		return fmt.Errorf("invitation is not pending (status: %s)", invite.Status)
	}

	// Update invitation status
	nquad := fmt.Sprintf(`<%s> <status> "declined" .`, invitationUID)
	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquad),
		CommitNow: true,
	}

	if _, err := txn.Mutate(ctx, mu); err != nil {
		return fmt.Errorf("failed to update invitation status: %w", err)
	}

	c.logger.Info("Invitation declined",
		zap.String("invitation", invitationUID),
		zap.String("user", userID))

	return nil
}

// GetPendingInvitations returns all pending invitations for a user
func (c *Client) GetPendingInvitations(ctx context.Context, userID string) ([]WorkspaceInvitation, error) {
	query := `query GetInvites($user: string) {
		invites(func: type(WorkspaceInvitation)) @filter(eq(invitee_user_id, $user) AND eq(status, "pending")) {
			uid
			workspace_id
			invitee_user_id
			role
			status
			created_at
			created_by
		}
	}`

	resp, err := c.Query(ctx, query, map[string]string{"$user": userID})
	if err != nil {
		return nil, err
	}

	var result struct {
		Invites []WorkspaceInvitation `json:"invites"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Invites, nil
}

// CreateShareLink generates a shareable link for a workspace
func (c *Client) CreateShareLink(ctx context.Context, workspaceNS, creatorID string, maxUses int, expiresAt *time.Time) (*ShareLink, error) {
	// Generate cryptographic token
	token := generateSecureToken()

	blankNode := fmt.Sprintf("_:sharelink_%d", time.Now().UnixNano())
	var nquads strings.Builder

	nquads.WriteString(fmt.Sprintf(`%s <dgraph.type> "ShareLink" .
`, blankNode))
	nquads.WriteString(fmt.Sprintf(`%s <workspace_id> %q .
`, blankNode, workspaceNS))
	nquads.WriteString(fmt.Sprintf(`%s <token> %q .
`, blankNode, token))
	nquads.WriteString(fmt.Sprintf(`%s <role> "subuser" .
`, blankNode))
	nquads.WriteString(fmt.Sprintf(`%s <max_uses> "%d"^^<xs:int> .
`, blankNode, maxUses))
	nquads.WriteString(fmt.Sprintf(`%s <current_uses> "0"^^<xs:int> .
`, blankNode))
	nquads.WriteString(fmt.Sprintf(`%s <is_active> "true"^^<xs:boolean> .
`, blankNode))
	nquads.WriteString(fmt.Sprintf(`%s <created_at> "%s"^^<xs:dateTime> .
`, blankNode, time.Now().Format(time.RFC3339)))
	nquads.WriteString(fmt.Sprintf(`%s <created_by> %q .
`, blankNode, creatorID))

	if expiresAt != nil {
		nquads.WriteString(fmt.Sprintf(`%s <expires_at> "%s"^^<xs:dateTime> .
`, blankNode, expiresAt.Format(time.RFC3339)))
	}

	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquads.String()),
		CommitNow: true,
	}

	resp, err := txn.Mutate(ctx, mu)
	if err != nil {
		return nil, fmt.Errorf("failed to create share link: %w", err)
	}

	linkUID := ""
	blankKey := blankNode[2:]
	if uid, ok := resp.Uids[blankKey]; ok {
		linkUID = uid
	}

	c.logger.Info("Created share link",
		zap.String("workspace", workspaceNS),
		zap.String("token", token[:8]+"..."),
		zap.Int("max_uses", maxUses))

	return &ShareLink{
		UID:         linkUID,
		WorkspaceID: workspaceNS,
		Token:       token,
		Role:        "subuser",
		MaxUses:     maxUses,
		CurrentUses: 0,
		ExpiresAt:   expiresAt,
		IsActive:    true,
		CreatedAt:   time.Now(),
		CreatedBy:   creatorID,
	}, nil
}

// generateSecureToken creates a cryptographically secure random token
func generateSecureToken() string {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		// Fallback to UUID if crypto/rand fails
		return uuid.New().String() + uuid.New().String()
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

// JoinViaShareLink allows an authenticated user to join a workspace using a share link
func (c *Client) JoinViaShareLink(ctx context.Context, token, userID string) (*ShareLink, error) {
	// Get the share link
	link, err := c.getShareLink(ctx, token)
	if err != nil {
		return nil, err
	}

	// Validate the link
	if !link.IsActive {
		return nil, fmt.Errorf("share link has been revoked")
	}
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		return nil, fmt.Errorf("share link has expired")
	}
	if link.MaxUses > 0 && link.CurrentUses >= link.MaxUses {
		return nil, fmt.Errorf("share link usage limit reached")
	}

	// Check if already a member
	isMember, err := c.IsWorkspaceMember(ctx, link.WorkspaceID, userID)
	if err != nil {
		return nil, err
	}
	if isMember {
		return nil, fmt.Errorf("you are already a member of this workspace")
	}

	// Add user to workspace as subuser
	if err := c.AddGroupMember(ctx, link.WorkspaceID, userID); err != nil {
		return nil, fmt.Errorf("failed to join workspace: %w", err)
	}

	// Increment usage count
	nquad := fmt.Sprintf(`<%s> <current_uses> "%d"^^<xs:int> .`, link.UID, link.CurrentUses+1)
	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquad),
		CommitNow: true,
	}

	if _, err := txn.Mutate(ctx, mu); err != nil {
		c.logger.Warn("Failed to increment share link usage", zap.Error(err))
	}

	c.logger.Info("User joined via share link",
		zap.String("user", userID),
		zap.String("workspace", link.WorkspaceID))

	return link, nil
}

// getShareLink retrieves a share link by token
func (c *Client) getShareLink(ctx context.Context, token string) (*ShareLink, error) {
	query := `query GetLink($token: string) {
		link(func: type(ShareLink)) @filter(eq(token, $token)) {
			uid
			workspace_id
			token
			role
			max_uses
			current_uses
			expires_at
			is_active
			created_at
			created_by
		}
	}`

	resp, err := c.Query(ctx, query, map[string]string{"$token": token})
	if err != nil {
		return nil, err
	}

	var result struct {
		Link []ShareLink `json:"link"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if len(result.Link) == 0 {
		return nil, fmt.Errorf("share link not found")
	}

	return &result.Link[0], nil
}

// RevokeShareLink deactivates a share link
func (c *Client) RevokeShareLink(ctx context.Context, token, userID string) error {
	// Get the share link
	link, err := c.getShareLink(ctx, token)
	if err != nil {
		return err
	}

	// Verify user is admin of the workspace
	isAdmin, err := c.IsGroupAdmin(ctx, link.WorkspaceID, userID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return fmt.Errorf("only admins can revoke share links")
	}

	// Update is_active to false
	nquad := fmt.Sprintf(`<%s> <is_active> "false"^^<xs:boolean> .`, link.UID)
	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquad),
		CommitNow: true,
	}

	if _, err := txn.Mutate(ctx, mu); err != nil {
		return fmt.Errorf("failed to revoke share link: %w", err)
	}

	c.logger.Info("Share link revoked",
		zap.String("token", token[:8]+"..."),
		zap.String("by", userID))

	return nil
}

// GetWorkspaceMembers returns all members of a workspace with their roles
func (c *Client) GetWorkspaceMembers(ctx context.Context, workspaceNS string) ([]WorkspaceMember, error) {
	query := `query GetMembers($ns: string) {
		group(func: eq(namespace, $ns)) @filter(type(Group)) {
			uid
			name
			group_has_admin {
				uid
				name
				created_at
			}
			group_has_member {
				uid
				name
				created_at
			}
		}
	}`

	resp, err := c.Query(ctx, query, map[string]string{"$ns": workspaceNS})
	if err != nil {
		return nil, err
	}

	var result struct {
		Group []struct {
			UID     string `json:"uid"`
			Name    string `json:"name"`
			Admins  []Node `json:"group_has_admin"`
			Members []Node `json:"group_has_member"`
		} `json:"group"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if len(result.Group) == 0 {
		return nil, fmt.Errorf("workspace not found: %s", workspaceNS)
	}

	var members []WorkspaceMember

	// Add admins
	for _, admin := range result.Group[0].Admins {
		members = append(members, WorkspaceMember{
			User:     &admin,
			Role:     "admin",
			JoinedAt: admin.CreatedAt,
		})
	}

	// Add subusers (members)
	for _, member := range result.Group[0].Members {
		members = append(members, WorkspaceMember{
			User:     &member,
			Role:     "subuser",
			JoinedAt: member.CreatedAt,
		})
	}

	return members, nil
}

// GetShareLinks returns all active share links for a workspace
func (c *Client) GetShareLinks(ctx context.Context, workspaceNS string) ([]ShareLink, error) {
	query := `query GetLinks($ws: string) {
		links(func: type(ShareLink)) @filter(eq(workspace_id, $ws) AND eq(is_active, true)) {
			uid
			workspace_id
			token
			role
			max_uses
			current_uses
			expires_at
			is_active
			created_at
			created_by
		}
	}`

	resp, err := c.Query(ctx, query, map[string]string{"$ws": workspaceNS})
	if err != nil {
		return nil, err
	}

	var result struct {
		Links []ShareLink `json:"links"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Links, nil
}
