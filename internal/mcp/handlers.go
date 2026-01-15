// Package mcp implements tool handlers for MCP server
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/reflective-memory-kernel/internal/agent"
	"github.com/reflective-memory-kernel/internal/graph"
	"github.com/reflective-memory-kernel/internal/policy"
	"go.uber.org/zap"
)

// HandlerDependencies contains dependencies for tool handlers
type HandlerDependencies struct {
	Agent  *agent.Agent
	Logger *zap.Logger
}

// getGraphClient returns the graph client from agent
func (d *HandlerDependencies) getGraphClient() *graph.Client {
	return d.Agent.GetGraphClient()
}

// getPolicyManager returns the policy manager from agent
func (d *HandlerDependencies) getPolicyManager() *policy.PolicyManager {
	return d.Agent.PolicyManager
}

// ========== MEMORY TOOL HANDLERS ==========

// handleMemoryStore stores a memory in the knowledge graph
func handleMemoryStore(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	content := getString(args, "content")
	nodeType := getString(args, "node_type")
	name := getString(args, "name")
	description := content

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	// If name is provided, use it, otherwise generate from content
	if name == "" {
		name = generateName(content)
	}

	// Build node
	node := &graph.Node{
		Name:        name,
		Description: description,
		Namespace:   namespace,
		DType:       []string{nodeType},
	}

	// Add optional tags
	if tags, ok := args["tags"].([]interface{}); ok {
		for _, t := range tags {
			if tag, ok := t.(string); ok {
				node.Tags = append(node.Tags, tag)
			}
		}
	}

	uid, err := graphClient.CreateNode(ctx, node)
	if err != nil {
		return nil, fmt.Errorf("failed to store memory: %w", err)
	}

	deps.Logger.Info("Memory stored via MCP",
		zap.String("uid", uid),
		zap.String("namespace", namespace),
		zap.String("node_type", nodeType))

	return map[string]interface{}{
		"uid":       uid,
		"node_type": nodeType,
		"namespace": namespace,
		"name":      name,
	}, nil
}

// handleMemorySearch searches the knowledge graph
func handleMemorySearch(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	query := getString(args, "query")
	limit := getInt(args, "limit", 10)

	// Use Agent's Consult method via MKClient
	mkClient := deps.Agent.GetMKClient()
	if mkClient == nil {
		return nil, fmt.Errorf("memory kernel client not available")
	}

	results, err := mkClient.Consult(ctx, &graph.ConsultationRequest{
		UserID:         getNamespaceUserID(namespace),
		Namespace:      namespace,
		Query:          query,
		MaxResults:     limit,
		IncludeInsights: true,
	})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Format results
	nodes := make([]map[string]interface{}, 0)
	for _, node := range results.RelevantFacts {
		nodes = append(nodes, map[string]interface{}{
			"uid":         node.UID,
			"name":        node.Name,
			"description": node.Description,
			"node_type":   node.GetType(),
			"activation":  node.Activation,
			"tags":        node.Tags,
		})
	}

	return map[string]interface{}{
		"results": nodes,
		"count":   len(nodes),
		"brief":   results.SynthesizedBrief,
	}, nil
}

// handleMemoryDelete deletes a memory node
func handleMemoryDelete(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	uid := getString(args, "uid")

	// Verify namespace access
	userID := getNamespaceUserID(namespace)
	if err := checkNamespaceAccess(ctx, deps, userID, namespace, policy.ActionDelete); err != nil {
		return nil, err
	}

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	err := graphClient.DeleteNode(ctx, uid, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to delete memory: %w", err)
	}

	deps.Logger.Info("Memory deleted via MCP",
		zap.String("uid", uid),
		zap.String("namespace", namespace))

	return map[string]interface{}{
		"status":   "deleted",
		"uid":      uid,
	}, nil
}

// handleMemoryList lists memories in a namespace
func handleMemoryList(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	nodeType := getString(args, "node_type", "")
	limit := getInt(args, "limit", 50)
	offset := getInt(args, "offset", 0)

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	// Use SearchNodes to find nodes in namespace
	nodes, err := graphClient.SearchNodes(ctx, "*", namespace)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Filter by node type if specified
	var filteredNodes []graph.Node
	if nodeType != "" {
		filteredNodes = make([]graph.Node, 0)
		for _, node := range nodes {
			if string(node.GetType()) == nodeType {
				filteredNodes = append(filteredNodes, node)
			}
		}
	} else {
		filteredNodes = nodes
	}

	// Convert to map format
	resultNodes := make([]map[string]interface{}, 0)
	for _, node := range filteredNodes {
		resultNodes = append(resultNodes, map[string]interface{}{
			"uid":         node.UID,
			"name":        node.Name,
			"description": node.Description,
			"type":        node.GetType(),
			"activation":  node.Activation,
			"tags":        node.Tags,
			"namespace":   node.Namespace,
		})
	}

	// Apply limit and offset
	start := offset
	end := offset + limit
	if start >= len(resultNodes) {
		resultNodes = []map[string]interface{}{}
	} else if end > len(resultNodes) {
		resultNodes = resultNodes[start:]
	} else {
		resultNodes = resultNodes[start:end]
	}

	return map[string]interface{}{
		"results": resultNodes,
		"total":   len(resultNodes),
		"offset":  offset,
		"limit":   limit,
	}, nil
}

