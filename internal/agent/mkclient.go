// Package agent provides the Memory Kernel client for the Front-End Agent.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/graph"
)

// MemoryKernel defines the interface for direct (zero-copy) usage
type MemoryKernel interface {
	Consult(ctx context.Context, req *graph.ConsultationRequest) (*graph.ConsultationResponse, error)
	CreateGroup(ctx context.Context, name, description, ownerID string) (string, error)
	ListUserGroups(ctx context.Context, userID string) ([]graph.Group, error)
	IsGroupAdmin(ctx context.Context, groupNamespace, userID string) (bool, error)
	AddGroupMember(ctx context.Context, groupID, username string) error
	RemoveGroupMember(ctx context.Context, groupID, username string) error
	DeleteGroup(ctx context.Context, groupID string) error
	ShareToGroup(ctx context.Context, conversationID, groupID string) error
	EnsureUserNode(ctx context.Context, username string) error
	GetStats(ctx context.Context) (map[string]interface{}, error)
	Speculate(ctx context.Context, req *graph.ConsultationRequest) error

	// Workspace Collaboration Methods
	InviteToWorkspace(ctx context.Context, workspaceNS, inviterID, inviteeUsername, role string) (*graph.WorkspaceInvitation, error)
	AcceptInvitation(ctx context.Context, invitationUID, userID string) error
	DeclineInvitation(ctx context.Context, invitationUID, userID string) error
	GetPendingInvitations(ctx context.Context, userID string) ([]graph.WorkspaceInvitation, error)
	CreateShareLink(ctx context.Context, workspaceNS, creatorID string, maxUses int, expiresAt *time.Time) (*graph.ShareLink, error)
	JoinViaShareLink(ctx context.Context, token, userID string) (*graph.ShareLink, error)
	RevokeShareLink(ctx context.Context, token, userID string) error
	GetWorkspaceMembers(ctx context.Context, workspaceNS string) ([]graph.WorkspaceMember, error)
	IsWorkspaceMember(ctx context.Context, workspaceNS, userID string) (bool, error)

	// Graph traversal methods
	GetGraphClient() *graph.Client

	// Admin methods
	TriggerReflection(ctx context.Context) error
}

// MKClient is a client for consulting the Memory Kernel
type MKClient struct {
	baseURL      string
	httpClient   *http.Client
	logger       *zap.Logger
	directKernel MemoryKernel // Zero-copy interface
}

// NewMKClient creates a new Memory Kernel client
func NewMKClient(baseURL string, logger *zap.Logger) *MKClient {
	return &MKClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // Increased for group operations
		},
		logger: logger,
	}
}

// SetDirectKernel enables zero-copy mode
func (c *MKClient) SetDirectKernel(k MemoryKernel) {
	c.directKernel = k
	c.logger.Info("MKClient: Direct Kernel access enabled (Zero-Copy)")
}

// Consult sends a consultation request to the Memory Kernel
func (c *MKClient) Consult(ctx context.Context, req *graph.ConsultationRequest) (*graph.ConsultationResponse, error) {
	// Zero-Copy Path
	if c.directKernel != nil {
		return c.directKernel.Consult(ctx, req)
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/api/consult",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MK returned status %d", resp.StatusCode)
	}

	var response graph.ConsultationResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

// Speculate triggers a speculative context lookup (Zero-Copy only for now)
func (c *MKClient) Speculate(ctx context.Context, req *graph.ConsultationRequest) error {
	if c.directKernel != nil {
		return c.directKernel.Speculate(ctx, req)
	}
	// For HTTP mode, we could add an endpoint, but skipping for MVP
	return nil
}

// GetStats retrieves Memory Kernel statistics
func (c *MKClient) GetStats(ctx context.Context) (map[string]interface{}, error) {
	if c.directKernel != nil {
		return c.directKernel.GetStats(ctx)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/stats", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var stats map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, err
	}

	return stats, nil
}

