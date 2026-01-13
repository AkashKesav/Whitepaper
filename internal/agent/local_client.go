package agent

import (
	"context"
	"time"

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

func (c *LocalKernelClient) StoreInHotCache(ctx context.Context, userID, namespace, query, response, convID string) error {
	return c.k.StoreInHotCache(userID, namespace, query, response, convID)
}

func (c *LocalKernelClient) GetStats(ctx context.Context) (map[string]interface{}, error) {
	return c.k.GetStats(ctx)
}

func (c *LocalKernelClient) EnsureUserNode(ctx context.Context, username, role string) error {
	return c.k.EnsureUserNode(ctx, username, role)
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

func (c *LocalKernelClient) FindNodeByName(ctx context.Context, namespace, name string, nodeType graph.NodeType) (*graph.Node, error) {
	return c.k.GetGraphClient().FindNodeByName(ctx, namespace, name, nodeType)
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
func (c *LocalKernelClient) GetSampleNodes(ctx context.Context, namespace string, limit int) ([]graph.Node, error) {
	return c.k.GetGraphClient().GetSampleNodes(ctx, namespace, limit)
}

// GetGraphClient returns the underlying graph client
func (c *LocalKernelClient) GetGraphClient() *graph.Client {
	return c.k.GetGraphClient()
}

// Speculate triggers a speculative context lookup
func (c *LocalKernelClient) Speculate(ctx context.Context, req *graph.ConsultationRequest) error {
	// Speculative lookup - just warm the cache, no response needed
	_, err := c.k.Consult(ctx, req)
	return err
}

// DeleteGroup deletes a group
func (c *LocalKernelClient) DeleteGroup(ctx context.Context, groupID, userID string) error {
	return c.k.DeleteGroup(ctx, groupID, userID)
}

// ============================================================================
// Workspace Collaboration Methods
// ============================================================================

// InviteToWorkspace invites a user to join a workspace
func (c *LocalKernelClient) InviteToWorkspace(ctx context.Context, workspaceNS, inviterID, inviteeUsername, role string) (*graph.WorkspaceInvitation, error) {
	return c.k.InviteToWorkspace(ctx, workspaceNS, inviterID, inviteeUsername, role)
}

// AcceptInvitation accepts a pending invitation
func (c *LocalKernelClient) AcceptInvitation(ctx context.Context, invitationUID, userID string) error {
	return c.k.AcceptInvitation(ctx, invitationUID, userID)
}

// DeclineInvitation declines a pending invitation
func (c *LocalKernelClient) DeclineInvitation(ctx context.Context, invitationUID, userID string) error {
	return c.k.DeclineInvitation(ctx, invitationUID, userID)
}

// GetPendingInvitations gets all pending invitations for a user
func (c *LocalKernelClient) GetPendingInvitations(ctx context.Context, userID string) ([]graph.WorkspaceInvitation, error) {
	return c.k.GetPendingInvitations(ctx, userID)
}

// GetWorkspaceSentInvitations gets all pending invitations sent by a workspace
func (c *LocalKernelClient) GetWorkspaceSentInvitations(ctx context.Context, workspaceNS string) ([]graph.WorkspaceInvitation, error) {
	return c.k.GetWorkspaceSentInvitations(ctx, workspaceNS)
}

// CreateShareLink creates a shareable link for a workspace
func (c *LocalKernelClient) CreateShareLink(ctx context.Context, workspaceNS, creatorID string, maxUses int, expiresAt *time.Time) (*graph.ShareLink, error) {
	return c.k.CreateShareLink(ctx, workspaceNS, creatorID, maxUses, expiresAt)
}

// JoinViaShareLink joins a workspace using a share link
func (c *LocalKernelClient) JoinViaShareLink(ctx context.Context, token, userID string) (*graph.ShareLink, error) {
	return c.k.JoinViaShareLink(ctx, token, userID)
}

// RevokeShareLink revokes a share link
func (c *LocalKernelClient) RevokeShareLink(ctx context.Context, token, userID string) error {
	return c.k.RevokeShareLink(ctx, token, userID)
}

// GetWorkspaceMembers gets all members of a workspace
func (c *LocalKernelClient) GetWorkspaceMembers(ctx context.Context, workspaceNS string) ([]graph.WorkspaceMember, error) {
	return c.k.GetWorkspaceMembers(ctx, workspaceNS)
}

// IsWorkspaceMember checks if a user is a member of a workspace
func (c *LocalKernelClient) IsWorkspaceMember(ctx context.Context, workspaceNS, userID string) (bool, error) {
	return c.k.IsWorkspaceMember(ctx, workspaceNS, userID)
}

// ============================================================================
// Ingestion Persistence Methods
// ============================================================================

// PersistEntities persists extracted entities to the graph
func (c *LocalKernelClient) PersistEntities(ctx context.Context, namespace, userID, conversationID string, entities []graph.ExtractedEntity) error {
	return c.k.PersistEntities(ctx, namespace, userID, conversationID, entities)
}

// PersistChunks persists document chunks to Qdrant
func (c *LocalKernelClient) PersistChunks(ctx context.Context, namespace, docID string, chunks []graph.DocumentChunk) error {
	return c.k.PersistChunks(ctx, namespace, docID, chunks)
}

// ============================================================================
// Search Methods
// ============================================================================

// SearchNodes searches for nodes matching a query string
func (c *LocalKernelClient) SearchNodes(ctx context.Context, namespace, query string) ([]graph.Node, error) {
	return c.k.GetGraphClient().SearchNodes(ctx, query, namespace)
}

// ListUserGroups lists groups the user is a member of
func (c *LocalKernelClient) ListUserGroups(ctx context.Context, userID string) ([]graph.Group, error) {
	return c.k.ListUserGroups(ctx, userID)
}