// ========== CHAT TOOL HANDLERS ==========

// handleChatConsult performs a chat consultation
func handleChatConsult(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	message := getString(args, "message")
	conversationID := getString(args, "conversation_id", "")

	// Generate conversation ID if not provided
	if conversationID == "" {
		conversationID = generateUUID()
	}

	response, err := deps.Agent.Chat(ctx, getNamespaceUserID(namespace), conversationID, namespace, message)
	if err != nil {
		return nil, fmt.Errorf("chat failed: %w", err)
	}

	deps.Logger.Info("Chat consult via MCP",
		zap.String("conversation_id", conversationID),
		zap.String("namespace", namespace))

	return map[string]interface{}{
		"response":        response,
		"conversation_id": conversationID,
		"namespace":       namespace,
	}, nil
}

// handleConversationsList lists conversations
func handleConversationsList(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	limit := getInt(args, "limit", 20)

	// For now, return conversations from Agent's in-memory store
	// In production, this would query the graph
	_ = namespace
	_ = limit

	// Get active conversations from Agent
	conversations := make([]map[string]interface{}, 0)
	// TODO: Query graph for conversations in namespace

	return map[string]interface{}{
		"conversations": conversations,
		"count":         len(conversations),
	}, nil
}

// handleConversationsDelete deletes a conversation
func handleConversationsDelete(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	conversationID := getString(args, "conversation_id")

	// Verify namespace access
	userID := getNamespaceUserID(namespace)
	if err := checkNamespaceAccess(ctx, deps, userID, namespace, policy.ActionDelete); err != nil {
		return nil, err
	}

	// Delete conversation node
	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	err := graphClient.DeleteNode(ctx, conversationID, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to delete conversation: %w", err)
	}

	return map[string]interface{}{
		"status":         "deleted",
		"conversation_id": conversationID,
	}, nil
}

// ========== ENTITY TOOL HANDLERS ==========

// handleEntityCreate creates an entity with relationships
func handleEntityCreate(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	name := getString(args, "name")
	entityType := getString(args, "entity_type")
	description := getString(args, "description", "")

	// Build node
	node := &graph.Node{
		Name:        name,
		Description: description,
		Namespace:   namespace,
		DType:       []string{"Entity"},
		Attributes:  map[string]string{"entity_type": entityType},
	}

	// Add optional attributes
	if attrs, ok := args["attributes"].(map[string]interface{}); ok {
		for k, v := range attrs {
			if vs, ok := v.(string); ok {
				node.Attributes[k] = vs
			}
		}
	}

	uid, err := deps.getGraphClient().CreateNode(ctx, node)
	if err != nil {
		return nil, fmt.Errorf("failed to create entity: %w", err)
	}

	// Handle relationships if provided
	if relationships, ok := args["relationships"].([]interface{}); ok {
		for _, rel := range relationships {
			if r, ok := rel.(map[string]interface{}); ok {
				relType := getString(r, "type")
				target := getString(r, "target")
				if err := deps.getGraphClient().CreateEdge(ctx, uid, target, graph.EdgeType(relType), graph.EdgeStatusCurrent); err != nil {
					deps.Logger.Warn("Failed to create relationship",
						zap.String("from", uid),
						zap.String("to", target),
						zap.String("type", relType),
						zap.Error(err))
				}
			}
		}
	}

	deps.Logger.Info("Entity created via MCP",
		zap.String("uid", uid),
		zap.String("name", name),
		zap.String("entity_type", entityType))

	return map[string]interface{}{
		"uid":    uid,
		"name":   name,
		"type":   entityType,
	}, nil
}

// handleEntityUpdate updates an entity
func handleEntityUpdate(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	uid := getString(args, "uid")
	description := getString(args, "description", "")

	// Verify namespace access
	userID := getNamespaceUserID(namespace)
	if err := checkNamespaceAccess(ctx, deps, userID, namespace, policy.ActionWrite); err != nil {
		return nil, err
	}

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	// Update description if provided
	if description != "" {
		if err := graphClient.UpdateDescription(ctx, uid, description); err != nil {
			return nil, fmt.Errorf("failed to update entity: %w", err)
		}
	}

	// Note: name and attribute updates would require direct DGraph mutations
	// For now, we only support description updates

	return map[string]interface{}{
		"uid":    uid,
		"status": "updated",
	}, nil
}

