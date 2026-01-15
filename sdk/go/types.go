// Package rmk provides types for the RMK Go SDK
package rmk

// NodeType is the type of a memory node
type NodeType string

const (
	NodeTypeEntity   NodeType = "Entity"
	NodeTypeFact     NodeType = "Fact"
	NodeTypeEvent    NodeType = "Event"
	NodeTypeInsight  NodeType = "Insight"
	NodeTypePattern  NodeType = "Pattern"
)

// RelationshipType is the type of relationship between entities
type RelationshipType string

const (
	RelationshipKnows       RelationshipType = "KNOWS"
	RelationshipLikes       RelationshipType = "LIKES"
	RelationshipWorksAt     RelationshipType = "WORKS_AT"
	RelationshipWorksOn     RelationshipType = "WORKS_ON"
	RelationshipFriendOf    RelationshipType = "FRIEND_OF"
	RelationshipRelatedTo   RelationshipType = "RELATED_TO"
	RelationshipPartOf      RelationshipType = "PART_OF"
)

// UserRole is a user role
type UserRole string

const (
	UserRoleUser  UserRole = "user"
	UserRoleAdmin UserRole = "admin"
)

// AdminAction is an admin action
type AdminAction string

const (
	AdminActionPromote AdminAction = "promote"
	AdminActionDemote  AdminAction = "demote"
	AdminActionDelete  AdminAction = "delete"
)

// PolicyEffect is the effect of a policy
type PolicyEffect string

const (
	PolicyEffectAllow PolicyEffect = "ALLOW"
	PolicyEffectDeny  PolicyEffect = "DENY"
)

// ========== AUTH TYPES ==========

// LoginRequest is a login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResponse is an authentication response
type AuthResponse struct {
	Token    string   `json:"token"`
	Username string   `json:"username"`
	Role     string   `json:"role"`
	Groups   []string `json:"groups,omitempty"`
}

// ========== MEMORY TYPES ==========

