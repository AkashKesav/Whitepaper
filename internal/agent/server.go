// Package agent provides HTTP/WebSocket handlers for the Front-End Agent.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Server provides HTTP and WebSocket endpoints for the agent
type Server struct {
	agent    *Agent
	logger   *zap.Logger
	upgrader websocket.Upgrader
}

// NewServer creates a new HTTP server for the agent
func NewServer(agent *Agent, logger *zap.Logger) *Server {
	return &Server{
		agent:  agent,
		logger: logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// SetupRoutes configures the HTTP routes
func (s *Server) SetupRoutes(r *mux.Router) {
	s.logger.Info("Registering routes...")

	// Create JWT middleware
	jwtMiddleware := NewJWTMiddleware(s.logger)

	// API Router
	api := r.PathPrefix("/api").Subrouter()

	// Public routes
	api.HandleFunc("/login", s.handleLogin).Methods("POST")
	api.HandleFunc("/register", s.handleRegister).Methods("POST")

	// Protected routes (Wrap with Middleware manually to avoid subrouter conflict)
	protect := func(h http.HandlerFunc) http.Handler {
		return jwtMiddleware.Middleware(h)
	}

	api.Handle("/chat", protect(s.handleChat)).Methods("POST")
	api.Handle("/stats", protect(s.handleStats)).Methods("GET")
	api.Handle("/conversations", protect(s.handleConversations)).Methods("GET")

	// Dashboard endpoints
	api.Handle("/dashboard/stats", protect(s.GetDashboardStats)).Methods("GET")
	api.Handle("/dashboard/graph", protect(s.GetVisualGraph)).Methods("GET")
	api.Handle("/dashboard/ingestion", protect(s.GetIngestionStats)).Methods("GET")

	// Document upload
	api.Handle("/upload", protect(s.handleUpload)).Methods("POST")

	// Groups
	api.Handle("/groups", protect(s.handleCreateGroup)).Methods("POST")
	api.Handle("/list-groups", protect(s.handleListGroups)).Methods("GET")
	api.Handle("/groups/{id}/members", protect(s.handleAddGroupMember)).Methods("POST")
	api.Handle("/groups/{id}/members/{username}", protect(s.handleRemoveGroupMember)).Methods("DELETE")
	api.Handle("/groups/{id}", protect(s.handleDeleteGroup)).Methods("DELETE")
	api.Handle("/groups/{id}/subusers", protect(s.handleCreateSubuser)).Methods("POST")

	// Workspace Collaboration
	api.Handle("/workspaces/{id}/invite", protect(s.handleInviteToWorkspace)).Methods("POST")
	api.Handle("/workspaces/{id}/share-link", protect(s.handleCreateShareLink)).Methods("POST")
	api.Handle("/workspaces/{id}/share-link/{token}", protect(s.handleRevokeShareLink)).Methods("DELETE")
	api.Handle("/workspaces/{id}/members", protect(s.handleGetWorkspaceMembers)).Methods("GET")
	api.Handle("/workspaces/{id}/members/{username}", protect(s.handleRemoveWorkspaceMember)).Methods("DELETE")
	api.Handle("/invitations", protect(s.handleGetPendingInvitations)).Methods("GET")
	api.Handle("/invitations/{id}/accept", protect(s.handleAcceptInvitation)).Methods("POST")
	api.Handle("/invitations/{id}/decline", protect(s.handleDeclineInvitation)).Methods("POST")
	api.Handle("/join/{token}", protect(s.handleJoinViaShareLink)).Methods("POST")

	// Health check (public, on root router or api?)
	r.HandleFunc("/health", s.handleHealth).Methods("GET")

	r.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		pathTemplate, _ := route.GetPathTemplate()
		methods, _ := route.GetMethods()
		s.logger.Info("Route registered", zap.String("path", pathTemplate), zap.Strings("methods", methods))
		return nil
	})

	// WebSocket for real-time chat (protected)
	wsRouter := r.PathPrefix("/ws").Subrouter()
	// wsRouter.Use(jwtMiddleware.Middleware) // WS middleware might differ, but assuming same
	// Gorilla Mux Middleware on WS might interfere with Upgrade?
	// Usually safe if passing header.
	// For now, let's just handle it. Note: WS usually needs query param token if headers not supported by client lib.
	// But let's assume standard Header.
	wsRouter.Handle("/chat", protect(s.handleWebSocketChat))
}

