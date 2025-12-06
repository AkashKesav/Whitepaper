// Package graph provides the DGraph client for the Knowledge Graph.
package graph

import (
	"context"
	"encoding/json"
	"fmt"
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

// CreateNode creates a new node in the graph
func (c *Client) CreateNode(ctx context.Context, node *Node) (string, error) {
	node.CreatedAt = time.Now()
	node.UpdatedAt = time.Now()
	node.LastAccessed = time.Now()

	if node.Activation == 0 {
		node.Activation = 0.5 // Default activation
	}

	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	nodeJSON, err := json.Marshal(node)
	if err != nil {
		return "", fmt.Errorf("failed to marshal node: %w", err)
	}

	mu := &api.Mutation{
		SetJson:   nodeJSON,
		CommitNow: true,
	}

	resp, err := txn.Mutate(ctx, mu)
	if err != nil {
		return "", fmt.Errorf("failed to create node: %w", err)
	}

	// Get the assigned UID
	for _, uid := range resp.Uids {
		c.logger.Debug("Created node", zap.String("uid", uid), zap.String("type", string(node.Type)))
		return uid, nil
	}

	return "", fmt.Errorf("no UID returned for created node")
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
