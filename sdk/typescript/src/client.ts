/**
 * RMK TypeScript/JavaScript SDK
 *
 * SDK for interacting with the Reflective Memory Kernel via MCP
 */

import type {
  RMKClientConfig,
  AuthResponse,
  MemoryStoreParams,
  MemoryStoreResult,
  MemorySearchParams,
  MemorySearchResult,
  MemoryDeleteParams,
  MemoryListParams,
  MemoryListResult,
  ChatConsultParams,
  ChatConsultResult,
  ConversationsListParams,
  ConversationsListResult,
  ConversationsDeleteParams,
  EntityCreateParams,
  EntityCreateResult,
  EntityUpdateParams,
  EntityUpdateResult,
  EntityQueryParams,
  EntityQueryResult,
  RelationshipCreateParams,
  RelationshipCreateResult,
  DocumentIngestParams,
  DocumentIngestResult,
  DocumentListParams,
  DocumentListResult,
  DocumentDeleteParams,
  GroupCreateParams,
  GroupCreateResult,
  GroupListResult,
  GroupInviteParams,
  GroupInviteResult,
  GroupMembersParams,
  GroupMembersResult,
  GroupShareLinkParams,
  GroupShareLinkResult,
  AdminUsersListResult,
  AdminUserUpdateParams,
  AdminUserUpdateResult,
  AdminMetricsResult,
  AdminPoliciesListResult,
  AdminPoliciesSetParams,
  AdminPoliciesSetResult,
  MCPToolsListResult,
  MCPToolCallParams,
  MCPToolCallResult,
} from "./types.js";

/**
 * Custom error class for RMK SDK errors
 */
export class RMKError extends Error {
  constructor(
    message: string,
    public statusCode?: number,
    public details?: unknown
  ) {
    super(message);
    this.name = "RMKError";
  }
}

/**
 * Main RMK client class
 */
export class RMKClient {
  private config: RMKClientConfig;
  private token?: string;
  private fetch: typeof window.fetch;

  constructor(config: RMKClientConfig) {
    this.config = {
      timeout: 30000,
      ...config,
    };
    this.token = config.token;

    // Use node-fetch in Node.js, native fetch in browsers
    if (typeof window === "undefined") {
      // Dynamic import for node-fetch
      this.fetch = async (...args: Parameters<typeof fetch>) => {
        const nodeFetch = await import("node-fetch");
        return nodeFetch.default(...args);
      };
    } else {
      this.fetch = window.fetch.bind(window);
    }
  }

  /**
   * Set authentication token
   */
  setToken(token: string): void {
    this.token = token;
  }

  /**
   * Get current token
   */
  getToken(): string | undefined {
    return this.token;
  }

  /**
   * Clear authentication token
   */
  clearToken(): void {
    this.token = undefined;
  }