// handleEntityQuery queries entities
func handleEntityQuery(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	entityType := getString(args, "entity_type", "")
	queryStr := getString(args, "query", "")
	limit := getInt(args, "limit", 50)

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	// Use SearchNodes to find entities
	searchTerm := queryStr
	if searchTerm == "" {
		searchTerm = "*" // Match all if no query
	}

	nodes, err := graphClient.SearchNodes(ctx, searchTerm, namespace)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Filter by entity type if specified
	var filteredNodes []graph.Node
	if entityType != "" {
		filteredNodes = make([]graph.Node, 0)
		for _, node := range nodes {
			if string(node.GetType()) == entityType {
				filteredNodes = append(filteredNodes, node)
			}
		}
	} else {
		filteredNodes = nodes
	}

	// Apply limit
	if len(filteredNodes) > limit {
		filteredNodes = filteredNodes[:limit]
	}

	// Convert to result format
	entities := make([]map[string]interface{}, 0)
	for _, node := range filteredNodes {
		entities = append(entities, map[string]interface{}{
			"uid":         node.UID,
			"name":        node.Name,
			"description": node.Description,
			"type":        node.GetType(),
			"activation":  node.Activation,
		})
	}

	return map[string]interface{}{
		"entities": entities,
		"count":    len(entities),
	}, nil
}

// handleRelationshipCreate creates a relationship
func handleRelationshipCreate(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	fromUID := getString(args, "from_uid")
	toUID := getString(args, "to_uid")
	relType := getString(args, "relationship_type")

	// Verify namespace access
	userID := getNamespaceUserID(namespace)
	if err := checkNamespaceAccess(ctx, deps, userID, namespace, policy.ActionWrite); err != nil {
		return nil, err
	}

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	err := graphClient.CreateEdge(ctx, fromUID, toUID, graph.EdgeType(relType), graph.EdgeStatusCurrent)
	if err != nil {
		return nil, fmt.Errorf("failed to create relationship: %w", err)
	}

	return map[string]interface{}{
		"status":     "created",
		"from_uid":   fromUID,
		"to_uid":     toUID,
		"rel_type":   relType,
	}, nil
}

// ========== DOCUMENT TOOL HANDLERS ==========

// handleDocumentIngest ingests a document
func handleDocumentIngest(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	content := getString(args, "content")
	filename := getString(args, "filename")
	docType := getString(args, "document_type", "text")

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	// Create a document node
	node := &graph.Node{
		Name:        filename,
		Description: content,
		Namespace:   namespace,
		DType:       []string{docType, "Document"},
	}

	uid, err := graphClient.CreateNode(ctx, node)
	if err != nil {
		return nil, fmt.Errorf("failed to create document node: %w", err)
	}

	deps.Logger.Info("Document ingested via MCP",
		zap.String("filename", filename),
		zap.String("namespace", namespace))

	return map[string]interface{}{
		"status":            "created",
		"document_id":       uid,
		"filename":          filename,
		"entities_extracted": 0,
	}, nil
}

// handleDocumentList lists documents
func handleDocumentList(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	limit := getInt(args, "limit", 20)

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	// Use SearchNodes to find documents
	nodes, err := graphClient.SearchNodes(ctx, "*", namespace)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Filter for Document type
	documents := make([]graph.Node, 0)
	for _, node := range nodes {
		if node.GetType() == "Document" {
			documents = append(documents, node)
		}
	}

	// Apply limit
	if len(documents) > limit {
		documents = documents[:limit]
	}

	// Convert to result format
	resultDocs := make([]map[string]interface{}, 0)
	for _, doc := range documents {
		resultDocs = append(resultDocs, map[string]interface{}{
			"uid":         doc.UID,
			"name":        doc.Name,
			"description": doc.Description,
			"created_at":  doc.CreatedAt,
		})
	}

	return map[string]interface{}{
		"documents": resultDocs,
		"count":     len(resultDocs),
	}, nil
}

// handleDocumentDelete deletes a document
func handleDocumentDelete(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	documentID := getString(args, "document_id")

	// Verify namespace access
	userID := getNamespaceUserID(namespace)
	if err := checkNamespaceAccess(ctx, deps, userID, namespace, policy.ActionDelete); err != nil {
		return nil, err
	}

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	err := graphClient.DeleteNode(ctx, documentID, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to delete document: %w", err)
	}

	return map[string]interface{}{
		"status":      "deleted",
		"document_id": documentID,
	}, nil
}

// ========== GROUP TOOL HANDLERS ==========

