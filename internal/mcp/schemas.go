// Package mcp defines tool schemas for MCP
package mcp

import "encoding/json"

// ToolSchemas returns all available tool definitions
func ToolSchemas() []Tool {
	return []Tool{
		// ========== MEMORY TOOLS ==========
		{
			Definition: ToolDefinition{
				Name:        "memory_store",
				Description: "Store a memory, fact, entity, or insight in the knowledge graph",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Namespace (user_<id> or group_<id>)",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "Content to store in memory",
						},
						"node_type": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"Entity", "Fact", "Event", "Insight", "Pattern"},
							"description": "Type of node to create",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Optional name/title for the memory",
						},
						"tags": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "string"},
							"description": "Optional tags for categorization",
						},
					},
					"required": []string{"namespace", "content", "node_type"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "memory_search",
				Description: "Search the knowledge graph for matching memories using semantic search",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Namespace to search within",
						},
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Search query",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum results to return",
							"default":     10,
						},
					},
					"required": []string{"namespace", "query"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "memory_delete",
				Description: "Delete a memory from the knowledge graph",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type": "string",
						},
						"uid": map[string]interface{}{
							"type":        "string",
							"description": "UID of the node to delete",
						},
					},
					"required": []string{"namespace", "uid"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "memory_list",
				Description: "List all memories in a namespace with optional filtering",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type": "string",
						},
						"node_type": map[string]interface{}{
							"type":        "string",
							"description": "Filter by node type",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"default":     50,
						},
						"offset": map[string]interface{}{
							"type":        "integer",
							"default":     0,
						},
					},
					"required": []string{"namespace"},
				},
			},
		},

		// ========== CHAT TOOLS ==========
		{
			Definition: ToolDefinition{
				Name:        "chat_consult",
				Description: "Consult the memory kernel with a query - returns AI response with memory context",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Namespace for consultation context",
						},
						"message": map[string]interface{}{
							"type":        "string",
							"description": "Message or query to send",
						},
						"conversation_id": map[string]interface{}{
							"type":        "string",
							"description": "Optional conversation ID for context continuity",
						},
					},
					"required": []string{"namespace", "message"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "conversations_list",
				Description: "List all conversations in a namespace",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type": "string",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"default":     20,
						},
					},
					"required": []string{"namespace"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "conversations_delete",
				Description: "Delete a conversation and its history",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type": "string",
						},
						"conversation_id": map[string]interface{}{
							"type": "string",
						},
					},
					"required": []string{"namespace", "conversation_id"},
				},
			},
		},

		// ========== ENTITY TOOLS ==========
		{
			Definition: ToolDefinition{
				Name:        "entity_create",
				Description: "Create an entity with optional relationships",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type": "string",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Name of the entity",
						},
						"entity_type": map[string]interface{}{
							"type":        "string",
							"description": "Type of entity (Person, Organization, Location, Concept, etc.)",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Description of the entity",
						},
						"relationships": map[string]interface{}{
							"type":        "array",
							"description": "Optional relationships to create",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"type": map[string]interface{}{
										"type": "string",
										"enum": []string{"KNOWS", "LIKES", "WORKS_AT", "WORKS_ON", "FRIEND_OF", "RELATED_TO", "PART_OF"},
									},
									"target": map[string]interface{}{
										"type": "string",
									},
								},
							},
						},
						"attributes": map[string]interface{}{
							"type":        "object",
							"description": "Optional custom attributes",
						},
					},
					"required": []string{"namespace", "name", "entity_type"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "entity_update",
				Description: "Update an existing entity",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type": "string",
						},
						"uid": map[string]interface{}{
							"type": "string",
						},
						"name": map[string]interface{}{
							"type": "string",
						},
						"description": map[string]interface{}{
							"type": "string",
						},
						"attributes": map[string]interface{}{
							"type": "object",
						},
					},
					"required": []string{"namespace", "uid"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "entity_query",
				Description: "Query entities by type or attributes",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type": "string",
						},
						"entity_type": map[string]interface{}{
							"type":        "string",
							"description": "Filter by entity type",
						},
						"query": map[string]interface{}{
							"type":        "string",
							"description": "DGraph query string",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"default":     50,
						},
					},
					"required": []string{"namespace"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "relationship_create",
				Description: "Create a relationship between two entities",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type": "string",
						},
						"from_uid": map[string]interface{}{
							"type": "string",
						},
						"to_uid": map[string]interface{}{
							"type": "string",
						},
						"relationship_type": map[string]interface{}{
							"type": "string",
							"enum": []string{"KNOWS", "LIKES", "WORKS_AT", "WORKS_ON", "FRIEND_OF", "RELATED_TO", "PART_OF"},
						},
					},
					"required": []string{"namespace", "from_uid", "to_uid", "relationship_type"},
				},
			},
		},

		// ========== DOCUMENT TOOLS ==========
		{
			Definition: ToolDefinition{
				Name:        "document_ingest",
				Description: "Ingest a document for semantic processing and entity extraction",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type": "string",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "Document content (text)",
						},
						"filename": map[string]interface{}{
							"type":        "string",
							"description": "Name of the file",
						},
						"document_type": map[string]interface{}{
							"type":        "string",
							"default":     "text",
							"description": "Type of document (text, markdown, json, etc.)",
						},
					},
					"required": []string{"namespace", "content", "filename"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "document_list",
				Description: "List all ingested documents in a namespace",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type": "string",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"default":     20,
						},
					},
					"required": []string{"namespace"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "document_delete",
				Description: "Delete a document and its extracted entities",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type": "string",
						},
						"document_id": map[string]interface{}{
							"type": "string",
						},
					},
					"required": []string{"namespace", "document_id"},
				},
			},
		},

		// ========== GROUP TOOLS ==========
		{
			Definition: ToolDefinition{
				Name:        "group_create",
				Description: "Create a shared workspace/group for collaboration",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Name of the group",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Description of the group",
						},
					},
					"required": []string{"name"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "group_list",
				Description: "List all groups the user is a member of",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties":    map[string]interface{}{},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "group_invite",
				Description: "Invite a user to join a group",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"group_id": map[string]interface{}{
							"type": "string",
						},
						"username": map[string]interface{}{
							"type": "string",
						},
						"role": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"admin", "subuser"},
							"description": "Role to assign to invited user",
						},
					},
					"required": []string{"group_id", "username", "role"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "group_members",
				Description: "List all members of a group",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"group_id": map[string]interface{}{
							"type": "string",
						},
					},
					"required": []string{"group_id"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "group_share_link",
				Description: "Create a share link for others to join the group",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"group_id": map[string]interface{}{
							"type": "string",
						},
						"max_uses": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum number of uses (0 for unlimited)",
							"default":     1,
						},
						"expires_in_hours": map[string]interface{}{
							"type":        "integer",
							"description": "Hours until link expires (0 for never)",
							"default":     24,
						},
					},
					"required": []string{"group_id"},
				},
			},
		},

		// ========== ADMIN TOOLS ==========
		{
			Definition: ToolDefinition{
				Name:        "admin_users_list",
				Description: "List all users (requires admin role)",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "admin_user_update",
				Description: "Update a user's role or settings (requires admin role)",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"username": map[string]interface{}{
							"type": "string",
						},
						"role": map[string]interface{}{
							"type": "string",
							"enum": []string{"user", "admin"},
						},
						"action": map[string]interface{}{
							"type":    "string",
							"enum":    []string{"promote", "demote", "delete"},
							"default": "promote",
						},
					},
					"required": []string{"username"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "admin_metrics",
				Description: "Get system metrics and statistics (requires admin role)",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "admin_policies_list",
				Description: "List all access control policies (requires admin role)",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "admin_policies_set",
				Description: "Create or update an access control policy (requires admin role)",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type": "string",
						},
						"description": map[string]interface{}{
							"type": "string",
						},
						"effect": map[string]interface{}{
							"type": "string",
							"enum": []string{"ALLOW", "DENY"},
						},
						"subjects": map[string]interface{}{
							"type": "array",
							"items": map[string]string{"type": "string"},
						},
						"resources": map[string]interface{}{
							"type": "array",
							"items": map[string]string{"type": "string"},
						},
						"actions": map[string]interface{}{
							"type": "array",
							"items": map[string]string{"type": "string"},
						},
					},
					"required": []string{"id", "effect", "subjects", "resources", "actions"},
				},
			},
		},

		// ========== GRAPH OPERATION TOOLS ==========
		{
			Definition: ToolDefinition{
				Name:        "graph_traverse",
				Description: "Navigate from a node following relationships with spreading activation",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Namespace (user_<id> or group_<id>)",
						},
						"start_node": map[string]interface{}{
							"type":        "string",
							"description": "Starting node UID",
						},
						"max_depth": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum traversal depth (default: 3)",
							"default":     3,
						},
						"decay_factor": map[string]interface{}{
							"type":        "number",
							"description": "Activation decay per hop (0.0-1.0, default: 0.7)",
							"default":     0.7,
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum results to return (default: 50)",
							"default":     50,
						},
					},
					"required": []string{"namespace", "start_node"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "graph_neighbors",
				Description: "Get direct neighbors of a node with relationship types",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Namespace (user_<id> or group_<id>)",
						},
						"node_id": map[string]interface{}{
							"type":        "string",
							"description": "Node UID",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum neighbors to return (default: 100)",
							"default":     100,
						},
					},
					"required": []string{"namespace", "node_id"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "graph_find_path",
				Description: "Find shortest path between two nodes using BFS",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Namespace (user_<id> or group_<id>)",
						},
						"source": map[string]interface{}{
							"type":        "string",
							"description": "Source node UID",
						},
						"target": map[string]interface{}{
							"type":        "string",
							"description": "Target node UID",
						},
						"max_hops": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum path length (default: 5)",
							"default":     5,
						},
					},
					"required": []string{"namespace", "source", "target"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "graph_communities",
				Description: "Detect communities/clusters in the knowledge graph",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Namespace (user_<id> or group_<id>)",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum communities to return (default: 20)",
							"default":     20,
						},
					},
					"required": []string{"namespace"},
				},
			},
		},

		// ========== DOCUMENT ANALYSIS TOOLS ==========
		{
			Definition: ToolDefinition{
				Name:        "document_summarize",
				Description: "Generate a summary of a document stored in the knowledge graph",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Namespace (user_<id> or group_<id>)",
						},
						"document_id": map[string]interface{}{
							"type":        "string",
							"description": "Document node UID",
						},
						"max_length": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum summary length in words (default: 200)",
							"default":     200,
						},
					},
					"required": []string{"namespace", "document_id"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "document_extract",
				Description: "Extract entities from a document stored in the knowledge graph",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Namespace (user_<id> or group_<id>)",
						},
						"document_id": map[string]interface{}{
							"type":        "string",
							"description": "Document node UID",
						},
						"entity_type": map[string]interface{}{
							"type":        "string",
							"description": "Filter by entity type (optional)",
						},
					},
					"required": []string{"namespace", "document_id"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "document_classify",
				Description: "Classify a document content into categories",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Namespace (user_<id> or group_<id>)",
						},
						"document_id": map[string]interface{}{
							"type":        "string",
							"description": "Document node UID",
						},
						"categories": map[string]interface{}{
							"type": "array",
							"description": "Custom categories to classify into (optional)",
							"items": map[string]string{"type": "string"},
						},
					},
					"required": []string{"namespace", "document_id"},
				},
			},
		},

		// ========== CONVERSATION MANAGEMENT TOOLS ==========
		{
			Definition: ToolDefinition{
				Name:        "conversation_export",
				Description: "Export conversation history in various formats",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Namespace (user_<id> or group_<id>)",
						},
						"conversation_id": map[string]interface{}{
							"type":        "string",
							"description": "Conversation ID",
						},
						"format": map[string]interface{}{
							"type":        "string",
							"description": "Export format: json, markdown, or text",
							"enum":        []string{"json", "markdown", "text"},
							"default":     "json",
						},
					},
					"required": []string{"namespace", "conversation_id"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "conversation_summarize",
				Description: "Generate a summary of a long conversation",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Namespace (user_<id> or group_<id>)",
						},
						"conversation_id": map[string]interface{}{
							"type":        "string",
							"description": "Conversation ID",
						},
						"max_points": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum key points to extract (default: 5)",
							"default":     5,
						},
					},
					"required": []string{"namespace", "conversation_id"},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "conversation_branch",
				Description: "Create a branch from a specific point in a conversation",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Namespace (user_<id> or group_<id>)",
						},
						"conversation_id": map[string]interface{}{
							"type":        "string",
							"description": "Source conversation ID",
						},
						"message_id": map[string]interface{}{
							"type":        "string",
							"description": "Message ID to branch from (optional, branches from end if not provided)",
						},
						"branch_name": map[string]interface{}{
							"type":        "string",
							"description": "Name for the new branch (optional)",
						},
					},
					"required": []string{"namespace", "conversation_id"},
				},
			},
		},

		// ========== USER SETTINGS TOOLS ==========
		{
			Definition: ToolDefinition{
				Name:        "user_profile_get",
				Description: "Get the current user's profile information",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "user_profile_update",
				Description: "Update the current user's profile information",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"display_name": map[string]interface{}{
							"type":        "string",
							"description": "Display name",
						},
						"bio": map[string]interface{}{
							"type":        "string",
							"description": "User biography",
						},
						"preferences": map[string]interface{}{
							"type":        "object",
							"description": "User preferences as key-value pairs",
						},
					},
				},
			},
		},
		{
			Definition: ToolDefinition{
				Name:        "user_preferences_get",
				Description: "Get the current user's preferences",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
	}
}

// GetToolSchema returns JSON schema for a specific tool
func GetToolSchema(name string) (map[string]interface{}, error) {
	for _, tool := range ToolSchemas() {
		if tool.Definition.Name == name {
			return tool.Definition.InputSchema, nil
		}
	}
	return nil, nil
}

// ToolDefinitionsAsJSON returns all tool definitions as JSON
func ToolDefinitionsAsJSON() (string, error) {
	defs := make([]map[string]interface{}, 0)
	for _, tool := range ToolSchemas() {
		defs = append(defs, map[string]interface{}{
			"name":        tool.Definition.Name,
			"description": tool.Definition.Description,
			"inputSchema": tool.Definition.InputSchema,
		})
	}
	data, err := json.MarshalIndent(defs, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