// MemoryStoreRequest is a memory store request
type MemoryStoreRequest struct {
	Namespace string   `json:"namespace"`
	Content   string   `json:"content"`
	NodeType  NodeType `json:"node_type"`
	Name      string   `json:"name,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

// MemoryStoreResponse is a memory store response
type MemoryStoreResponse struct {
	UID       string   `json:"uid"`
	NodeType  string   `json:"node_type"`
	Namespace string   `json:"namespace"`
	Name      string   `json:"name,omitempty"`
}

// MemorySearchRequest is a memory search request
type MemorySearchRequest struct {
	Namespace string `json:"namespace"`
	Query     string `json:"query"`
	Limit     int    `json:"limit,omitempty"`
}

// MemoryNode is a memory node
type MemoryNode struct {
	UID         string   `json:"uid"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	NodeType    string   `json:"node_type,omitempty"`
	Activation  float64  `json:"activation,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// MemorySearchResponse is a memory search response
type MemorySearchResponse struct {
	Results []MemoryNode `json:"results"`
	Count   int          `json:"count"`
}

// MemoryDeleteRequest is a memory delete request
type MemoryDeleteRequest struct {
	Namespace string `json:"namespace"`
	UID       string `json:"uid"`
}

// MemoryListRequest is a memory list request
type MemoryListRequest struct {
	Namespace string   `json:"namespace"`
	NodeType  NodeType `json:"node_type,omitempty"`
	Limit     int      `json:"limit,omitempty"`
	Offset    int      `json:"offset,omitempty"`
}

// MemoryListResponse is a memory list response
type MemoryListResponse struct {
	Results []interface{} `json:"results"`
	Total   int           `json:"total"`
	Offset  int           `json:"offset"`
	Limit   int           `json:"limit"`
}

// ========== CHAT TYPES ==========

// ChatConsultRequest is a chat consult request
type ChatConsultRequest struct {
	Namespace      string `json:"namespace"`
	Message        string `json:"message"`
	ConversationID string `json:"conversation_id,omitempty"`
}

// ChatConsultResponse is a chat consult response
type ChatConsultResponse struct {
	Response       string `json:"response"`
	ConversationID string `json:"conversation_id"`
	Namespace      string `json:"namespace"`
}

// ConversationsListRequest is a conversations list request
type ConversationsListRequest struct {
	Namespace string `json:"namespace"`
	Limit     int    `json:"limit,omitempty"`
}

// Conversation is a conversation
type Conversation struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// ConversationsListResponse is a conversations list response
type ConversationsListResponse struct {
	Conversations []Conversation `json:"conversations"`
	Count         int            `json:"count"`
}

// ConversationsDeleteRequest is a conversations delete request
type ConversationsDeleteRequest struct {
	Namespace      string `json:"namespace"`
	ConversationID string `json:"conversation_id"`
}

// ========== ENTITY TYPES ==========

// Relationship is a relationship between entities
type Relationship struct {
	Type   RelationshipType `json:"type"`
	Target string           `json:"target"`
}

// EntityCreateRequest is an entity create request
type EntityCreateRequest struct {
	Namespace      string                    `json:"namespace"`
	Name           string                    `json:"name"`
	EntityType     string                    `json:"entity_type"`
	Description    string                    `json:"description,omitempty"`
	Relationships  []Relationship            `json:"relationships,omitempty"`
	Attributes     map[string]string         `json:"attributes,omitempty"`
}

// EntityCreateResponse is an entity create response
type EntityCreateResponse struct {
	UID  string `json:"uid"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// EntityUpdateRequest is an entity update request
type EntityUpdateRequest struct {
	Namespace  string            `json:"namespace"`
	UID        string            `json:"uid"`
	Name       string            `json:"name,omitempty"`
	Description string           `json:"description,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// EntityUpdateResponse is an entity update response
type EntityUpdateResponse struct {
	UID    string `json:"uid"`
	Status string `json:"status"`
}

// EntityQueryRequest is an entity query request
type EntityQueryRequest struct {
	Namespace  string `json:"namespace"`
	EntityType string `json:"entity_type,omitempty"`
	Query      string `json:"query,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

// Entity is an entity
type Entity struct {
	UID         string  `json:"uid"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	EntityType  string  `json:"entity_type,omitempty"`
	Activation  float64 `json:"activation,omitempty"`
}

// EntityQueryResponse is an entity query response
type EntityQueryResponse struct {
	Entities []Entity `json:"entities"`
	Count    int      `json:"count"`
}

// RelationshipCreateRequest is a relationship create request
type RelationshipCreateRequest struct {
	Namespace        string           `json:"namespace"`
	FromUID          string           `json:"from_uid"`
	ToUID            string           `json:"to_uid"`
	RelationshipType RelationshipType `json:"relationship_type"`
}

// RelationshipCreateResponse is a relationship create response
type RelationshipCreateResponse struct {
	Status   string `json:"status"`
	FromUID  string `json:"from_uid"`
	ToUID    string `json:"to_uid"`
	RelType  string `json:"rel_type"`
}

// ========== DOCUMENT TYPES ==========

// DocumentIngestRequest is a document ingest request
type DocumentIngestRequest struct {
	Namespace    string `json:"namespace"`
	Content      string `json:"content"`
	Filename     string `json:"filename"`
	DocumentType string `json:"document_type,omitempty"`
}

// DocumentIngestResponse is a document ingest response
type DocumentIngestResponse struct {
	Status            string `json:"status"`
	Filename          string `json:"filename"`
	DocumentID        string `json:"document_id"`
	EntitiesExtracted int    `json:"entities_extracted"`
}

// DocumentListRequest is a document list request
type DocumentListRequest struct {
	Namespace string `json:"namespace"`
	Limit     int    `json:"limit,omitempty"`
}

// Document is a document
type Document struct {
	UID         string `json:"uid"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// DocumentListResponse is a document list response
type DocumentListResponse struct {
	Documents []Document `json:"documents"`
	Count     int        `json:"count"`
}

// DocumentDeleteRequest is a document delete request
type DocumentDeleteRequest struct {
	Namespace  string `json:"namespace"`
	DocumentID string `json:"document_id"`
}

// ========== GROUP TYPES ==========

// GroupCreateRequest is a group create request
type GroupCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// GroupCreateResponse is a group create response
type GroupCreateResponse struct {
	GroupID   string `json:"group_id"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// Group is a group
type Group struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	AdminID   string `json:"admin_id"`
	CreatedAt string `json:"created_at,omitempty"`
}

// GroupListResponse is a group list response
type GroupListResponse struct {
	Groups []Group `json:"groups"`
	Count  int     `json:"count"`
}

// GroupInviteRequest is a group invite request
type GroupInviteRequest struct {
	GroupID  string `json:"group_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// GroupInviteResponse is a group invite response
type GroupInviteResponse struct {
	InvitationID string `json:"invitation_id"`
	Status       string `json:"status"`
}

// GroupMembersRequest is a group members request
type GroupMembersRequest struct {
	GroupID string `json:"group_id"`
}

// GroupMember is a group member
type GroupMember struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// GroupMembersResponse is a group members response
type GroupMembersResponse struct {
	GroupID string       `json:"group_id"`
	Members []GroupMember `json:"members"`
	Count   int          `json:"count"`
}

// GroupShareLinkRequest is a group share link request
type GroupShareLinkRequest struct {
	GroupID       string `json:"group_id"`
	MaxUses       int    `json:"max_uses,omitempty"`
	ExpiresInHours int    `json:"expires_in_hours,omitempty"`
}

// GroupShareLinkResponse is a group share link response
type GroupShareLinkResponse struct {
	Token string `json:"token"`
	URL   string `json:"url"`
}

// ========== ADMIN TYPES ==========

// AdminUser is an admin user
type AdminUser struct {
	Username  string `json:"username"`
	Role      UserRole `json:"role"`
	CreatedAt string `json:"created_at,omitempty"`
}

// AdminUsersListResponse is an admin users list response
type AdminUsersListResponse struct {
	Users []AdminUser `json:"users"`
	Count int         `json:"count"`
}

// AdminUserUpdateRequest is an admin user update request
type AdminUserUpdateRequest struct {
	Username string       `json:"username"`
	Role     UserRole     `json:"role,omitempty"`
	Action   AdminAction  `json:"action"`
}

// AdminUserUpdateResponse is an admin user update response
type AdminUserUpdateResponse struct {
	Status   string   `json:"status"`
	Username string   `json:"username"`
	Role     string   `json:"role,omitempty"`
}

// AdminMetricsResponse is an admin metrics response
type AdminMetricsResponse struct {
	TotalUsers        int     `json:"total_users,omitempty"`
	TotalGroups       int     `json:"total_groups,omitempty"`
	TotalNodes        int     `json:"total_nodes,omitempty"`
	TotalConversations int    `json:"total_conversations,omitempty"`
	UptimeSeconds     float64 `json:"uptime_seconds,omitempty"`
}

// Policy is a policy
type Policy struct {
	ID          string   `json:"id"`
	Description string   `json:"description,omitempty"`
	Effect      PolicyEffect `json:"effect"`
	Subjects    []string `json:"subjects"`
	Resources   []string `json:"resources"`
	Actions     []string `json:"actions"`
}

// AdminPoliciesListResponse is an admin policies list response
type AdminPoliciesListResponse struct {
	Policies []Policy `json:"policies"`
	Count    int      `json:"count"`
}

// AdminPoliciesSetRequest is an admin policies set request
type AdminPoliciesSetRequest struct {
	ID          string       `json:"id"`
	Description string       `json:"description,omitempty"`
	Effect      PolicyEffect `json:"effect"`
	Subjects    []string     `json:"subjects"`
	Resources   []string     `json:"resources"`
	Actions     []string     `json:"actions"`
}

// AdminPoliciesSetResponse is an admin policies set response
type AdminPoliciesSetResponse struct {
	Status string       `json:"status"`
	ID     string       `json:"id"`
	Effect PolicyEffect `json:"effect"`
}

// ========== MCP TYPES ==========

// ToolCallRequest is an MCP tool call request
type ToolCallRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolContent is MCP tool content
type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ToolCallResponse is an MCP tool call response
type ToolCallResponse struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
	Meta    map[string]interface{} `json:"_meta,omitempty"`
}

// Tool is an MCP tool
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolsListResponse is a tools list response
type ToolsListResponse struct {
	Tools []Tool `json:"tools"`
}