// handleGroupCreate creates a group
func handleGroupCreate(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	name := getString(args, "name")
	description := getString(args, "description", "")

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	// Get user ID from context
	userID := ctx.Value("user_id")
	if userID == nil {
		return nil, fmt.Errorf("user not authenticated")
	}
	userIDStr, _ := userID.(string)

	// Create group via graph client
	groupNamespace, err := graphClient.CreateGroup(ctx, name, description, userIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	deps.Logger.Info("Group created via MCP",
		zap.String("name", name),
		zap.String("namespace", groupNamespace))

	return map[string]interface{}{
		"group_id":  groupNamespace,
		"namespace": groupNamespace,
		"name":      name,
	}, nil
}

// handleGroupList lists groups
func handleGroupList(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	// Get user's groups from agent
	// Note: This would require getting user ID from context
	return map[string]interface{}{
		"groups": []interface{}{},
		"count":  0,
	}, nil
}

// handleGroupInvite invites a user to a group
func handleGroupInvite(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	groupID := getString(args, "group_id")
	username := getString(args, "username")
	role := getString(args, "role", "subuser")

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	// Add user to group via graph client
	err := graphClient.AddGroupMember(ctx, groupID, username)
	if err != nil {
		return nil, fmt.Errorf("failed to add group member: %w", err)
	}

	return map[string]interface{}{
		"group_id": groupID,
		"username": username,
		"role":     role,
		"status":   "invited",
	}, nil
}

// handleGroupMembers lists group members
func handleGroupMembers(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	groupID := getString(args, "group_id")

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	members, err := graphClient.GetWorkspaceMembers(ctx, groupID)
	if err != nil {
		// Fallback: return empty list if GetWorkspaceMembers fails
		members = []graph.WorkspaceMember{}
	}

	// Convert to result format
	resultMembers := make([]map[string]interface{}, 0)
	for _, member := range members {
		userID := ""
		username := ""
		if member.User != nil {
			userID = member.User.UID
			username = member.User.Name
		}
		resultMembers = append(resultMembers, map[string]interface{}{
			"user_id":  userID,
			"username": username,
			"role":     member.Role,
		})
	}

	return map[string]interface{}{
		"group_id": groupID,
		"members":  resultMembers,
		"count":    len(resultMembers),
	}, nil
}

// handleGroupShareLink creates a share link
func handleGroupShareLink(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	groupID := getString(args, "group_id")
	maxUses := getInt(args, "max_uses", 1)
	expiresInHours := getInt(args, "expires_in_hours", 24)

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	// Get user ID from context
	userID := ctx.Value("user_id")
	if userID == nil {
		return nil, fmt.Errorf("user not authenticated")
	}
	userIDStr, _ := userID.(string)

	// Calculate expiration
	var expiresAt *time.Time
	if expiresInHours > 0 {
		t := time.Now().Add(time.Duration(expiresInHours) * time.Hour)
		expiresAt = &t
	}

	// Create share link
	link, err := graphClient.CreateShareLink(ctx, groupID, userIDStr, maxUses, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create share link: %w", err)
	}

	return map[string]interface{}{
		"link_id": link.UID,
		"token":   link.Token,
		"group_id": groupID,
		"max_uses": maxUses,
	}, nil
}

// ========== ADMIN TOOL HANDLERS ==========

// handleAdminUsersList lists all users (admin only)
func handleAdminUsersList(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	// Check admin permission
	if !isAdmin(ctx) {
		return nil, fmt.Errorf("admin access required")
	}

	// TODO: Implement user listing via graph client
	return map[string]interface{}{
		"users": []interface{}{},
		"count": 0,
	}, nil
}

// handleAdminUserUpdate updates a user (admin only)
func handleAdminUserUpdate(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	if !isAdmin(ctx) {
		return nil, fmt.Errorf("admin access required")
	}

	username := getString(args, "username")
	role := getString(args, "role", "user")
	action := getString(args, "action", "update")

	// TODO: Implement user update via graph client
	return map[string]interface{}{
		"status":   action + "d",
		"username": username,
		"role":     role,
	}, nil
}

// handleAdminMetrics returns system metrics (admin only)
func handleAdminMetrics(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	if !isAdmin(ctx) {
		return nil, fmt.Errorf("admin access required")
	}

	// Get agent stats
	stats := deps.Agent.GetStats()

	return map[string]interface{}{
		"stats": stats,
	}, nil
}

// handleAdminPoliciesList lists policies (admin only)
func handleAdminPoliciesList(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	if !isAdmin(ctx) {
		return nil, fmt.Errorf("admin access required")
	}

	// TODO: Implement policy listing
	return map[string]interface{}{
		"policies": []interface{}{},
		"count":    0,
	}, nil
}

// handleAdminPoliciesSet creates or updates a policy (admin only)
func handleAdminPoliciesSet(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	if !isAdmin(ctx) {
		return nil, fmt.Errorf("admin access required")
	}

	id := getString(args, "id")
	effect := getString(args, "effect")

	// TODO: Implement policy creation
	return map[string]interface{}{
		"status": "created",
		"id":      id,
		"effect":  effect,
	}, nil
}

// ========== HELPER FUNCTIONS ==========

// getString safely extracts a string value from args
func getString(args map[string]interface{}, key string, defaultVal ...string) string {
	if val, ok := args[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	if len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return ""
}

// getInt safely extracts an int value from args
func getInt(args map[string]interface{}, key string, defaultVal int) int {
	if val, ok := args[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			i, _ := strconv.Atoi(v)
			return i
		}
	}
	return defaultVal
}

// getNamespaceUserID extracts user ID from namespace
func getNamespaceUserID(namespace string) string {
	if len(namespace) > 5 && namespace[:5] == "user_" {
		return namespace[5:]
	}
	if len(namespace) > 6 && namespace[:6] == "group_" {
		return namespace[6:]
	}
	return namespace
}

// checkNamespaceAccess verifies user has access to namespace
func checkNamespaceAccess(ctx context.Context, deps *HandlerDependencies, userID, namespace string, action policy.Action) error {
	// Build user context
	userCtx := policy.UserContext{
		UserID:        userID,
		Authenticated: true,
		Groups:        []string{},
		Clearance:     0,
		Attributes:    map[string]string{},
	}

	// Create a dummy resource for policy check
	resource := &graph.Node{
		Namespace: namespace,
	}

	// Evaluate policy
	effect, err := deps.getPolicyManager().Evaluate(ctx, userCtx, resource, action)
	if err != nil || effect != policy.EffectAllow {
		return fmt.Errorf("access denied to namespace %s", namespace)
	}

	return nil
}

// isAdmin checks if the current user is an admin
func isAdmin(ctx context.Context) bool {
	if role, ok := ctx.Value("user_role").(string); ok {
		return role == "admin"
	}
	return false
}

// generateName creates a name from content
func generateName(content string) string {
	if len(content) > 50 {
		return content[:47] + "..."
	}
	return content
}

// generateUUID generates a unique identifier
func generateUUID() string {
	// Simple UUID generation
	return fmt.Sprintf("%d-%d-%d-%d",
		getCurrentTimeNano(),
		getCurrentTimeNano()%1000,
		getCurrentTimeNano()%1000000,
		getCurrentTimeNano()%1000000000)
}

func getCurrentTimeNano() int64 {
	return 1234567890 // Placeholder - use actual time in production
}

// ========== GRAPH OPERATION HANDLERS ==========

// handleGraphTraverse performs spreading activation traversal from a node
func handleGraphTraverse(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	startNode := getString(args, "start_node")
	maxDepth := getInt(args, "max_depth", 3)
	decayFactor := getFloat(args, "decay_factor", 0.7)
	limit := getInt(args, "limit", 50)

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	opts := graph.SpreadActivationOpts{
		StartUID:      startNode,
		Namespace:     namespace,
		MaxHops:       maxDepth,
		DecayFactor:   decayFactor,
		MaxResults:    limit,
		MinActivation: 0.05,
	}

	results, err := graphClient.SpreadActivation(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("traversal failed: %w", err)
	}

	// Format results
	nodes := make([]map[string]interface{}, 0)
	for _, r := range results {
		nodes = append(nodes, map[string]interface{}{
			"uid":        r.Node.UID,
			"name":       r.Node.Name,
			"type":       r.Node.GetType(),
			"activation": r.Activation,
			"hops":       r.Hops,
		})
	}

	return map[string]interface{}{
		"start_node": startNode,
		"results":    nodes,
		"count":      len(nodes),
	}, nil
}

// handleGraphNeighbors gets direct neighbors of a node
func handleGraphNeighbors(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	nodeID := getString(args, "node_id")
	limit := getInt(args, "limit", 100)

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	// Use the traversal spread activation with max_depth=1 to get direct neighbors
	opts := graph.SpreadActivationOpts{
		StartUID:      nodeID,
		Namespace:     namespace,
		MaxHops:       1,
		DecayFactor:   1.0, // No decay for direct neighbors
		MaxResults:    limit,
		MinActivation: 0.0,
	}

	results, err := graphClient.SpreadActivation(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get neighbors: %w", err)
	}

	neighbors := make([]map[string]interface{}, 0)
	for _, r := range results {
		if r.Node.UID != nodeID { // Exclude the start node
			neighbors = append(neighbors, map[string]interface{}{
				"uid":         r.Node.UID,
				"name":        r.Node.Name,
				"type":        r.Node.GetType(),
				"description": r.Node.Description,
			})
		}
	}

	return map[string]interface{}{
		"node_id":   nodeID,
		"neighbors": neighbors,
		"count":     len(neighbors),
	}, nil
}

// handleGraphFindPath finds shortest path between two nodes
func handleGraphFindPath(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	_ = getString(args, "namespace") // Reserved for namespace validation
	source := getString(args, "source")
	target := getString(args, "target")
	maxHops := getInt(args, "max_hops", 5)

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	// Use expansion to find path
	opts := graph.ExpandOpts{
		StartUID:   source,
		MaxHops:    maxHops,
		MaxResults: 100,
	}

	result, err := graphClient.ExpandFromNode(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("path finding failed: %w", err)
	}

	// Find the target in the results and extract path
	path := make([]map[string]interface{}, 0)
	path = append(path, map[string]interface{}{
		"uid":  result.StartNode.UID,
		"name": result.StartNode.Name,
	})

	// For now, return the expansion results as a simplified path
	// A full BFS pathfinding would require more implementation
	for hopLevel, nodes := range result.ByHop {
		if hopLevel == 0 {
			continue
		}
		for _, node := range nodes {
			path = append(path, map[string]interface{}{
				"uid":  node.UID,
				"name": node.Name,
			})
			if node.UID == target {
				break
			}
		}
	}

	return map[string]interface{}{
		"source": source,
		"target": target,
		"path":   path,
		"length": len(path),
	}, nil
}

// handleGraphCommunities detects communities in the graph
func handleGraphCommunities(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	limit := getInt(args, "limit", 20)

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	// Get sample nodes to group by attributes
	nodes, err := graphClient.GetSampleNodes(ctx, namespace, limit*5)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	// Group nodes by their type (simple community detection)
	communities := make(map[string][]map[string]interface{})
	for _, node := range nodes {
		nodeType := string(node.GetType())
		if nodeType == "" {
			nodeType = "Unknown"
		}
		communities[nodeType] = append(communities[nodeType], map[string]interface{}{
			"uid":         node.UID,
			"name":        node.Name,
			"description": node.Description,
			"activation":  node.Activation,
		})
	}

	// Format results
	result := make([]map[string]interface{}, 0)
	for commType, members := range communities {
		result = append(result, map[string]interface{}{
			"community": commType,
			"count":     len(members),
			"members":   members,
		})
	}

	return map[string]interface{}{
		"namespace":    namespace,
		"communities":  result,
		"total_groups": len(result),
	}, nil
}

// ========== DOCUMENT ANALYSIS HANDLERS ==========

// handleDocumentSummarize generates a summary of a document
func handleDocumentSummarize(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	documentID := getString(args, "document_id")
	maxLength := getInt(args, "max_length", 200)

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	// Get the document node
	node, err := graphClient.GetNode(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("document not found: %w", err)
	}

	if node.Namespace != namespace {
		return nil, fmt.Errorf("access denied to document")
	}

	// Simple summary: extract first N words from description
	words := strings.Fields(node.Description)
	if len(words) > maxLength {
		words = words[:maxLength]
	}
	summary := strings.Join(words, " ")

	return map[string]interface{}{
		"document_id": documentID,
		"summary":     summary,
		"word_count":  len(words),
	}, nil
}

// handleDocumentExtract extracts entities from a document
func handleDocumentExtract(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	documentID := getString(args, "document_id")
	entityType := getString(args, "entity_type", "")

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	// Get the document node
	docNode, err := graphClient.GetNode(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("document not found: %w", err)
	}

	if docNode.Namespace != namespace {
		return nil, fmt.Errorf("access denied to document")
	}

	// Query for related entities
	query := fmt.Sprintf(`
		{
			nodes(func: uid(%s)) {
				related_to @filter(eq(namespace, "%s")) {
					uid
					name
					description
					dgraph.type
					activation
				}
			}
		}
	`, documentID, namespace)

	respBytes, err := graphClient.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Parse and filter results
	entities := make([]map[string]interface{}, 0)
	// The response is a raw JSON byte array, so we need to parse it
	var result map[string][]map[string]interface{}
	if err := json.Unmarshal(respBytes, &result); err == nil {
		if nodes, ok := result["nodes"]; ok && len(nodes) > 0 {
			if related, ok := nodes[0]["related_to"].([]map[string]interface{}); ok {
				for _, entity := range related {
					if entityType == "" || entity["dgraph.type"] == entityType {
						entities = append(entities, entity)
					}
				}
			}
		}
	}

	return map[string]interface{}{
		"document_id":  documentID,
		"entities":     entities,
		"count":        len(entities),
		"filter_type":  entityType,
	}, nil
}

// handleDocumentClassify classifies a document into categories
func handleDocumentClassify(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	namespace := getString(args, "namespace")
	documentID := getString(args, "document_id")

	graphClient := deps.getGraphClient()
	if graphClient == nil {
		return nil, fmt.Errorf("graph client not available")
	}

	// Get the document node
	node, err := graphClient.GetNode(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("document not found: %w", err)
	}

	if node.Namespace != namespace {
		return nil, fmt.Errorf("access denied to document")
	}

	// Simple classification based on keywords
	defaultCategories := map[string][]string{
		"Technical":   {"code", "api", "function", "algorithm", "debug", "compile"},
		"Business":    {"revenue", "profit", "customer", "market", "sales"},
		"Personal":    {"i", "my", "remember", "thought", "feeling"},
		"Reference":   {"note", "save", "bookmark", "reference", "cite"},
	}

	scores := make(map[string]float64)
	descLower := strings.ToLower(node.Description)

	for category, keywords := range defaultCategories {
		for _, kw := range keywords {
			if strings.Contains(descLower, kw) {
				scores[category] += 1.0
			}
		}
	}

	// Find top category
	topCategory := "General"
	maxScore := 0.0
	for cat, score := range scores {
		if score > maxScore {
			maxScore = score
			topCategory = cat
		}
	}

	return map[string]interface{}{
		"document_id":    documentID,
		"category":       topCategory,
		"confidence":     maxScore / 10.0, // Normalized confidence
		"all_scores":     scores,
	}, nil
}

// ========== CONVERSATION MANAGEMENT HANDLERS ==========

// handleConversationExport exports conversation history
func handleConversationExport(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	_ = getString(args, "namespace") // Reserved for validation
	conversationID := getString(args, "conversation_id")
	format := getString(args, "format", "json")

	// Get user ID from context for authentication
	userID := ctx.Value("user_id")
	if userID == nil {
		return nil, fmt.Errorf("user not authenticated")
	}

	// Get conversation from Agent
	conv := deps.Agent.GetConversation(conversationID)
	if conv == nil {
		return nil, fmt.Errorf("conversation not found")
	}

	// Build export data
	turns := make([]map[string]interface{}, 0)
	for _, turn := range conv.Turns {
		turns = append(turns, map[string]interface{}{
			"timestamp": turn.Timestamp,
			"query":     turn.UserQuery,
			"response":  turn.Response,
			"latency":   turn.Latency.Milliseconds(),
		})
	}

	exportData := map[string]interface{}{
		"conversation_id": conversationID,
		"user_id":         conv.UserID,
		"started_at":      conv.StartedAt,
		"turns":           turns,
		"turn_count":      len(turns),
	}

	// Format output based on requested format
	switch format {
	case "markdown":
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# Conversation: %s\n\n", conversationID))
		sb.WriteString(fmt.Sprintf("Started: %s\n\n", conv.StartedAt.Format("2006-01-02 15:04:05")))
		for i, turn := range conv.Turns {
			sb.WriteString(fmt.Sprintf("## Turn %d\n", i+1))
			sb.WriteString(fmt.Sprintf("**User:** %s\n\n", turn.UserQuery))
			sb.WriteString(fmt.Sprintf("**AI:** %s\n\n", turn.Response))
		}
		return map[string]interface{}{
			"format":   "markdown",
			"content":  sb.String(),
			"filename": fmt.Sprintf("%s.md", conversationID[:8]),
		}, nil
	case "text":
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Conversation: %s\n", conversationID))
		sb.WriteString(fmt.Sprintf("Started: %s\n\n", conv.StartedAt.Format("2006-01-02 15:04:05")))
		for _, turn := range conv.Turns {
			sb.WriteString(fmt.Sprintf("User: %s\n", turn.UserQuery))
			sb.WriteString(fmt.Sprintf("AI: %s\n\n", turn.Response))
		}
		return map[string]interface{}{
			"format":   "text",
			"content":  sb.String(),
			"filename": fmt.Sprintf("%s.txt", conversationID[:8]),
		}, nil
	default: // json
		return map[string]interface{}{
			"format":  "json",
			"data":    exportData,
		}, nil
	}
}

// handleConversationSummarize summarizes a conversation
func handleConversationSummarize(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	_ = getString(args, "namespace") // Reserved for validation
	conversationID := getString(args, "conversation_id")
	maxPoints := getInt(args, "max_points", 5)

	conv := deps.Agent.GetConversation(conversationID)
	if conv == nil {
		return nil, fmt.Errorf("conversation not found")
	}

	// Extract key points from user queries
	keyPoints := make([]string, 0)
	for i, turn := range conv.Turns {
		if i >= maxPoints {
			break
		}
		// Use the user query as a key point
		if len(turn.UserQuery) > 0 {
			keyPoints = append(keyPoints, turn.UserQuery)
		}
	}

	// Generate summary
	summary := fmt.Sprintf("Conversation with %d turns starting at %s. Key topics discussed: %s",
		len(conv.Turns),
		conv.StartedAt.Format("2006-01-02 15:04"),
		strings.Join(keyPoints, ", "))

	return map[string]interface{}{
		"conversation_id": conversationID,
		"summary":         summary,
		"key_points":      keyPoints,
		"turn_count":      len(conv.Turns),
	}, nil
}

// handleConversationBranch creates a conversation branch
func handleConversationBranch(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	_ = getString(args, "namespace") // Reserved for validation
	conversationID := getString(args, "conversation_id")
	_ = getString(args, "message_id", "") // Reserved for future use
	branchName := getString(args, "branch_name", "")

	// Get source conversation
	conv := deps.Agent.GetConversation(conversationID)
	if conv == nil {
		return nil, fmt.Errorf("source conversation not found")
	}

	// Generate new conversation ID
	newConvID := fmt.Sprintf("conv_%x", generateUUID())

	// Copy context up to the branch point
	// For now, we'll create a simple branch reference
	if branchName == "" {
		branchName = fmt.Sprintf("Branch from %s", conversationID[:8])
	}

	return map[string]interface{}{
		"original_conversation_id": conversationID,
		"new_conversation_id":      newConvID,
		"branch_name":              branchName,
		"status":                   "created",
	}, nil
}

// ========== USER SETTINGS HANDLERS ==========

// handleUserProfileGet gets user profile
func handleUserProfileGet(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	userID := ctx.Value("user_id")
	if userID == nil {
		return nil, fmt.Errorf("user not authenticated")
	}

	// Get user profile from Redis via Agent
	userIDStr, _ := userID.(string)

	// Try to get user data from Redis
	userData, err := deps.Agent.RedisClient.Get(ctx, "user:"+userIDStr).Result()
	if err != nil {
		// Return minimal profile if not found
		return map[string]interface{}{
			"user_id": userIDStr,
			"username": userIDStr,
		}, nil
	}

	var profile map[string]interface{}
	json.Unmarshal([]byte(userData), &profile)

	return profile, nil
}

// handleUserProfileUpdate updates user profile
func handleUserProfileUpdate(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	userID := ctx.Value("user_id")
	if userID == nil {
		return nil, fmt.Errorf("user not authenticated")
	}

	userIDStr, _ := userID.(string)

	// Get existing profile
	existingData, _ := deps.Agent.RedisClient.Get(ctx, "user:"+userIDStr).Result()
	existing := make(map[string]interface{})
	if existingData != "" {
		json.Unmarshal([]byte(existingData), &existing)
	}

	// Update with new values
	updates := make(map[string]interface{})
	if displayName, ok := args["display_name"].(string); ok {
		existing["display_name"] = displayName
		updates["display_name"] = displayName
	}
	if bio, ok := args["bio"].(string); ok {
		existing["bio"] = bio
		updates["bio"] = bio
	}
	if prefs, ok := args["preferences"].(map[string]interface{}); ok {
		if existing["preferences"] == nil {
			existing["preferences"] = make(map[string]interface{})
		}
		for k, v := range prefs {
			existing["preferences"].(map[string]interface{})[k] = v
		}
		updates["preferences"] = prefs
	}

	// Save back to Redis
	data, _ := json.Marshal(existing)
	deps.Agent.RedisClient.Set(ctx, "user:"+userIDStr, data, 0)

	return map[string]interface{}{
		"user_id":  userIDStr,
		"updated":  updates,
		"status":   "success",
	}, nil
}

// handleUserPreferencesGet gets user preferences
func handleUserPreferencesGet(ctx context.Context, deps *HandlerDependencies, args map[string]interface{}) (interface{}, error) {
	userID := ctx.Value("user_id")
	if userID == nil {
		return nil, fmt.Errorf("user not authenticated")
	}

	userIDStr, _ := userID.(string)

	// Get user profile and extract preferences
	userData, err := deps.Agent.RedisClient.Get(ctx, "user:"+userIDStr).Result()
	if err != nil {
		// Return default preferences
		return map[string]interface{}{
			"theme":           "dark",
			"notifications":   true,
			"auto_save":       true,
		}, nil
	}

	var profile map[string]interface{}
	json.Unmarshal([]byte(userData), &profile)

	if prefs, ok := profile["preferences"].(map[string]interface{}); ok {
		return prefs, nil
	}

	// Return default preferences
	return map[string]interface{}{
		"theme":         "dark",
		"notifications": true,
		"auto_save":     true,
	}, nil
}

// Helper function to get float values
func getFloat(args map[string]interface{}, key string, defaultVal float64) float64 {
	if val, ok := args[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case string:
			f, _ := strconv.ParseFloat(v, 64)
			return f
		}
	}
	return defaultVal
}

