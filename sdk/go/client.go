// Package rmk provides the Go SDK for the Reflective Memory Kernel
package rmk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the RMK Go client
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// ClientConfig configures the RMK client
type ClientConfig struct {
	BaseURL   string
	Timeout   time.Duration
	AuthToken string
}

// NewClient creates a new RMK client
func NewClient(config ClientConfig) *Client {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		baseURL: config.BaseURL,
		token:   config.AuthToken,
	}
}

// SetToken sets the authentication token
func (c *Client) SetToken(token string) {
	c.token = token
}

// GetToken returns the current token
func (c *Client) GetToken() string {
	return c.token
}

// Login authenticates with username and password
func (c *Client) Login(ctx context.Context, username, password string) (*AuthResponse, error) {
	req := LoginRequest{
		Username: username,
		Password: password,
	}

	var resp AuthResponse
	if err := c.post(ctx, "/api/login", req, &resp); err != nil {
		return nil, err
	}

	c.token = resp.Token
	return &resp, nil
}

// Logout clears the authentication token
func (c *Client) Logout(ctx context.Context) error {
	err := c.post(ctx, "/api/logout", nil, nil)
	c.token = ""
	return err
}

// MemoryStore stores a memory in the knowledge graph
func (c *Client) MemoryStore(ctx context.Context, req *MemoryStoreRequest) (*MemoryStoreResponse, error) {
	var resp MemoryStoreResponse
	if err := c.toolCall(ctx, "memory_store", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// MemorySearch searches memories
func (c *Client) MemorySearch(ctx context.Context, req *MemorySearchRequest) (*MemorySearchResponse, error) {
	var resp MemorySearchResponse
	if err := c.toolCall(ctx, "memory_search", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// MemoryDelete deletes a memory
func (c *Client) MemoryDelete(ctx context.Context, req *MemoryDeleteRequest) error {
	return c.toolCall(ctx, "memory_delete", req, nil)
}

// MemoryList lists memories
func (c *Client) MemoryList(ctx context.Context, req *MemoryListRequest) (*MemoryListResponse, error) {
	var resp MemoryListResponse
	if err := c.toolCall(ctx, "memory_list", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ChatConsult performs a chat consultation
func (c *Client) ChatConsult(ctx context.Context, req *ChatConsultRequest) (*ChatConsultResponse, error) {
	var resp ChatConsultResponse
	if err := c.toolCall(ctx, "chat_consult", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ConversationsList lists conversations
func (c *Client) ConversationsList(ctx context.Context, req *ConversationsListRequest) (*ConversationsListResponse, error) {
	var resp ConversationsListResponse
	if err := c.toolCall(ctx, "conversations_list", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ConversationsDelete deletes a conversation
func (c *Client) ConversationsDelete(ctx context.Context, req *ConversationsDeleteRequest) error {
	return c.toolCall(ctx, "conversations_delete", req, nil)
}

// EntityCreate creates an entity
func (c *Client) EntityCreate(ctx context.Context, req *EntityCreateRequest) (*EntityCreateResponse, error) {
	var resp EntityCreateResponse
	if err := c.toolCall(ctx, "entity_create", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// EntityUpdate updates an entity
func (c *Client) EntityUpdate(ctx context.Context, req *EntityUpdateRequest) (*EntityUpdateResponse, error) {
	var resp EntityUpdateResponse
	if err := c.toolCall(ctx, "entity_update", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// EntityQuery queries entities
func (c *Client) EntityQuery(ctx context.Context, req *EntityQueryRequest) (*EntityQueryResponse, error) {
	var resp EntityQueryResponse
	if err := c.toolCall(ctx, "entity_query", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RelationshipCreate creates a relationship
func (c *Client) RelationshipCreate(ctx context.Context, req *RelationshipCreateRequest) (*RelationshipCreateResponse, error) {
	var resp RelationshipCreateResponse
	if err := c.toolCall(ctx, "relationship_create", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DocumentIngest ingests a document
func (c *Client) DocumentIngest(ctx context.Context, req *DocumentIngestRequest) (*DocumentIngestResponse, error) {
	var resp DocumentIngestResponse
	if err := c.toolCall(ctx, "document_ingest", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DocumentList lists documents
func (c *Client) DocumentList(ctx context.Context, req *DocumentListRequest) (*DocumentListResponse, error) {
	var resp DocumentListResponse
	if err := c.toolCall(ctx, "document_list", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DocumentDelete deletes a document
func (c *Client) DocumentDelete(ctx context.Context, req *DocumentDeleteRequest) error {
	return c.toolCall(ctx, "document_delete", req, nil)
}

// GroupCreate creates a group
func (c *Client) GroupCreate(ctx context.Context, req *GroupCreateRequest) (*GroupCreateResponse, error) {
	var resp GroupCreateResponse
	if err := c.toolCall(ctx, "group_create", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GroupList lists groups
func (c *Client) GroupList(ctx context.Context) (*GroupListResponse, error) {
	var resp GroupListResponse
	if err := c.toolCall(ctx, "group_list", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GroupInvite invites a user to a group
func (c *Client) GroupInvite(ctx context.Context, req *GroupInviteRequest) (*GroupInviteResponse, error) {
	var resp GroupInviteResponse
	if err := c.toolCall(ctx, "group_invite", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GroupMembers lists group members
func (c *Client) GroupMembers(ctx context.Context, req *GroupMembersRequest) (*GroupMembersResponse, error) {
	var resp GroupMembersResponse
	if err := c.toolCall(ctx, "group_members", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GroupShareLink creates a share link
func (c *Client) GroupShareLink(ctx context.Context, req *GroupShareLinkRequest) (*GroupShareLinkResponse, error) {
	var resp GroupShareLinkResponse
	if err := c.toolCall(ctx, "group_share_link", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AdminUsersList lists all users (admin only)
func (c *Client) AdminUsersList(ctx context.Context) (*AdminUsersListResponse, error) {
	var resp AdminUsersListResponse
	if err := c.toolCall(ctx, "admin_users_list", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AdminUserUpdate updates a user (admin only)
func (c *Client) AdminUserUpdate(ctx context.Context, req *AdminUserUpdateRequest) (*AdminUserUpdateResponse, error) {
	var resp AdminUserUpdateResponse
	if err := c.toolCall(ctx, "admin_user_update", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AdminMetrics gets system metrics (admin only)
func (c *Client) AdminMetrics(ctx context.Context) (*AdminMetricsResponse, error) {
	var resp AdminMetricsResponse
	if err := c.toolCall(ctx, "admin_metrics", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AdminPoliciesList lists policies (admin only)
func (c *Client) AdminPoliciesList(ctx context.Context) (*AdminPoliciesListResponse, error) {
	var resp AdminPoliciesListResponse
	if err := c.toolCall(ctx, "admin_policies_list", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AdminPoliciesSet creates or updates a policy (admin only)
func (c *Client) AdminPoliciesSet(ctx context.Context, req *AdminPoliciesSetRequest) (*AdminPoliciesSetResponse, error) {
	var resp AdminPoliciesSetResponse
	if err := c.toolCall(ctx, "admin_policies_set", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ToolsList lists all available MCP tools
func (c *Client) ToolsList(ctx context.Context) (*ToolsListResponse, error) {
	var resp ToolsListResponse
	if err := c.get(ctx, "/api/mcp/tools", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// toolCall calls an MCP tool
func (c *Client) toolCall(ctx context.Context, name string, args, resp interface{}) error {
	var arguments map[string]interface{}
	if args != nil {
		data, err := json.Marshal(args)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &arguments); err != nil {
			return err
		}
	}

	toolReq := ToolCallRequest{
		Name:      name,
		Arguments: arguments,
	}

	var toolResp ToolCallResponse
	if err := c.post(ctx, "/api/mcp/tools/call", toolReq, &toolResp); err != nil {
		return err
	}

	if toolResp.IsError {
		return fmt.Errorf("tool %s failed: %s", name, toolResp.Content[0].Text)
	}

	// Parse the text content as JSON
	if resp != nil && len(toolResp.Content) > 0 {
		if err := json.Unmarshal([]byte(toolResp.Content[0].Text), resp); err != nil {
			return fmt.Errorf("failed to parse tool response: %w", err)
		}
	}

	return nil
}

// post makes a POST request
func (c *Client) post(ctx context.Context, path string, body, resp interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		data, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, string(data))
	}

	if resp != nil {
		return json.NewDecoder(httpResp.Body).Decode(resp)
	}

	return nil
}

// get makes a GET request
func (c *Client) get(ctx context.Context, path string, resp interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return err
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		data, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, string(data))
	}

	return json.NewDecoder(httpResp.Body).Decode(resp)
}