// CreateGroup creates a new group
func (c *MKClient) CreateGroup(ctx context.Context, name, description, ownerID string) (string, error) {
	if c.directKernel != nil {
		return c.directKernel.CreateGroup(ctx, name, description, ownerID)
	}

	payload := map[string]string{
		"name":        name,
		"description": description,
		"owner_id":    ownerID,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/groups", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("MK returned status %d", resp.StatusCode)
	}

	var result struct {
		GroupID string `json:"group_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.GroupID, nil
}

// ListGroups lists groups the user is a member of
func (c *MKClient) ListGroups(ctx context.Context, userID string) ([]map[string]interface{}, error) {
	if c.directKernel != nil {
		// Convert []graph.Group to []map[string]interface{} for compatibility
		// Or update interface to return []graph.Group if feasible, but keep signature for now.
		groups, err := c.directKernel.ListUserGroups(ctx, userID)
		if err != nil {
			return nil, err
		}
		// Conversion: serialized JSON re-unmarshal or manual map
		// Fast path: json marshal/unmarshal to map (lazy but works for now)
		data, _ := json.Marshal(groups)
		var res []map[string]interface{}
		json.Unmarshal(data, &res)
		return res, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/groups?user="+userID, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var groups []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
		return nil, err
	}

	return groups, nil
}

// AddGroupMember adds a member to a group
func (c *MKClient) AddGroupMember(ctx context.Context, groupID, username string) error {
	if c.directKernel != nil {
		return c.directKernel.AddGroupMember(ctx, groupID, username)
	}

	payload := map[string]string{
		"group_id": groupID,
		"username": username,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/groups/members", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MK returned status %d", resp.StatusCode)
	}

	return nil
}

// RemoveGroupMember removes a member from a group
func (c *MKClient) RemoveGroupMember(ctx context.Context, groupID, username string) error {
	if c.directKernel != nil {
		return c.directKernel.RemoveGroupMember(ctx, groupID, username)
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"/api/groups/"+groupID+"/members/"+username, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MK returned status %d", resp.StatusCode)
	}

	return nil
}

// ShareToGroup shares a conversation with a group
func (c *MKClient) ShareToGroup(ctx context.Context, conversationID, groupID string) error {
	if c.directKernel != nil {
		return c.directKernel.ShareToGroup(ctx, conversationID, groupID)
	}

	payload := map[string]string{
		"conversation_id": conversationID,
		"group_id":        groupID,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/groups/share", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MK returned status %d", resp.StatusCode)
	}

	return nil
}

// DeleteGroup deletes a group
func (c *MKClient) DeleteGroup(ctx context.Context, groupID string) error {
	if c.directKernel != nil {
		return c.directKernel.DeleteGroup(ctx, groupID)
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"/api/groups/"+groupID, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MK returned status %d", resp.StatusCode)
	}

	return nil
}

// IsGroupAdmin checks if a user is an admin of a group
func (c *MKClient) IsGroupAdmin(ctx context.Context, groupNamespace, userID string) (bool, error) {
	if c.directKernel != nil {
		return c.directKernel.IsGroupAdmin(ctx, groupNamespace, userID)
	}

	payload := map[string]string{
		"group_namespace": groupNamespace,
		"user_id":         userID,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/groups/is-admin", bytes.NewBuffer(jsonData))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("MK returned status %d", resp.StatusCode)
	}

	var result struct {
		IsAdmin bool `json:"is_admin"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	return result.IsAdmin, nil
}

// EnsureUserNode creates a User node in DGraph if it doesn't exist
func (c *MKClient) EnsureUserNode(ctx context.Context, username string) error {
	if c.directKernel != nil {
		return c.directKernel.EnsureUserNode(ctx, username)
	}

	payload := map[string]string{"username": username}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/ensure-user", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MK returned status %d", resp.StatusCode)
	}

	return nil
}

// ============================================================================
// WORKSPACE COLLABORATION WRAPPER METHODS
// ============================================================================

// InviteToWorkspace invites a user to join a workspace
func (c *MKClient) InviteToWorkspace(ctx context.Context, workspaceNS, inviterID, inviteeUsername, role string) (*graph.WorkspaceInvitation, error) {
	if c.directKernel != nil {
		return c.directKernel.InviteToWorkspace(ctx, workspaceNS, inviterID, inviteeUsername, role)
	}
	return nil, fmt.Errorf("HTTP mode not supported for InviteToWorkspace")
}

// AcceptInvitation accepts a pending invitation
func (c *MKClient) AcceptInvitation(ctx context.Context, invitationUID, userID string) error {
	if c.directKernel != nil {
		return c.directKernel.AcceptInvitation(ctx, invitationUID, userID)
	}
	return fmt.Errorf("HTTP mode not supported for AcceptInvitation")
}

// DeclineInvitation declines a pending invitation
func (c *MKClient) DeclineInvitation(ctx context.Context, invitationUID, userID string) error {
	if c.directKernel != nil {
		return c.directKernel.DeclineInvitation(ctx, invitationUID, userID)
	}
	return fmt.Errorf("HTTP mode not supported for DeclineInvitation")
}

// GetPendingInvitations gets all pending invitations for a user
func (c *MKClient) GetPendingInvitations(ctx context.Context, userID string) ([]graph.WorkspaceInvitation, error) {
	if c.directKernel != nil {
		return c.directKernel.GetPendingInvitations(ctx, userID)
	}
	return nil, fmt.Errorf("HTTP mode not supported for GetPendingInvitations")
}

// CreateShareLink creates a shareable link for a workspace
func (c *MKClient) CreateShareLink(ctx context.Context, workspaceNS, creatorID string, maxUses int, expiresAt *time.Time) (*graph.ShareLink, error) {
	if c.directKernel != nil {
		return c.directKernel.CreateShareLink(ctx, workspaceNS, creatorID, maxUses, expiresAt)
	}
	return nil, fmt.Errorf("HTTP mode not supported for CreateShareLink")
}

// JoinViaShareLink joins a workspace using a share link
func (c *MKClient) JoinViaShareLink(ctx context.Context, token, userID string) (*graph.ShareLink, error) {
	if c.directKernel != nil {
		return c.directKernel.JoinViaShareLink(ctx, token, userID)
	}
	return nil, fmt.Errorf("HTTP mode not supported for JoinViaShareLink")
}

