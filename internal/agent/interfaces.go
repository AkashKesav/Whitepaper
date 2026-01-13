package agent

import (
	"context"

	"github.com/reflective-memory-kernel/internal/graph"
)

// MemoryKernelClient defines the interface for interacting with the Memory Kernel.
// This allows switching between HTTP-based (Remote) and direct (Local) communication.
type MemoryKernelClient interface {
	// core operations
	Consult(ctx context.Context, req *graph.ConsultationRequest) (*graph.ConsultationResponse, error)
	StoreInHotCache(ctx context.Context, userID, namespace, query, response, convID string) error
	GetStats(ctx context.Context) (map[string]interface{}, error)
	EnsureUserNode(ctx context.Context, username string) error

	// group operations
	CreateGroup(ctx context.Context, name, description, ownerID string) (string, error)
	ListGroups(ctx context.Context, userID string) ([]map[string]interface{}, error)
	AddGroupMember(ctx context.Context, groupID, username string) error
	RemoveGroupMember(ctx context.Context, groupID, username string) error
	ShareToGroup(ctx context.Context, conversationID, groupID string) error
	IsGroupAdmin(ctx context.Context, groupNamespace, userID string) (bool, error)

	// graph traversal operations
	FindNodeByName(ctx context.Context, name string, nodeType graph.NodeType) (*graph.Node, error)
	SpreadActivation(ctx context.Context, opts graph.SpreadActivationOpts) ([]graph.ActivatedNode, error)
	TraverseViaCommunity(ctx context.Context, opts graph.CommunityTraversalOpts) (*graph.CommunityResult, error)
	QueryWithTemporalDecay(ctx context.Context, opts graph.TemporalQueryOpts) ([]graph.RankedNode, error)
	ExpandFromNode(ctx context.Context, opts graph.ExpandOpts) (*graph.ExpandResult, error)
	GetSampleNodes(ctx context.Context, limit int) ([]graph.Node, error)

	// admin operations
	TriggerReflection(ctx context.Context) error
}