// ChatRequest represents an incoming chat request
type ChatRequest struct {
	UserID         string `json:"user_id"`
	ConversationID string `json:"conversation_id,omitempty"`
	Message        string `json:"message"`
	ContextType    string `json:"context_type,omitempty"` // "user" or "group"
	ContextID      string `json:"context_id,omitempty"`   // UserID or GroupID
}

// ChatResponse represents a chat response
type ChatResponse struct {
	ConversationID string `json:"conversation_id"`
	Response       string `json:"response"`
	LatencyMs      int64  `json:"latency_ms,omitempty"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents a login response with JWT token
type LoginResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	// Check if Redis is available
	if s.agent.RedisClient == nil {
		http.Error(w, "Authentication service unavailable", http.StatusServiceUnavailable)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	// Check if user already exists
	ctx := r.Context()
	exists, err := s.agent.RedisClient.Exists(ctx, "user:"+req.Username).Result()
	if err != nil {
		s.logger.Error("Failed to check user existence", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if exists > 0 {
		http.Error(w, "Username already taken", http.StatusConflict)
		return
	}

	// Hash password
	hashedPassword, err := HashPassword(req.Password)
	if err != nil {
		s.logger.Error("Failed to hash password", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Store user credentials in Redis
	if err := s.agent.RedisClient.Set(ctx, "user:"+req.Username, hashedPassword, 0).Err(); err != nil {
		s.logger.Error("Failed to store user", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Create User node in DGraph via Memory Kernel for Group V2 support
	if err := s.agent.mkClient.EnsureUserNode(ctx, req.Username); err != nil {
		s.logger.Warn("Failed to create User node in DGraph (groups may not work)", zap.Error(err))
		// Non-fatal: registration succeeds but groups may not work until user does first chat
	}

	// Generate JWT token
	token, err := GenerateToken(req.Username)
	if err != nil {
		s.logger.Error("Failed to generate token", zap.Error(err))
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	s.logger.Info("User registered", zap.String("username", req.Username))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Token:    token,
		Username: req.Username,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Check if Redis is available
	if s.agent.RedisClient == nil {
		http.Error(w, "Authentication service unavailable", http.StatusServiceUnavailable)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	// Get stored password hash from Redis
	ctx := r.Context()
	hashedPassword, err := s.agent.RedisClient.Get(ctx, "user:"+req.Username).Result()
	if err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	// Verify password
	if !CheckPassword(hashedPassword, req.Password) {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	// Generate JWT token
	token, err := GenerateToken(req.Username)
	if err != nil {
		s.logger.Error("Failed to generate token", zap.Error(err))
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	s.logger.Info("User logged in", zap.String("username", req.Username))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Token:    token,
		Username: req.Username,
	})
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get user ID from JWT context (set by middleware)
	userID := GetUserID(r.Context())
	s.logger.Debug("Processing chat request", zap.String("user_id", userID))

	// Determine Namespace
	namespace := fmt.Sprintf("user_%s", userID) // Default to private
	if req.ContextType == "group" && req.ContextID != "" {
		// Check if user is a member of the workspace
		isMember, err := s.agent.mkClient.IsWorkspaceMember(r.Context(), req.ContextID, userID)
		if err != nil {
			s.logger.Error("Failed to check workspace membership", zap.Error(err))
			http.Error(w, "Failed to verify workspace access", http.StatusInternalServerError)
			return
		}
		if !isMember {
			http.Error(w, "You are not a member of this workspace", http.StatusForbidden)
			return
		}
		namespace = req.ContextID
	}

	// Get or generate conversation ID
	conversationID := req.ConversationID
	if conversationID == "" {
		conversationID = uuid.New().String()
	}

	// Create context with timeout for AI service
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	response, err := s.agent.Chat(ctx, userID, conversationID, namespace, req.Message)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			s.logger.Warn("Chat timed out", zap.String("user_id", userID))
			http.Error(w, "Request timed out, please try again", http.StatusGatewayTimeout)
			return
		}
		s.logger.Error("Chat failed", zap.Error(err))
		http.Error(w, "Failed to generate response", http.StatusInternalServerError)
		return
	}

	resp := ChatResponse{
		ConversationID: conversationID,
		Response:       response,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.agent.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// ConversationSummary represents a conversation summary for the API
type ConversationSummary struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Namespace    string `json:"namespace"`
	UpdatedAt    string `json:"updated_at"`
	MessageCount int    `json:"message_count"`
}

// handleConversations returns the list of conversations for the current user
func (s *Server) handleConversations(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())

	// TODO: Implement actual conversation retrieval from DGraph/Redis
	// For now, return empty list (conversations are not persisted in current implementation)
	conversations := []ConversationSummary{}

	// Try to get recent conversation IDs from Redis if available
	if s.agent.RedisClient != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// Get conversation IDs pattern: conv:{userID}:*
		pattern := fmt.Sprintf("conv:%s:*", userID)
		keys, err := s.agent.RedisClient.Keys(ctx, pattern).Result()
		if err == nil && len(keys) > 0 {
			for _, key := range keys {
				// Extract conversation ID from key
				parts := key[len(fmt.Sprintf("conv:%s:", userID)):]
				if parts != "" {
					conversations = append(conversations, ConversationSummary{
						ID:           parts,
						Title:        "Chat",
						Namespace:    fmt.Sprintf("user_%s", userID),
						UpdatedAt:    time.Now().Format(time.RFC3339),
						MessageCount: 0,
					})
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"conversations": conversations,
	})
}

// UploadResponse represents the response for a document upload
type UploadResponse struct {
	Status   string `json:"status"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	Entities int    `json:"entities_extracted"`
	Message  string `json:"message"`
}