  /**
   * Login with username and password
   */
  async login(username: string, password: string): Promise<AuthResponse> {
    const response = await this.request<AuthResponse>("/api/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    });
    this.token = response.token;
    return response;
  }

  /**
   * Logout
   */
  async logout(): Promise<void> {
    try {
      await this.request<void>("/api/logout", { method: "POST" });
    } finally {
      this.clearToken();
    }
  }

  // ========== MEMORY TOOLS ==========

  /**
   * Store a memory in the knowledge graph
   */
  async memoryStore(params: MemoryStoreParams): Promise<MemoryStoreResult> {
    return this.toolCall<MemoryStoreResult>("memory_store", params);
  }

  /**
   * Search memories
   */
  async memorySearch(params: MemorySearchParams): Promise<MemorySearchResult> {
    return this.toolCall<MemorySearchResult>("memory_search", params);
  }

  /**
   * Delete a memory
   */
  async memoryDelete(params: MemoryDeleteParams): Promise<void> {
    return this.toolCall<void>("memory_delete", params);
  }

  /**
   * List memories
   */
  async memoryList(params: MemoryListParams): Promise<MemoryListResult> {
    return this.toolCall<MemoryListResult>("memory_list", params);
  }

  // ========== CHAT TOOLS ==========

  /**
   * Consult the memory kernel with a query
   */
  async chatConsult(params: ChatConsultParams): Promise<ChatConsultResult> {
    return this.toolCall<ChatConsultResult>("chat_consult", params);
  }

  /**
   * List conversations
   */
  async conversationsList(params: ConversationsListParams): Promise<ConversationsListResult> {
    return this.toolCall<ConversationsListResult>("conversations_list", params);
  }

  /**
   * Delete a conversation
   */
  async conversationsDelete(params: ConversationsDeleteParams): Promise<void> {
    return this.toolCall<void>("conversations_delete", params);
  }

  // ========== ENTITY TOOLS ==========

  /**
   * Create an entity
   */
  async entityCreate(params: EntityCreateParams): Promise<EntityCreateResult> {
    return this.toolCall<EntityCreateResult>("entity_create", params);
  }

  /**
   * Update an entity
   */
  async entityUpdate(params: EntityUpdateParams): Promise<EntityUpdateResult> {
    return this.toolCall<EntityUpdateResult>("entity_update", params);
  }

  /**
   * Query entities
   */
  async entityQuery(params: EntityQueryParams): Promise<EntityQueryResult> {
    return this.toolCall<EntityQueryResult>("entity_query", params);
  }

  /**
   * Create a relationship
   */
  async relationshipCreate(params: RelationshipCreateParams): Promise<RelationshipCreateResult> {
    return this.toolCall<RelationshipCreateResult>("relationship_create", params);
  }

  // ========== DOCUMENT TOOLS ==========

  /**
   * Ingest a document
   */
  async documentIngest(params: DocumentIngestParams): Promise<DocumentIngestResult> {
    return this.toolCall<DocumentIngestResult>("document_ingest", params);
  }

  /**
   * List documents
   */
  async documentList(params: DocumentListParams): Promise<DocumentListResult> {
    return this.toolCall<DocumentListResult>("document_list", params);
  }

  /**
   * Delete a document
   */
  async documentDelete(params: DocumentDeleteParams): Promise<void> {
    return this.toolCall<void>("document_delete", params);
  }

  // ========== GROUP TOOLS ==========

  /**
   * Create a group
   */
  async groupCreate(params: GroupCreateParams): Promise<GroupCreateResult> {
    return this.toolCall<GroupCreateResult>("group_create", params);
  }

  /**
   * List groups
   */
  async groupList(): Promise<GroupListResult> {
    return this.toolCall<GroupListResult>("group_list", {});
  }

  /**
   * Invite a user to a group
   */
  async groupInvite(params: GroupInviteParams): Promise<GroupInviteResult> {
    return this.toolCall<GroupInviteResult>("group_invite", params);
  }

  /**
   * List group members
   */
  async groupMembers(params: GroupMembersParams): Promise<GroupMembersResult> {
    return this.toolCall<GroupMembersResult>("group_members", params);
  }

  /**
   * Create a share link for a group
   */
  async groupShareLink(params: GroupShareLinkParams): Promise<GroupShareLinkResult> {
    return this.toolCall<GroupShareLinkResult>("group_share_link", params);
  }

  // ========== ADMIN TOOLS ==========

  /**
   * List all users (admin only)
   */
  async adminUsersList(): Promise<AdminUsersListResult> {
    return this.toolCall<AdminUsersListResult>("admin_users_list", {});
  }

  /**
   * Update a user (admin only)
   */
  async adminUserUpdate(params: AdminUserUpdateParams): Promise<AdminUserUpdateResult> {
    return this.toolCall<AdminUserUpdateResult>("admin_user_update", params);
  }

  /**
   * Get system metrics (admin only)
   */
  async adminMetrics(): Promise<AdminMetricsResult> {
    return this.toolCall<AdminMetricsResult>("admin_metrics", {});
  }

  /**
   * List policies (admin only)
   */
  async adminPoliciesList(): Promise<AdminPoliciesListResult> {
    return this.toolCall<AdminPoliciesListResult>("admin_policies_list", {});
  }

  /**
   * Create or update a policy (admin only)
   */
  async adminPoliciesSet(params: AdminPoliciesSetParams): Promise<AdminPoliciesSetResult> {
    return this.toolCall<AdminPoliciesSetResult>("admin_policies_set", params);
  }

  // ========== MCP PROTOCOL ==========

  /**
   * List all available MCP tools
   */
  async toolsList(): Promise<MCPToolsListResult> {
    return this.request<MCPToolsListResult>("/api/mcp/tools", {
      method: "GET",
    });
  }

  /**
   * Call an MCP tool directly
   */
  async toolCall<T>(name: string, arguments_: Record<string, unknown>): Promise<T> {
    const result = await this.request<MCPToolCallResult>("/api/mcp/tools/call", {
      method: "POST",
      body: JSON.stringify({
        name,
        arguments: arguments_,
      }),
    });

    // Extract content from MCP response
    if (result.isError) {
      throw new RMKError(`Tool ${name} failed`, undefined, result);
    }

    // Parse the text content as JSON
    const content = result.content[0]?.text;
    if (content) {
      try {
        return JSON.parse(content) as T;
      } catch {
        return content as T;
      }
    }

    return undefined as T;
  }

  /**
   * Make an authenticated HTTP request
   */
  private async request<T>(
    path: string,
    init: RequestInit = {}
  ): Promise<T> {
    const headers: HeadersInit = {
      "Content-Type": "application/json",
    };

    if (this.token) {
      headers["Authorization"] = `Bearer ${this.token}`;
    }

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.config.timeout);

    try {
      const response = await (this.fetch as typeof fetch)(
        `${this.config.baseURL}${path}`,
        {
          ...init,
          headers,
          signal: controller.signal,
        }
      );

      clearTimeout(timeoutId);

      if (!response.ok) {
        const errorText = await response.text().catch(() => "Unknown error");
        throw new RMKError(
          `HTTP ${response.status}: ${errorText}`,
          response.status
        );
      }

      return (await response.json()) as T;
    } catch (error) {
      clearTimeout(timeoutId);

      if (error instanceof RMKError) {
        throw error;
      }

      if (error instanceof Error && error.name === "AbortError") {
        throw new RMKError(`Request timeout after ${this.config.timeout}ms`);
      }

      throw new RMKError(
        `Network error: ${error instanceof Error ? error.message : String(error)}`
      );
    }
  }
}

// Re-export types
export * from "./types.js";