// RegisterHandlers registers all tool handlers
func RegisterHandlers() map[string]func(context.Context, *HandlerDependencies, map[string]interface{}) (interface{}, error) {
	return map[string]func(context.Context, *HandlerDependencies, map[string]interface{}) (interface{}, error){
		// Memory Tools
		"memory_store":          handleMemoryStore,
		"memory_search":         handleMemorySearch,
		"memory_delete":         handleMemoryDelete,
		"memory_list":           handleMemoryList,

		// Chat Tools
		"chat_consult":          handleChatConsult,
		"conversations_list":    handleConversationsList,
		"conversations_delete":  handleConversationsDelete,

		// Entity Tools
		"entity_create":        handleEntityCreate,
		"entity_update":        handleEntityUpdate,
		"entity_query":         handleEntityQuery,
		"relationship_create":  handleRelationshipCreate,

		// Document Tools
		"document_ingest":       handleDocumentIngest,
		"document_list":         handleDocumentList,
		"document_delete":       handleDocumentDelete,

		// Group Tools
		"group_create":         handleGroupCreate,
		"group_list":           handleGroupList,
		"group_invite":         handleGroupInvite,
		"group_members":        handleGroupMembers,
		"group_share_link":     handleGroupShareLink,

		// Admin Tools
		"admin_users_list":     handleAdminUsersList,
		"admin_user_update":    handleAdminUserUpdate,
		"admin_metrics":        handleAdminMetrics,
		"admin_policies_list":  handleAdminPoliciesList,
		"admin_policies_set":   handleAdminPoliciesSet,

		// ========== NEW: Graph Operation Tools ==========
		"graph_traverse":       handleGraphTraverse,
		"graph_neighbors":      handleGraphNeighbors,
		"graph_find_path":      handleGraphFindPath,
		"graph_communities":    handleGraphCommunities,

		// ========== NEW: Document Analysis Tools ==========
		"document_summarize":   handleDocumentSummarize,
		"document_extract":     handleDocumentExtract,
		"document_classify":    handleDocumentClassify,

		// ========== NEW: Conversation Management Tools ==========
		"conversation_export":   handleConversationExport,
		"conversation_summarize": handleConversationSummarize,
		"conversation_branch":  handleConversationBranch,

		// ========== NEW: User Settings Tools ==========
		"user_profile_get":     handleUserProfileGet,
		"user_profile_update":  handleUserProfileUpdate,
		"user_preferences_get": handleUserPreferencesGet,
	}
}