// handleUpload handles document upload for ingestion
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())

	// Parse multipart form (max 32MB)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing file in request", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		s.logger.Error("Failed to read file", zap.Error(err))
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Document upload received",
		zap.String("user", userID),
		zap.String("filename", header.Filename),
		zap.Int64("size", header.Size))

	// Get namespace for user
	namespace := fmt.Sprintf("user_%s", userID)
	if contextType := r.FormValue("context_type"); contextType == "group" {
		if contextID := r.FormValue("context_id"); contextID != "" {
			namespace = contextID
		}
	}

	// Process document via AI services extraction
	entities := 0
	if s.agent.aiClient == nil {
		s.logger.Warn("aiClient is nil, cannot extract entities")
	} else {
		// Call AI service to extract entities from document
		type ExtractRequest struct {
			UserQuery  string `json:"user_query"`
			AIResponse string `json:"ai_response"`
		}

		extractReq := ExtractRequest{
			UserQuery:  "Extract all people, companies, locations, and topics from this document",
			AIResponse: string(content),
		}

		reqBody, _ := json.Marshal(extractReq)
		resp, err := s.agent.aiClient.httpClient.Post(
			s.agent.aiClient.baseURL+"/extract",
			"application/json",
			bytes.NewReader(reqBody),
		)
		if err != nil {
			s.logger.Warn("AI extract request failed", zap.Error(err))
		} else if resp.StatusCode == 200 {
			defer resp.Body.Close()

			// Read the body first
			bodyBytes, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				s.logger.Warn("Failed to read extract response", zap.Error(readErr))
			} else {
				// Try to decode as object first (expected format: {"value": [...], "Count": N})
				var result struct {
					Value []interface{} `json:"value"`
					Count int           `json:"Count"`
				}
				if decodeErr := json.Unmarshal(bodyBytes, &result); decodeErr == nil {
					entities = len(result.Value)
					if result.Count > 0 {
						entities = result.Count
					}
				} else {
					// Try to decode as raw array
					var rawEntities []interface{}
					if decodeErr2 := json.Unmarshal(bodyBytes, &rawEntities); decodeErr2 == nil {
						entities = len(rawEntities)
					} else {
						s.logger.Warn("Failed to decode extract response",
							zap.Error(decodeErr),
							zap.String("body_preview", string(bodyBytes[:min(len(bodyBytes), 200)])))
					}
				}

				if entities > 0 {
					s.logger.Info("Extracted entities from document",
						zap.Int("count", entities),
						zap.String("filename", header.Filename))
				}
			}
		} else {
			s.logger.Warn("AI extract returned non-200", zap.Int("status", resp.StatusCode))
		}
	}

	// Log document processing for memory (actual kernel ingestion happens via chat flow)
	s.logger.Info("Document processed for user",
		zap.String("user", userID),
		zap.String("namespace", namespace),
		zap.Int("entities", entities))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UploadResponse{
		Status:   "success",
		Filename: header.Filename,
		Size:     header.Size,
		Entities: entities,
		Message:  fmt.Sprintf("Document '%s' uploaded and processed", header.Filename),
	})
}