// RevokeShareLink revokes a share link
func (c *MKClient) RevokeShareLink(ctx context.Context, token, userID string) error {
	if c.directKernel != nil {
		return c.directKernel.RevokeShareLink(ctx, token, userID)
	}
	return fmt.Errorf("HTTP mode not supported for RevokeShareLink")
}

// GetWorkspaceMembers gets all members of a workspace
func (c *MKClient) GetWorkspaceMembers(ctx context.Context, workspaceNS string) ([]graph.WorkspaceMember, error) {
	if c.directKernel != nil {
		return c.directKernel.GetWorkspaceMembers(ctx, workspaceNS)
	}
	return nil, fmt.Errorf("HTTP mode not supported for GetWorkspaceMembers")
}

// IsWorkspaceMember checks if a user is a member of a workspace
func (c *MKClient) IsWorkspaceMember(ctx context.Context, workspaceNS, userID string) (bool, error) {
	if c.directKernel != nil {
		return c.directKernel.IsWorkspaceMember(ctx, workspaceNS, userID)
	}
	return false, fmt.Errorf("HTTP mode not supported for IsWorkspaceMember")
}

// ============================================================================
// GRAPH TRAVERSAL METHODS (Zero-Copy only - requires directKernel)
// ============================================================================

// FindNodeByName finds a node by its name and type
func (c *MKClient) FindNodeByName(ctx context.Context, name string, nodeType graph.NodeType) (*graph.Node, error) {
	if c.directKernel != nil {
		return c.directKernel.GetGraphClient().FindNodeByName(ctx, name, nodeType)
	}
	return nil, fmt.Errorf("HTTP mode not supported for FindNodeByName")
}

// SpreadActivation performs spreading activation from a start node
func (c *MKClient) SpreadActivation(ctx context.Context, opts graph.SpreadActivationOpts) ([]graph.ActivatedNode, error) {
	if c.directKernel != nil {
		return c.directKernel.GetGraphClient().SpreadActivation(ctx, opts)
	}
	return nil, fmt.Errorf("HTTP mode not supported for SpreadActivation")
}

// TraverseViaCommunity traverses the graph via community detection
func (c *MKClient) TraverseViaCommunity(ctx context.Context, opts graph.CommunityTraversalOpts) (*graph.CommunityResult, error) {
	if c.directKernel != nil {
		return c.directKernel.GetGraphClient().TraverseViaCommunity(ctx, opts)
	}
	return nil, fmt.Errorf("HTTP mode not supported for TraverseViaCommunity")
}

// QueryWithTemporalDecay queries with temporal decay applied
func (c *MKClient) QueryWithTemporalDecay(ctx context.Context, opts graph.TemporalQueryOpts) ([]graph.RankedNode, error) {
	if c.directKernel != nil {
		return c.directKernel.GetGraphClient().QueryWithTemporalDecay(ctx, opts)
	}
	return nil, fmt.Errorf("HTTP mode not supported for QueryWithTemporalDecay")
}

// ExpandFromNode expands outward from a starting node
func (c *MKClient) ExpandFromNode(ctx context.Context, opts graph.ExpandOpts) (*graph.ExpandResult, error) {
	if c.directKernel != nil {
		return c.directKernel.GetGraphClient().ExpandFromNode(ctx, opts)
	}
	return nil, fmt.Errorf("HTTP mode not supported for ExpandFromNode")
}

// GetSampleNodes returns sample nodes for visualization
func (c *MKClient) GetSampleNodes(ctx context.Context, limit int) ([]graph.Node, error) {
	if c.directKernel != nil {
		return c.directKernel.GetGraphClient().GetSampleNodes(ctx, limit)
	}
	return nil, fmt.Errorf("HTTP mode not supported for GetSampleNodes")
}

// ============================================================================
// ADMIN METHODS
// ============================================================================

// TriggerReflection triggers a reflection cycle on the kernel
func (c *MKClient) TriggerReflection(ctx context.Context) error {
	if c.directKernel != nil {
		return c.directKernel.TriggerReflection(ctx)
	}
	return fmt.Errorf("HTTP mode not supported for TriggerReflection")
}

// AIClient is a client for AI services
type AIClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewAIClient creates a new AI service client
func NewAIClient(baseURL string, logger *zap.Logger) *AIClient {
	return &AIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		logger: logger,
	}
}

// GenerateResponse generates a conversational response
func (c *AIClient) GenerateResponse(ctx context.Context, query, context string, alerts []string) (string, error) {
	type GenerateRequest struct {
		Query           string   `json:"query"`
		Context         string   `json:"context,omitempty"`
		ProactiveAlerts []string `json:"proactive_alerts,omitempty"`
	}

	reqBody := GenerateRequest{
		Query:           query,
		Context:         context,
		ProactiveAlerts: alerts,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/generate",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI service returned status %d", resp.StatusCode)
	}

	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Response, nil
}
