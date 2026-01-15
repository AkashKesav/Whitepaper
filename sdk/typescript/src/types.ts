// Type definitions for RMK MCP SDK

/**
 * Configuration for the RMK client
 */
export interface RMKClientConfig {
  /** Base URL for the RMK API */
  baseURL: string;
  /** API key for authentication (optional) */
  apiKey?: string;
  /** JWT token (if already authenticated) */
  token?: string;
  /** Request timeout in milliseconds */
  timeout?: number;
}

/**
 * Authentication response
 */
export interface AuthResponse {
  token: string;
  username: string;
  role: string;
  groups?: string[];
}

/**
 * Node types for memory storage
 */
export type NodeType = "Entity" | "Fact" | "Event" | "Insight" | "Pattern";

/**
 * Relationship types between entities
 */
export type RelationshipType = "KNOWS" | "LIKES" | "WORKS_AT" | "WORKS_ON" | "FRIEND_OF" | "RELATED_TO" | "PART_OF";

/**
 * Entity types
 */
export type EntityType = "Person" | "Organization" | "Location" | "Concept" | "Product" | "Event" | string;

/**
 * Document types
 */
export type DocumentType = "text" | "markdown" | "json" | "pdf" | "docx" | string;

/**
 * User roles
 */
export type UserRole = "user" | "admin";

/**
 * Admin actions
 */
export type AdminAction = "promote" | "demote" | "delete";

/**
 * Policy effect
 */
export type PolicyEffect = "ALLOW" | "DENY";

/**
 * Memory store parameters
 */
export interface MemoryStoreParams {
  /** Namespace (user_<id> or group_<id>) */
  namespace: string;
  /** Content to store in memory */
  content: string;
  /** Type of node to create */
  nodeType: NodeType;
  /** Optional name/title for the memory */
  name?: string;
  /** Optional tags for categorization */
  tags?: string[];
}

/**
 * Memory store result
 */
export interface MemoryStoreResult {
  uid: string;
  node_type: string;
  namespace: string;
  name?: string;
}

/**
 * Memory search parameters
 */
export interface MemorySearchParams {
  /** Namespace to search within */
  namespace: string;
  /** Search query */
  query: string;
  /** Maximum results to return */
  limit?: number;
}

/**
 * Memory search result
 */
export interface MemorySearchResult {
  results: MemoryNode[];
  count: number;
}

/**
 * Memory node
 */
export interface MemoryNode {
  uid: string;
  name: string;
  description: string;
  node_type?: string;
  activation?: number;
  tags?: string[];
}

/**
 * Memory delete parameters
 */
export interface MemoryDeleteParams {
  namespace: string;
  uid: string;
}

/**
 * Memory list parameters
 */
export interface MemoryListParams {
  namespace: string;
  nodeType?: NodeType;
  limit?: number;
  offset?: number;
}

/**
 * Chat consult parameters
 */
export interface ChatConsultParams {
  namespace: string;
  message: string;
  conversationId?: string;
}

/**
 * Chat consult result
 */
export interface ChatConsultResult {
  response: string;
  conversation_id: string;
  namespace: string;
}

/**
 * Conversations list parameters
 */
export interface ConversationsListParams {
  namespace: string;
  limit?: number;
}

/**
 * Conversations list result
 */
export interface ConversationsListResult {
  conversations: Conversation[];
  count: number;
}

/**
 * Conversation
 */
export interface Conversation {
  id: string;
  name?: string;
  description?: string;
  created_at?: string;
}

/**
 * Conversations delete parameters
 */
export interface ConversationsDeleteParams {
  namespace: string;
  conversationId: string;
}

/**
 * Relationship definition
 */
export interface Relationship {
  type: RelationshipType;
  target: string; // UID of target entity
}

/**
 * Entity create parameters
 */
export interface EntityCreateParams {
  namespace: string;
  name: string;
  entityType: EntityType;
  description?: string;
  relationships?: Relationship[];
  attributes?: Record<string, string>;
}

/**
 * Entity create result
 */
export interface EntityCreateResult {
  uid: string;
  name: string;
  type: string;
}

/**
 * Entity update parameters
 */
export interface EntityUpdateParams {
  namespace: string;
  uid: string;
  name?: string;
  description?: string;
  attributes?: Record<string, string>;
}

/**
 * Entity query parameters
 */
export interface EntityQueryParams {
  namespace: string;
  entityType?: EntityType;
  query?: string;
  limit?: number;
}

/**
 * Entity query result
 */
export interface EntityQueryResult {
  entities: Entity[];
  count: number;
}