// WebSocket message types
type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type WSChatPayload struct {
	Message     string `json:"message"`
	ContextType string `json:"context_type,omitempty"`
	ContextID   string `json:"context_id,omitempty"`
}

func (s *Server) handleWebSocketChat(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}

	// Get user ID from JWT context (set by middleware)
	userID := GetUserID(r.Context())
	conversationID := uuid.New().String()

	s.logger.Info("WebSocket connected",
		zap.String("user_id", userID),
		zap.String("conversation_id", conversationID))

	// Handle connection
	go s.handleWSConnection(conn, userID, conversationID)
}

func (s *Server) handleWSConnection(conn *websocket.Conn, userID, conversationID string) {
	defer conn.Close()

	var wsMu sync.Mutex

	for {
		var msg WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			s.logger.Debug("WebSocket read error", zap.Error(err))
			return
		}

		switch msg.Type {
		case "chat":
			var payload WSChatPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				continue
			}

			// Determine Namespace
			namespace := fmt.Sprintf("user_%s", userID)
			if payload.ContextType == "group" && payload.ContextID != "" {
				namespace = payload.ContextID
			}

			// Use context.Background() for async WS handler
			response, err := s.agent.Chat(context.Background(), userID, conversationID, namespace, payload.Message)
			if err != nil {
				s.logger.Error("Chat failed", zap.Error(err))
				continue
			}

			wsMu.Lock()
			conn.WriteJSON(map[string]interface{}{
				"type": "response",
				"payload": map[string]string{
					"response": response,
				},
			})
			wsMu.Unlock()

		case "typing":
			var payload struct {
				Message     string `json:"message"`
				ContextType string `json:"context_type"`
				ContextID   string `json:"context_id"`
			}
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				continue
			}

			// Determine Namespace
			namespace := fmt.Sprintf("user_%s", userID)
			if payload.ContextType == "group" && payload.ContextID != "" {
				namespace = payload.ContextID
			}

			// Trigger Speculation (Time Travel)
			s.logger.Debug("Received typing event", zap.String("msg", payload.Message))
			s.agent.Speculate(context.Background(), userID, namespace, payload.Message)

		case "ping":
			wsMu.Lock()
			conn.WriteJSON(map[string]string{"type": "pong"})
			wsMu.Unlock()
		}
	}
}

// ShareRequest represents a request to share a conversation
type ShareRequest struct {
	TargetUsername string `json:"target_username"`
	ConversationID string `json:"conversation_id"`
}

