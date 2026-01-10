package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dgraph-io/dgo/v240/protos/api"
	"github.com/reflective-memory-kernel/internal/graph"
	"go.uber.org/zap"
)

// PolicyStore handles persistence of policies to DGraph
type PolicyStore struct {
	graphClient *graph.Client
	logger      *zap.Logger
}

// NewPolicyStore creates a new policy store
func NewPolicyStore(graphClient *graph.Client, logger *zap.Logger) *PolicyStore {
	return &PolicyStore{
		graphClient: graphClient,
		logger:      logger,
	}
}

// toStringSlice converts interface{} (which can be string or []interface{}) to []string
// DGraph sometimes returns single-value list predicates as strings instead of arrays
func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []interface{}:
		result := make([]string, len(val))
		for i, item := range val {
			if s, ok := item.(string); ok {
				result[i] = s
			}
		}
		return result
	case string:
		return []string{val}
	case []string:
		return val
	default:
		return nil
	}
}

// PolicyNode represents a policy stored in DGraph
type PolicyNode struct {
	UID         string    `json:"uid,omitempty"`
	PolicyID    string    `json:"policy_id,omitempty"`
	Description string    `json:"description,omitempty"`
	Subjects    []string  `json:"subjects,omitempty"`
	Resources   []string  `json:"resources,omitempty"`
	Actions     []string  `json:"actions,omitempty"`
	Effect      string    `json:"effect,omitempty"`
	Conditions  string    `json:"conditions,omitempty"` // JSON-encoded map
	Namespace   string    `json:"namespace,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
	CreatedBy   string    `json:"created_by,omitempty"`
	IsActive    bool      `json:"is_active,omitempty"`
	Priority    int       `json:"priority,omitempty"` // Higher priority policies are evaluated first
}

// SavePolicy persists a policy to DGraph
func (ps *PolicyStore) SavePolicy(ctx context.Context, namespace string, policy Policy, createdBy string) (string, error) {
	// Marshal conditions to JSON
	conditionsJSON := "{}"
	if len(policy.Conditions) > 0 {
		data, _ := json.Marshal(policy.Conditions)
		conditionsJSON = string(data)
	}

	// Convert actions to strings
	actionStrs := make([]string, len(policy.Actions))
	for i, a := range policy.Actions {
		actionStrs[i] = string(a)
	}

	now := time.Now()
	nowStr := now.Format(time.RFC3339)

	// Build NQuads for the policy node
	blankNode := "_:policy"
	nquads := fmt.Sprintf(`
		%s <dgraph.type> "Policy" .
		%s <policy_id> %q .
		%s <description> %q .
		%s <namespace> %q .
		%s <effect> %q .
		%s <conditions> %q .
		%s <created_at> "%s"^^<xs:dateTime> .
		%s <updated_at> "%s"^^<xs:dateTime> .
		%s <created_by> %q .
		%s <is_active> "true"^^<xs:boolean> .
		%s <priority> "%d"^^<xs:int> .
	`, blankNode, blankNode, policy.ID,
		blankNode, policy.Description,
		blankNode, namespace,
		blankNode, string(policy.Effect),
		blankNode, conditionsJSON,
		blankNode, nowStr,
		blankNode, nowStr,
		blankNode, createdBy,
		blankNode,
		blankNode, 0)

	// Add subjects as faceted edges or string arrays
	for _, subject := range policy.Subjects {
		nquads += fmt.Sprintf(`%s <policy_subjects> %q .
`, blankNode, subject)
	}

	// Add resources
	for _, resource := range policy.Resources {
		nquads += fmt.Sprintf(`%s <policy_resources> %q .
`, blankNode, resource)
	}

	// Add actions
	for _, action := range actionStrs {
		nquads += fmt.Sprintf(`%s <policy_actions> %q .
`, blankNode, action)
	}

	txn := ps.graphClient.GetDgraphClient().NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquads),
		CommitNow: true,
	}

	resp, err := txn.Mutate(ctx, mu)
	if err != nil {
		return "", fmt.Errorf("failed to save policy: %w", err)
	}

	policyUID := ""
	if uid, ok := resp.Uids["policy"]; ok {
		policyUID = uid
	}

	ps.logger.Info("Saved policy to DGraph",
		zap.String("policy_id", policy.ID),
		zap.String("uid", policyUID),
		zap.String("namespace", namespace))

	return policyUID, nil
}

// LoadPolicies loads all policies for a namespace from DGraph
func (ps *PolicyStore) LoadPolicies(ctx context.Context, namespace string) ([]Policy, error) {
	// Query all policies (type based) to avoid index issues filters
	query := `query LoadPolicies() {
		policies(func: type(Policy)) {
			uid
			policy_id
			description
			namespace
			effect
			conditions
			policy_subjects
			policy_resources
			policy_actions
			priority
			is_active
			created_at
			updated_at
			created_by
		}
	}`

	resp, err := ps.graphClient.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query policies: %w", err)
	}

	var result struct {
		Policies []struct {
			UID       string      `json:"uid"`
			PolicyID  string      `json:"policy_id"`
			Desc      string      `json:"description"`
			Namespace string      `json:"namespace"`
			Effect    string      `json:"effect"`
			Conds     string      `json:"conditions"`
			Subjects  interface{} `json:"policy_subjects"` // Can be string or []string from DGraph
			Resources interface{} `json:"policy_resources"`
			Actions   interface{} `json:"policy_actions"`
			Priority  int         `json:"priority"`
			IsActive  bool        `json:"is_active"`
		} `json:"policies"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal policies: %w", err)
	}

	policies := make([]Policy, 0, len(result.Policies))
	for _, p := range result.Policies {
		// Filter by is_active (manual check)
		if !p.IsActive {
			continue
		}

		// In-memory filter for namespace
		// Default to "system" if p.Namespace is empty (backward compatibility)
		pNs := p.Namespace
		if pNs == "" {
			pNs = "system"
		}

		if pNs != namespace {
			continue
		}

		// Parse conditions
		var conditions map[string]string
		if p.Conds != "" {
			json.Unmarshal([]byte(p.Conds), &conditions)
		}

		// Convert flexible DGraph arrays to []string
		subjects := toStringSlice(p.Subjects)
		resources := toStringSlice(p.Resources)
		actionStrs := toStringSlice(p.Actions)

		// Convert action strings to Action type
		actions := make([]Action, len(actionStrs))
		for i, a := range actionStrs {
			actions[i] = Action(a)
		}

		policies = append(policies, Policy{
			ID:          p.PolicyID,
			Description: p.Desc,
			Subjects:    subjects,
			Resources:   resources,
			Actions:     actions,
			Effect:      Effect(p.Effect),
			Conditions:  conditions,
		})
	}

	ps.logger.Info("Loaded policies from DGraph",
		zap.String("namespace", namespace),
		zap.Int("count", len(policies)))

	return policies, nil
}