/**
 * Entity
 */
export interface Entity {
  uid: string;
  name: string;
  description?: string;
  entity_type?: string;
  activation?: number;
}

/**
 * Relationship create parameters
 */
export interface RelationshipCreateParams {
  namespace: string;
  fromUid: string;
  toUid: string;
  relationshipType: RelationshipType;
}

/**
 * Document ingest parameters
 */
export interface DocumentIngestParams {
  namespace: string;
  content: string;
  filename: string;
  documentType?: DocumentType;
}

/**
 * Document ingest result
 */
export interface DocumentIngestResult {
  status: string;
  filename: string;
  document_id: string;
  entities_extracted: number;
}

/**
 * Document list parameters
 */
export interface DocumentListParams {
  namespace: string;
  limit?: number;
}

/**
 * Document list result
 */
export interface DocumentListResult {
  documents: Document[];
  count: number;
}

/**
 * Document
 */
export interface Document {
  uid: string;
  name: string;
  description?: string;
  created_at?: string;
}

/**
 * Document delete parameters
 */
export interface DocumentDeleteParams {
  namespace: string;
  documentId: string;
}

/**
 * Group create parameters
 */
export interface GroupCreateParams {
  name: string;
  description?: string;
}

/**
 * Group create result
 */
export interface GroupCreateResult {
  group_id: string;
  namespace: string;
  name: string;
}

/**
 * Group list result
 */
export interface GroupListResult {
  groups: Group[];
  count: number;
}

/**
 * Group
 */
export interface Group {
  id: string;
  name: string;
  description?: string;
  namespace: string;
  admin_id: string;
  created_at?: string;
}

/**
 * Group invite parameters
 */
export interface GroupInviteParams {
  groupId: string;
  username: string;
  role: "admin" | "subuser";
}

/**
 * Group invite result
 */
export interface GroupInviteResult {
  invitation_id: string;
  status: string;
}

/**
 * Group members parameters
 */
export interface GroupMembersParams {
  groupId: string;
}

/**
 * Group members result
 */
export interface GroupMembersResult {
  group_id: string;
  members: GroupMember[];
  count: number;
}

/**
 * Group member
 */
export interface GroupMember {
  user_id: string;
  username: string;
  role: string;
}

/**
 * Group share link parameters
 */
export interface GroupShareLinkParams {
  groupId: string;
  maxUses?: number;
  expiresInHours?: number;
}

/**
 * Group share link result
 */
export interface GroupShareLinkResult {
  token: string;
  url: string;
}

/**
 * Admin users list result
 */
export interface AdminUsersListResult {
  users: AdminUser[];
  count: number;
}

/**
 * Admin user
 */
export interface AdminUser {
  username: string;
  role: UserRole;
  created_at?: string;
}

/**
 * Admin user update parameters
 */
export interface AdminUserUpdateParams {
  username: string;
  role?: UserRole;
  action: AdminAction;
}

/**
 * Admin user update result
 */
export interface AdminUserUpdateResult {
  status: string;
  username: string;
  role?: string;
}

/**
 * Admin metrics result
 */
export interface AdminMetricsResult {
  total_users?: number;
  total_groups?: number;
  total_nodes?: number;
  total_conversations?: number;
  uptime_seconds?: number;
}

/**
 * Policy
 */
export interface Policy {
  id: string;
  description?: string;
  effect: PolicyEffect;
  subjects: string[];
  resources: string[];
  actions: string[];
}

/**
 * Admin policies list result
 */
export interface AdminPoliciesListResult {
  policies: Policy[];
  count: number;
}

/**
 * Admin policies set parameters
 */
export interface AdminPoliciesSetParams {
  id: string;
  description?: string;
  effect: PolicyEffect;
  subjects: string[];
  resources: string[];
  actions: string[];
}

/**
 * Admin policies set result
 */
export interface AdminPoliciesSetResult {
  status: string;
  id: string;
  effect: string;
}

/**
 * MCP tool definition
 */
export interface MCPTool {
  name: string;
  description: string;
  inputSchema: Record<string, unknown>;
}

/**
 * MCP tools list result
 */
export interface MCPToolsListResult {
  tools: MCPTool[];
}

/**
 * MCP tool call parameters
 */
export interface MCPToolCallParams {
  name: string;
  arguments: Record<string, unknown>;
}

/**
 * MCP tool call result
 */
export interface MCPToolCallResult {
  content: Array<{
    type: string;
    text: string;
  }>;
  isError?: boolean;
  _meta?: Record<string, unknown>;
}