// CreateGroupRequest represents a request to create a group
type CreateGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CreateGroupResponse represents the response for group creation
type CreateGroupResponse struct {
	GroupID   string `json:"group_id"`
	Namespace string `json:"namespace"`
}

func (s *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	if userID == "anonymous" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Group name is required", http.StatusBadRequest)
		return
	}

	namespace, err := s.agent.mkClient.CreateGroup(r.Context(), req.Name, req.Description, userID)
	if err != nil {
		s.logger.Error("Failed to create group", zap.Error(err))
		http.Error(w, "Failed to create group", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CreateGroupResponse{
		GroupID:   namespace, // Using namespace as the ID for now
		Namespace: namespace,
	})
}

// AddMemberRequest represents a request to add a member
type AddMemberRequest struct {
	Username string `json:"username"`
}

func (s *Server) handleAddGroupMember(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context()) // Requester
	vars := mux.Vars(r)
	groupNamespace := vars["id"] // ID in URL is the namespace string

	var req AddMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 1. Check if Requester is Admin
	isAdmin, err := s.agent.mkClient.IsGroupAdmin(r.Context(), groupNamespace, userID)
	if err != nil {
		s.logger.Error("Failed to check admin status", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !isAdmin {
		http.Error(w, "Only admins can add members", http.StatusForbidden)
		return
	}

	// 2. Add Member
	if err := s.agent.mkClient.AddGroupMember(r.Context(), groupNamespace, req.Username); err != nil {
		s.logger.Error("Failed to add member", zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to add member: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "member_added"})
}

func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())

	groups, err := s.agent.mkClient.ListGroups(r.Context(), userID)
	if err != nil {
		s.logger.Error("Failed to list groups", zap.Error(err))
		http.Error(w, "Failed to list groups", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"groups": groups,
	})
}

