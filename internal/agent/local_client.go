package agent

import (
	"context"

	"github.com/reflective-memory-kernel/internal/graph"
	"github.com/reflective-memory-kernel/internal/kernel"
)

// LocalKernelClient implements MemoryKernelClient by wrapping a local Kernel instance.
// This provides Zero-Copy access for the unified binary.
type LocalKernelClient struct {
	k *kernel.Kernel
}

// NewLocalKernelClient creates a new local client wrapper
func NewLocalKernelClient(k *kernel.Kernel) *LocalKernelClient {
	return &LocalKernelClient{k: k}
}

func (c *LocalKernelClient) Consult(ctx context.Context, req *graph.ConsultationRequest) (*graph.ConsultationResponse, error) {
	return c.k.Consult(ctx, req)
}

func (c *LocalKernelClient) StoreInHotCache(ctx context.Context, userID, query, response, convID string) error {
	return c.k.StoreInHotCache(userID, query, response, convID)
}

func (c *LocalKernelClient) GetStats(ctx context.Context) (map[string]interface{}, error) {
	return c.k.GetStats(ctx)
}

func (c *LocalKernelClient) EnsureUserNode(ctx context.Context, username string) error {
	return c.k.EnsureUserNode(ctx, username)
}

func (c *LocalKernelClient) CreateGroup(ctx context.Context, name, description, ownerID string) (string, error) {
	return c.k.CreateGroup(ctx, name, description, ownerID)
}

func (c *LocalKernelClient) ListGroups(ctx context.Context, userID string) ([]map[string]interface{}, error) {
	groups, err := c.k.ListUserGroups(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Convert to map to match interface
	result := make([]map[string]interface{}, len(groups))
	for i, g := range groups {
		result[i] = map[string]interface{}{
			"uid":         g.UID,
			"name":        g.Name,
			"description": g.Description,
			"namespace":   g.Namespace,
			"created_at":  g.CreatedAt,
		}
	}
	return result, nil
}

func (c *LocalKernelClient) AddGroupMember(ctx context.Context, groupID, username string) error {
	return c.k.AddGroupMember(ctx, groupID, username)
}

func (c *LocalKernelClient) RemoveGroupMember(ctx context.Context, groupID, username string) error {
	return c.k.RemoveGroupMember(ctx, groupID, username)
}

func (c *LocalKernelClient) ShareToGroup(ctx context.Context, conversationID, groupID string) error {
	return c.k.ShareToGroup(ctx, conversationID, groupID)
}

func (c *LocalKernelClient) IsGroupAdmin(ctx context.Context, groupNamespace, userID string) (bool, error) {
	return c.k.IsGroupAdmin(ctx, groupNamespace, userID)
}

// ============================================================================
// Graph Traversal Methods - delegate to GraphClient
// ============================================================================

func (c *LocalKernelClient) FindNodeByName(ctx context.Context, name string, nodeType graph.NodeType) (*graph.Node, error) {
	return c.k.GetGraphClient().FindNodeByName(ctx, name, nodeType)
}

func (c *LocalKernelClient) SpreadActivation(ctx context.Context, opts graph.SpreadActivationOpts) ([]graph.ActivatedNode, error) {
	return c.k.GetGraphClient().SpreadActivation(ctx, opts)
}

func (c *LocalKernelClient) TraverseViaCommunity(ctx context.Context, opts graph.CommunityTraversalOpts) (*graph.CommunityResult, error) {
	return c.k.GetGraphClient().TraverseViaCommunity(ctx, opts)
}

func (c *LocalKernelClient) QueryWithTemporalDecay(ctx context.Context, opts graph.TemporalQueryOpts) ([]graph.RankedNode, error) {
	return c.k.GetGraphClient().QueryWithTemporalDecay(ctx, opts)
}

func (c *LocalKernelClient) ExpandFromNode(ctx context.Context, opts graph.ExpandOpts) (*graph.ExpandResult, error) {
	return c.k.GetGraphClient().ExpandFromNode(ctx, opts)
}

// TriggerReflection triggers a reflection cycle on the kernel
func (c *LocalKernelClient) TriggerReflection(ctx context.Context) error {
	return c.k.TriggerReflection(ctx)
}

// GetSampleNodes returns sample nodes from the graph for visualization
func (c *LocalKernelClient) GetSampleNodes(ctx context.Context, limit int) ([]graph.Node, error) {
	return c.k.GetGraphClient().GetSampleNodes(ctx, limit)
}