// LoadAllPolicies loads all active policies from DGraph (for system-wide policies)
func (ps *PolicyStore) LoadAllPolicies(ctx context.Context) ([]Policy, error) {
	query := `{
		policies(func: type(Policy)) @filter(eq(is_active, true)) {
			uid
			policy_id
			description
			namespace
			effect
			conditions
			policy_subjects
			policy_resources
			policy_actions
			priority
		}
	}`

	resp, err := ps.graphClient.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query all policies: %w", err)
	}

	var result struct {
		Policies []struct {
			PolicyID  string      `json:"policy_id"`
			Desc      string      `json:"description"`
			Namespace string      `json:"namespace"`
			Effect    string      `json:"effect"`
			Conds     string      `json:"conditions"`
			Subjects  interface{} `json:"policy_subjects"`
			Resources interface{} `json:"policy_resources"`
			Actions   interface{} `json:"policy_actions"`
		} `json:"policies"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal policies: %w", err)
	}

	policies := make([]Policy, 0, len(result.Policies))
	for _, p := range result.Policies {
		var conditions map[string]string
		if p.Conds != "" {
			json.Unmarshal([]byte(p.Conds), &conditions)
		}

		subjects := toStringSlice(p.Subjects)
		resources := toStringSlice(p.Resources)
		actionStrs := toStringSlice(p.Actions)

		actions := make([]Action, len(actionStrs))
		for i, a := range actionStrs {
			actions[i] = Action(a)
		}

		policies = append(policies, Policy{
			ID:          p.PolicyID,
			Description: p.Desc,
			Subjects:    subjects,
			Resources:   resources,
			Actions:     actions,
			Effect:      Effect(p.Effect),
			Conditions:  conditions,
		})
	}

	return policies, nil
}

// DeletePolicy marks a policy as inactive (soft delete)
func (ps *PolicyStore) DeletePolicy(ctx context.Context, policyID string) error {
	// Find the policy UID
	query := `query FindPolicy($id: string) {
		p(func: type(Policy)) @filter(eq(policy_id, $id)) {
			uid
		}
	}`

	resp, err := ps.graphClient.Query(ctx, query, map[string]string{"$id": policyID})
	if err != nil {
		return fmt.Errorf("failed to find policy: %w", err)
	}

	var result struct {
		P []struct {
			UID string `json:"uid"`
		} `json:"p"`
	}
	json.Unmarshal(resp, &result)

	if len(result.P) == 0 {
		return fmt.Errorf("policy not found: %s", policyID)
	}

	// Soft delete by setting is_active to false
	nquad := fmt.Sprintf(`<%s> <is_active> "false"^^<xs:boolean> .`, result.P[0].UID)

	txn := ps.graphClient.GetDgraphClient().NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetNquads: []byte(nquad),
		CommitNow: true,
	}

	if _, err := txn.Mutate(ctx, mu); err != nil {
		return fmt.Errorf("failed to delete policy: %w", err)
	}

	ps.logger.Info("Soft deleted policy", zap.String("policy_id", policyID))
	return nil
}