func (s *Server) handleRemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	vars := mux.Vars(r)
	groupID := vars["id"]
	targetUser := vars["username"]

	// Check Admin
	isAdmin, err := s.agent.mkClient.IsGroupAdmin(r.Context(), groupID, userID)
	if err != nil {
		s.logger.Error("Failed to check admin status", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	// Allow removing self (leave group) or Admin removing others
	if !isAdmin && userID != targetUser {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := s.agent.mkClient.RemoveGroupMember(r.Context(), groupID, targetUser); err != nil {
		s.logger.Error("Failed to remove member", zap.Error(err))
		http.Error(w, "Failed to remove member", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	vars := mux.Vars(r)
	groupID := vars["id"]

	isAdmin, err := s.agent.mkClient.IsGroupAdmin(r.Context(), groupID, userID)
	if err != nil || !isAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := s.agent.mkClient.DeleteGroup(r.Context(), groupID); err != nil {
		s.logger.Error("Failed to delete group", zap.Error(err))
		http.Error(w, "Failed to delete group", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

type CreateSubuserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleCreateSubuser(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context()) // Requesting Admin
	vars := mux.Vars(r)
	groupID := vars["id"]

	// 1. Verify Admin
	isAdmin, err := s.agent.mkClient.IsGroupAdmin(r.Context(), groupID, userID)
	if err != nil || !isAdmin {
		http.Error(w, "Forbidden: Only admins can create subusers", http.StatusForbidden)
		return
	}

	var req CreateSubuserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 2. Register User (Redis)
	// Check existence
	ctx := r.Context()
	exists, err := s.agent.RedisClient.Exists(ctx, "user:"+req.Username).Result()
	if exists > 0 {
		http.Error(w, "Username already taken", http.StatusConflict)
		return // Or handle as "Add existing user" if desired, but request implies creation
	}

	hashedPassword, _ := HashPassword(req.Password)
	if err := s.agent.RedisClient.Set(ctx, "user:"+req.Username, hashedPassword, 0).Err(); err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// 3. Ensure User Node in Graph
	if err := s.agent.mkClient.EnsureUserNode(ctx, req.Username); err != nil {
		s.logger.Error("Failed to ensure user node", zap.Error(err))
		// Continue? If node missing, AddMember might fail or auto-create.
	}

	// 4. Add to Group
	if err := s.agent.mkClient.AddGroupMember(ctx, groupID, req.Username); err != nil {
		s.logger.Error("Failed to add subuser to group", zap.Error(err))
		http.Error(w, "User created but failed to join group", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "subuser_created", "username": req.Username})
}

// ============================================================================
// WORKSPACE COLLABORATION HANDLERS
// ============================================================================

// InviteRequest represents a request to invite a user to a workspace
type InviteRequest struct {
	Username string `json:"username"`
	Role     string `json:"role"` // "admin" or "subuser"
}

// InviteResponse represents the response for an invitation
type InviteResponse struct {
	InvitationID string `json:"invitation_id"`
	Status       string `json:"status"`
	Message      string `json:"message"`
}

func (s *Server) handleInviteToWorkspace(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	vars := mux.Vars(r)
	workspaceNS := vars["id"]

	// Check if user is admin
	isAdmin, err := s.agent.mkClient.IsGroupAdmin(r.Context(), workspaceNS, userID)
	if err != nil {
		s.logger.Error("Failed to check admin status", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !isAdmin {
		http.Error(w, "Only admins can invite users", http.StatusForbidden)
		return
	}

	var req InviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}

	// Default role to subuser
	if req.Role == "" {
		req.Role = "subuser"
	}

	invite, err := s.agent.mkClient.InviteToWorkspace(r.Context(), workspaceNS, userID, req.Username, req.Role)
	if err != nil {
		s.logger.Error("Failed to create invitation", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(InviteResponse{
		InvitationID: invite.UID,
		Status:       "pending",
		Message:      fmt.Sprintf("Invitation sent to %s", req.Username),
	})
}

func (s *Server) handleGetPendingInvitations(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())

	invitations, err := s.agent.mkClient.GetPendingInvitations(r.Context(), userID)
	if err != nil {
		s.logger.Error("Failed to get invitations", zap.Error(err))
		http.Error(w, "Failed to get invitations", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"invitations": invitations,
	})
}

func (s *Server) handleAcceptInvitation(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	vars := mux.Vars(r)
	invitationID := vars["id"]

	if err := s.agent.mkClient.AcceptInvitation(r.Context(), invitationID, userID); err != nil {
		s.logger.Error("Failed to accept invitation", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "accepted",
		"message": "You have joined the workspace",
	})
}

func (s *Server) handleDeclineInvitation(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	vars := mux.Vars(r)
	invitationID := vars["id"]

	if err := s.agent.mkClient.DeclineInvitation(r.Context(), invitationID, userID); err != nil {
		s.logger.Error("Failed to decline invitation", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "declined",
	})
}

// CreateShareLinkRequest represents a request to create a share link
type CreateShareLinkRequest struct {
	MaxUses        int `json:"max_uses"`         // 0 = unlimited
	ExpiresInHours int `json:"expires_in_hours"` // 0 = never
}

// ShareLinkResponse represents the response for a share link
type ShareLinkResponse struct {
	Token     string  `json:"token"`
	URL       string  `json:"url"`
	MaxUses   int     `json:"max_uses"`
	ExpiresAt *string `json:"expires_at,omitempty"`
}

func (s *Server) handleCreateShareLink(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	vars := mux.Vars(r)
	workspaceNS := vars["id"]

	// Check if user is admin
	isAdmin, err := s.agent.mkClient.IsGroupAdmin(r.Context(), workspaceNS, userID)
	if err != nil {
		s.logger.Error("Failed to check admin status", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !isAdmin {
		http.Error(w, "Only admins can create share links", http.StatusForbidden)
		return
	}

	var req CreateShareLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body (use defaults)
		req = CreateShareLinkRequest{}
	}

	var expiresAt *time.Time
	if req.ExpiresInHours > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresInHours) * time.Hour)
		expiresAt = &t
	}

	link, err := s.agent.mkClient.CreateShareLink(r.Context(), workspaceNS, userID, req.MaxUses, expiresAt)
	if err != nil {
		s.logger.Error("Failed to create share link", zap.Error(err))
		http.Error(w, "Failed to create share link", http.StatusInternalServerError)
		return
	}

	resp := ShareLinkResponse{
		Token:   link.Token,
		URL:     fmt.Sprintf("/api/join/%s", link.Token),
		MaxUses: link.MaxUses,
	}
	if link.ExpiresAt != nil {
		expStr := link.ExpiresAt.Format(time.RFC3339)
		resp.ExpiresAt = &expStr
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleJoinViaShareLink(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	if userID == "anonymous" {
		http.Error(w, "Authentication required to join via share link", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	token := vars["token"]

	link, err := s.agent.mkClient.JoinViaShareLink(r.Context(), token, userID)
	if err != nil {
		s.logger.Error("Failed to join via share link", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":       "joined",
		"workspace_id": link.WorkspaceID,
		"role":         link.Role,
	})
}

func (s *Server) handleRevokeShareLink(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	vars := mux.Vars(r)
	token := vars["token"]

	if err := s.agent.mkClient.RevokeShareLink(r.Context(), token, userID); err != nil {
		s.logger.Error("Failed to revoke share link", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "revoked",
	})
}

func (s *Server) handleGetWorkspaceMembers(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	vars := mux.Vars(r)
	workspaceNS := vars["id"]

	// Check if user is a member
	isMember, err := s.agent.mkClient.IsWorkspaceMember(r.Context(), workspaceNS, userID)
	if err != nil {
		s.logger.Error("Failed to check membership", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !isMember {
		http.Error(w, "You are not a member of this workspace", http.StatusForbidden)
		return
	}

	members, err := s.agent.mkClient.GetWorkspaceMembers(r.Context(), workspaceNS)
	if err != nil {
		s.logger.Error("Failed to get members", zap.Error(err))
		http.Error(w, "Failed to get members", http.StatusInternalServerError)
		return
	}

	// Convert to JSON-friendly format
	var memberList []map[string]interface{}
	for _, m := range members {
		memberList = append(memberList, map[string]interface{}{
			"username":  m.User.Name,
			"role":      m.Role,
			"joined_at": m.JoinedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"members": memberList,
	})
}

func (s *Server) handleRemoveWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	vars := mux.Vars(r)
	workspaceNS := vars["id"]
	targetUser := vars["username"]

	// Check if user is admin OR trying to leave themselves
	isAdmin, err := s.agent.mkClient.IsGroupAdmin(r.Context(), workspaceNS, userID)
	if err != nil {
		s.logger.Error("Failed to check admin status", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Allow removing self (leave workspace) or Admin removing others
	if !isAdmin && userID != targetUser {
		http.Error(w, "Only admins can remove other members", http.StatusForbidden)
		return
	}

	// Prevent admin from removing themselves if they're the only admin
	if userID == targetUser && isAdmin {
		members, _ := s.agent.mkClient.GetWorkspaceMembers(r.Context(), workspaceNS)
		adminCount := 0
		for _, m := range members {
			if m.Role == "admin" {
				adminCount++
			}
		}
		if adminCount <= 1 {
			http.Error(w, "Cannot leave workspace: you are the only admin", http.StatusBadRequest)
			return
		}
	}

	if err := s.agent.mkClient.RemoveGroupMember(r.Context(), workspaceNS, targetUser); err != nil {
		s.logger.Error("Failed to remove member", zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to remove member: %v", err), http.StatusInternalServerError)
		return
	}

	s.logger.Info("Member removed from workspace",
		zap.String("workspace", workspaceNS),
		zap.String("removed_user", targetUser),
		zap.String("by", userID))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "removed",
		"message": fmt.Sprintf("User %s removed from workspace", targetUser),
	})
}
