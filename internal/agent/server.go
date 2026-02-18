// Package agent provides HTTP/WebSocket handlers for the Front-End Agent.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/reflective-memory-kernel/internal/graph"
	"github.com/reflective-memory-kernel/internal/policy"
	"go.uber.org/zap"
)

// Server provides HTTP and WebSocket endpoints for the agent
type Server struct {
	agent          *Agent
	logger         *zap.Logger
	upgrader       websocket.Upgrader
	allowedOrigins []string  // Allowed origins for WebSocket connections
	groupLock      *GroupLockManager // Distributed lock manager for group operations
	crypto         *Crypto    // Encryption/decryption for sensitive user data
}

// NewServer creates a new HTTP server for the agent
func NewServer(agent *Agent, logger *zap.Logger, allowedOrigins ...string) *Server {
	// Default to localhost in development if no origins specified
	origins := allowedOrigins
	if len(origins) == 0 {
		origins = []string{"http://localhost:*"}
	}

	// Initialize group lock manager for distributed locking
	var groupLock *GroupLockManager
	if agent.RedisClient != nil {
		groupLock = NewGroupLockManager(agent.RedisClient, logger.Named("group_lock"))
	}

	// Initialize crypto for sensitive data encryption
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "default-dev-secret-change-in-production" // Fallback for development
		logger.Warn("Using default JWT secret for crypto - set JWT_SECRET in production")
	}
	crypto, err := NewCrypto(jwtSecret, logger.Named("crypto"))
	if err != nil {
		logger.Warn("Failed to initialize crypto, API key encryption will be disabled", zap.Error(err))
		crypto = nil
	}

	return &Server{
		agent:          agent,
		logger:         logger,
		allowedOrigins: origins,
		groupLock:      groupLock,
		crypto:         crypto,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				// Same-origin request (no Origin header) is allowed
				if origin == "" {
					return true
				}

				// Check against allowed origins
				for _, allowed := range origins {
					if allowed == "*" {
						return true  // Allow all (use only in development)
					}
					if originMatches(origin, allowed) {
						return true
					}
				}

				// Log rejected origin for security monitoring
				logger.Warn("WebSocket origin rejected",
					zap.String("origin", origin),
					zap.Strings("allowed", origins))
				return false
			},
		},
	}
}

// originMatches checks if an origin matches a pattern (supports wildcards)
func originMatches(origin, pattern string) bool {
	if origin == pattern {
		return true
	}
	// Wildcard matching (e.g., "https://*.example.com")
	if strings.Contains(pattern, "*") {
		patternRegex := strings.ReplaceAll(pattern, ".", "\\.")
		patternRegex = strings.ReplaceAll(patternRegex, "*", ".*")
		matched, _ := regexp.MatchString("^"+patternRegex+"$", origin)
		return matched
	}
	return false
}

// SetupRoutes configures the HTTP routes
func (s *Server) SetupRoutes(r *mux.Router) error {
	s.logger.Info("Registering routes...")
	fmt.Println("DEBUG: SetupRoutes called")

	// Create JWT middleware (now returns error for security)
	jwtMiddleware, err := NewJWTMiddleware(s.logger)
	if err != nil {
		return fmt.Errorf("failed to create JWT middleware: %w", err)
	}

	// Register Admin Routes (Must be before /api to avoid shadowing)
	s.SetupAdminRoutes(r, jwtMiddleware)

	// Register Policy Routes (Must be before /api to avoid shadowing)
	s.SetupPolicyRoutes(r, jwtMiddleware)

	// API Router
	api := r.PathPrefix("/api").Subrouter()

	// Bootstrap endpoint (creates initial admin user if system is empty)
	// This is a special endpoint that works before any users exist
	api.HandleFunc("/bootstrap", s.handleBootstrap).Methods("POST")

	// User Settings (Per-user encrypted API key storage) - registering on root router
	fmt.Println("DEBUG: Registering /profile/settings routes on root router")
	r.Handle("/api/profile/settings", jwtMiddleware.Middleware(s.rateLimitMiddleware(http.HandlerFunc(s.handleGetUserSettings)))).Methods("GET")
	r.Handle("/api/profile/settings", jwtMiddleware.Middleware(s.rateLimitMiddleware(http.HandlerFunc(s.handleSaveUserSettings)))).Methods("PUT")
	r.Handle("/api/profile/settings/keys/{provider}", jwtMiddleware.Middleware(s.rateLimitMiddleware(http.HandlerFunc(s.handleDeleteUserAPIKey)))).Methods("DELETE")
	fmt.Println("DEBUG: /profile/settings routes registered on root router")

	// Alias routes for /api/user/settings (compatibility with frontend)
	r.Handle("/api/user/settings", jwtMiddleware.Middleware(s.rateLimitMiddleware(http.HandlerFunc(s.handleGetUserSettings)))).Methods("GET")
	r.Handle("/api/user/settings", jwtMiddleware.Middleware(s.rateLimitMiddleware(http.HandlerFunc(s.handleSaveUserSettings)))).Methods("PUT")
	r.Handle("/api/user/settings/keys/{provider}", jwtMiddleware.Middleware(s.rateLimitMiddleware(http.HandlerFunc(s.handleDeleteUserAPIKey)))).Methods("DELETE")
	fmt.Println("DEBUG: /user/settings alias routes registered")

	// NIM API Key Test endpoint (public, for testing API key before saving)
	api.HandleFunc("/test/nim", s.handleTestNIM).Methods("POST")

	// TEST: Simple profile endpoint
	api.HandleFunc("/test/profile", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "profile test works"})
	}).Methods("GET")

	// Public Routes
	api.HandleFunc("/register", s.handleRegister).Methods("POST")
	api.HandleFunc("/login", s.handleLogin).Methods("POST")

	// Protected routes (Wrap with Middleware manually to avoid subrouter conflict)
	protected := func(h http.HandlerFunc) http.Handler {
		return jwtMiddleware.Middleware(s.rateLimitMiddleware(h))
	}

	protect := protected // Alias for backward compatibility if needed or just use protected

	api.Handle("/chat", protect(s.handleChat)).Methods("POST")
	api.Handle("/search", protect(s.handleSearch)).Methods("GET")
	api.Handle("/search/temporal", protect(s.handleTemporalQuery)).Methods("POST")
	api.Handle("/stats", protect(s.handleStats)).Methods("GET")
	api.Handle("/conversations", protect(s.handleConversations)).Methods("GET")

	// Dashboard endpoints
	api.Handle("/dashboard/stats", protect(s.GetDashboardStats)).Methods("GET")
	api.Handle("/dashboard/graph", protect(s.GetVisualGraph)).Methods("GET")
	api.Handle("/dashboard/ingestion", protect(s.GetIngestionStats)).Methods("GET")

	// Document upload
	api.Handle("/upload", protect(s.handleUpload)).Methods("POST")
	// Document deletion (by document ID)
	api.Handle("/documents/{id}", protect(s.handleDeleteDocument)).Methods("DELETE")
	// List documents
	api.Handle("/documents", protect(s.handleListDocuments)).Methods("GET")

	// Groups
	// SECURITY: Apply rate limiting to group management operations
	// Note: Rate limiting is applied inline in handlers to avoid type issues
	api.Handle("/groups", protect(s.handleCreateGroup)).Methods("POST")
	api.Handle("/groups", protect(s.handleListGroups)).Methods("GET")
	api.Handle("/list-groups", protect(s.handleListGroups)).Methods("GET") // Legacy endpoint
	api.Handle("/groups/{id}/members", protect(s.handleAddGroupMember)).Methods("POST")
	api.Handle("/groups/{id}/members", protect(s.handleGetGroupMembers)).Methods("GET")
	api.Handle("/groups/{id}/members/{username}", protect(s.handleRemoveGroupMember)).Methods("DELETE")
	api.Handle("/groups/{id}", protect(s.handleDeleteGroup)).Methods("DELETE")
	api.Handle("/groups/{id}/subusers", protect(s.handleCreateSubuser)).Methods("POST")

	// User listing (for invitation/member selection - available to all authenticated users)
	api.Handle("/users", protect(s.handleListUsers)).Methods("GET")

	// Workspace Collaboration
	api.Handle("/workspaces/{id}/invite", protect(s.handleInviteToWorkspace)).Methods("POST")
	api.Handle("/workspaces/{id}/share-link", protect(s.handleCreateShareLink)).Methods("POST")
	api.Handle("/workspaces/{id}/share-link/{token}", protect(s.handleRevokeShareLink)).Methods("DELETE")
	api.Handle("/workspaces/{id}/members", protect(s.handleGetWorkspaceMembers)).Methods("GET")
	api.Handle("/workspaces/{id}/members/{username}", protect(s.handleRemoveWorkspaceMember)).Methods("DELETE")
	api.Handle("/invitations", protect(s.handleGetPendingInvitations)).Methods("GET")
	api.Handle("/workspaces/{id}/invitations/sent", protect(s.handleGetWorkspaceSentInvitations)).Methods("GET")
	api.Handle("/invitations/{id}/accept", protect(s.handleAcceptInvitation)).Methods("POST")
	api.Handle("/invitations/{id}/decline", protect(s.handleDeclineInvitation)).Methods("POST")
	api.Handle("/join/{token}", protect(s.handleJoinViaShareLink)).Methods("POST")

	// Health check (public, on root router or api?)
	r.HandleFunc("/health", s.handleHealth).Methods("GET")

	// MCP (Model Context Protocol) endpoints
	api.Handle("/mcp", protect(s.handleMCPJSONRPC)).Methods("POST")
	api.Handle("/mcp/tools", protect(s.handleMCPGetTools)).Methods("GET")
	api.Handle("/mcp/tools/call", protect(s.handleMCPInvokeTool)).Methods("POST")

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

	return nil
}

// ChatRequest represents an incoming chat request
type ChatRequest struct {
	UserID         string `json:"user_id"`
	ConversationID string `json:"conversation_id,omitempty"`
	Message        string `json:"message"`
	ContextType    string `json:"context_type,omitempty"` // "user" or "group"
	ContextID      string `json:"context_id,omitempty"`   // UserID or GroupID
	Namespace      string `json:"namespace,omitempty"`    // Direct namespace specification (preferred)
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
	Role     string `json:"role"`
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleRegister registers a new user
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

	// Determine Role (Bootstrap Admin)
	role := "user"
	isAdminInit, _ := s.agent.RedisClient.Get(ctx, "system:admin_initialized").Result()
	if isAdminInit == "" || strings.HasPrefix(req.Username, "super_admin_") {
		role = "admin"
		if isAdminInit == "" {
			s.agent.RedisClient.Set(ctx, "system:admin_initialized", "true", 0)
		}
	}

	// Persist Role in Redis
	s.agent.RedisClient.Set(ctx, "user_role:"+req.Username, role, 0)

	// Create User node in DGraph via Memory Kernel for Group V2 support
	if err := s.agent.mkClient.EnsureUserNode(ctx, req.Username, role); err != nil {
		s.logger.Warn("Failed to create User node in DGraph (groups may not work)", zap.Error(err))
		// Non-fatal: registration succeeds but groups may not work until user does first chat
	}

	// Generate JWT token
	token, err := GenerateToken(req.Username, role)
	if err != nil {
		s.logger.Error("Failed to generate token", zap.Error(err))
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	s.logger.Info("User registered", zap.String("username", req.Username), zap.String("role", role))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Token:    token,
		Username: req.Username,
		Role:     role,
	})
}

// BootstrapRequest is the request for system bootstrap
type BootstrapRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email,omitempty"`
}

// handleBootstrap creates the initial admin user for a fresh system
// POST /api/bootstrap
// This endpoint only works if no users exist yet (system initialization)
func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if Redis is available
	if s.agent.RedisClient == nil {
		http.Error(w, "Authentication service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Check if system is already bootstrapped
	adminInit, _ := s.agent.RedisClient.Get(ctx, "system:admin_initialized").Result()
	if adminInit == "true" {
		http.Error(w, "System already initialized. Use /api/register to create new users.", http.StatusForbidden)
		return
	}

	var req BootstrapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	// Validate password strength (minimum 8 characters)
	if len(req.Password) < 8 {
		http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	// Check if user already exists (shouldn't happen on fresh system, but safety check)
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

	// Store user with admin role
	role := "admin"
	if err := s.agent.RedisClient.Set(ctx, "user:"+req.Username, hashedPassword, 0).Err(); err != nil {
		s.logger.Error("Failed to store user", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Mark system as initialized
	if err := s.agent.RedisClient.Set(ctx, "system:admin_initialized", "true", 0).Err(); err != nil {
		s.logger.Error("Failed to mark system as initialized", zap.Error(err))
	}

	// Store role
	s.agent.RedisClient.Set(ctx, "user_role:"+req.Username, role, 0)

	// Create User node in DGraph
	if err := s.agent.mkClient.EnsureUserNode(ctx, req.Username, role); err != nil {
		s.logger.Warn("Failed to create User node in DGraph", zap.Error(err))
	}

	// Generate JWT token
	token, err := GenerateToken(req.Username, role)
	if err != nil {
		s.logger.Error("Failed to generate token", zap.Error(err))
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	s.logger.Info("System bootstrapped with initial admin user",
		zap.String("username", req.Username))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(LoginResponse{
		Token:    token,
		Username: req.Username,
		Role:     role,
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

	// Get Role
	role, err := s.agent.RedisClient.Get(ctx, "user_role:"+req.Username).Result()
	if err != nil {
		role = "user"
	}

	// Generate JWT token
	token, err := GenerateToken(req.Username, role)
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
		Role:     role,
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
	// Priority: 1. req.Namespace (direct), 2. context_type/context_id (legacy), 3. default user namespace
	namespace := fmt.Sprintf("user_%s", userID) // Default to private

	if req.Namespace != "" {
		// Direct namespace specification (preferred by frontend)
		// SECURITY: Validate namespace access to prevent cross-namespace access
		if strings.HasPrefix(req.Namespace, "user_") {
			// Users can only access their own namespace
			expectedNamespace := fmt.Sprintf("user_%s", userID)
			if req.Namespace != expectedNamespace {
				s.logger.Warn("Attempted cross-namespace access denied",
					zap.String("user_id", userID),
					zap.String("requested_namespace", req.Namespace))
				http.Error(w, "Access denied: you can only access your own namespace", http.StatusForbidden)
				return
			}
		} else if strings.HasPrefix(req.Namespace, "group_") {
			// Verify group membership
			isMember, err := s.agent.mkClient.IsWorkspaceMember(r.Context(), req.Namespace, userID)
			if err != nil {
				s.logger.Error("Failed to check workspace membership", zap.Error(err))
				http.Error(w, "Failed to verify workspace access", http.StatusInternalServerError)
				return
			}
			if !isMember {
				s.logger.Warn("Attempted group access by non-member",
					zap.String("user_id", userID),
					zap.String("requested_namespace", req.Namespace))
				http.Error(w, "You are not a member of this workspace", http.StatusForbidden)
				return
			}
		} else {
			// Invalid namespace format
			s.logger.Warn("Invalid namespace format",
				zap.String("user_id", userID),
				zap.String("requested_namespace", req.Namespace))
			http.Error(w, "Invalid namespace format", http.StatusBadRequest)
			return
		}
		namespace = req.Namespace
	} else if req.ContextType == "group" && req.ContextID != "" {
		// Legacy context_type/context_id approach
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

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	// Get namespace from user context
	userID := GetUserID(r.Context())
	namespace := fmt.Sprintf("user_%s", userID)

	nodes, err := s.agent.mkClient.SearchNodes(r.Context(), namespace, query)
	if err != nil {
		s.logger.Error("Search failed", zap.Error(err))
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
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

	filename := header.Filename

	// SECURITY: Comprehensive file validation using FileValidator
	const maxFileSize = 10 << 20 // 10MB
	validator := NewFileValidator(maxFileSize, true)

	// 1. Validate filename (path traversal, Unicode homographs, control characters, etc.)
	if err := validator.ValidateFilename(filename); err != nil {
		s.logger.Warn("Invalid filename rejected",
			zap.String("filename", filename),
			zap.Error(err))
		http.Error(w, fmt.Sprintf("Invalid filename: %v", err), http.StatusBadRequest)
		return
	}

	// 2. Validate file extension is allowed
	if !validator.IsAllowedExtension(filename) {
		ext := strings.ToLower(filepath.Ext(filename))
		s.logger.Warn("File type not allowed",
			zap.String("filename", filename),
			zap.String("extension", ext))
		http.Error(w, fmt.Sprintf("File type '%s' is not allowed", ext), http.StatusBadRequest)
		return
	}

	// 3. Validate file size
	if err := validator.ValidateFileSize(header.Size); err != nil {
		s.logger.Warn("File size validation failed",
			zap.String("filename", filename),
			zap.Int64("size", header.Size),
			zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 4. Read file content with size limit
	content, err := io.ReadAll(io.LimitReader(file, maxFileSize))
	if err != nil {
		s.logger.Error("Failed to read file", zap.Error(err))
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// 5. Validate file content matches declared type (magic number check)
	if err := validator.ValidateFileContent(content, filename); err != nil {
		s.logger.Warn("File content validation failed",
			zap.String("filename", filename),
			zap.Error(err))
		http.Error(w, fmt.Sprintf("File validation failed: %v", err), http.StatusBadRequest)
		return
	}

	// 6. Scan for malware and suspicious content
	if err := validator.ScanForMalware(content, filename); err != nil {
		s.logger.Warn("File rejected by security scan",
			zap.String("filename", filename),
			zap.Error(err))
		http.Error(w, fmt.Sprintf("File rejected by security scan: %v", err), http.StatusBadRequest)
		return
	}

	s.logger.Info("Document upload validated successfully",
		zap.String("user", userID),
		zap.String("filename", filename),
		zap.Int64("size", header.Size))

	// Get namespace for user
	namespace := fmt.Sprintf("user_%s", userID)
	if contextType := r.FormValue("context_type"); contextType == "group" {
		if contextID := r.FormValue("context_id"); contextID != "" {
			namespace = contextID
		}
	}

	// Process document via AI services - Vector-Native Ingestion
	entities := 0
	chunks := 0
	relationships := 0

	if s.agent.aiClient == nil {
		s.logger.Warn("aiClient is nil, cannot ingest document")
	} else {
		// Call AI service /ingest endpoint for Vector-Native processing
		type IngestRequest struct {
			Text         string `json:"text"`
			DocumentType string `json:"document_type"`
		}

		ingestReq := IngestRequest{
			Text:         string(content),
			DocumentType: "text",
		}

		reqBody, _ := json.Marshal(ingestReq)
		resp, err := s.agent.aiClient.httpClient.Post(
			s.agent.aiClient.baseURL+"/ingest",
			"application/json",
			bytes.NewReader(reqBody),
		)
		if err != nil {
			s.logger.Warn("AI ingest request failed", zap.Error(err))
		} else if resp.StatusCode == 200 {
			defer resp.Body.Close()

			// Parse ingest response
			// Parse ingest response
			var result struct {
				Entities      []graph.ExtractedEntity `json:"entities"`
				Relationships []interface{}           `json:"relationships"`
				Chunks        []graph.DocumentChunk   `json:"chunks"`
				Stats         map[string]interface{}  `json:"stats"`
				Summary       string                  `json:"summary"`
				VectorTree    interface{}             `json:"vector_tree"`
			}
			if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr == nil {
				entities = len(result.Entities)
				relationships = len(result.Relationships)
				chunks = len(result.Chunks)

				s.logger.Info("Document ingested with Vector-Native processing",
					zap.Int("entities", entities),
					zap.Int("relationships", relationships),
					zap.Int("chunks", chunks),
					zap.String("filename", filename))

				// Persist Extracted Data
				ctx := context.Background()
				// Use filename as "conversation ID" context for now
				docContextID := fmt.Sprintf("doc_%s", filename)

				// 1. Persist Entities to DGraph
				if len(result.Entities) > 0 {
					if err := s.agent.mkClient.PersistEntities(ctx, namespace, userID, docContextID, result.Entities); err != nil {
						s.logger.Error("Failed to persist entities", zap.Error(err))
					} else {
						s.logger.Info("Persisted entities to DGraph", zap.Int("count", len(result.Entities)))
					}
				}

				// 2. Persist Chunks to Qdrant
				if len(result.Chunks) > 0 {
					// Use a unique docID for chunk namespacing
					docID := fmt.Sprintf("doc_%d_%s", time.Now().Unix(), filename)
					if err := s.agent.mkClient.PersistChunks(ctx, namespace, docID, result.Chunks); err != nil {
						s.logger.Error("Failed to persist chunks", zap.Error(err))
					} else {
						s.logger.Info("Persisted chunks to Qdrant", zap.Int("count", len(result.Chunks)))
					}
				}

			} else {
				s.logger.Warn("Failed to decode ingest response", zap.Error(decodeErr))
			}
		} else {
			s.logger.Warn("AI ingest returned non-200", zap.Int("status", resp.StatusCode))
		}
	}

	// Log document processing
	s.logger.Info("Document processed for user",
		zap.String("user", userID),
		zap.String("namespace", namespace),
		zap.Int("entities", entities))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UploadResponse{
		Status:   "success",
		Filename: filename,
		Size:     header.Size,
		Entities: entities,
		Message:  fmt.Sprintf("Document '%s' uploaded and processed (%d entities, %d chunks)", filename, entities, chunks),
	})
}

// DocumentInfo represents a document in the system
type DocumentInfo struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Namespace  string    `json:"namespace"`
	CreatedAt  time.Time `json:"created_at"`
	Size       int64     `json:"size"`
	EntityCount int      `json:"entity_count"`
}

// DocumentListResponse is the response for listing documents
type DocumentListResponse struct {
	Documents []DocumentInfo `json:"documents"`
	Total     int            `json:"total"`
}

// handleListDocuments returns a list of documents for the current user
// GET /api/documents
func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	ctx := r.Context()

	// Determine namespace (default to user's namespace)
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = fmt.Sprintf("user_%s", userID)
	}

	// SECURITY: For group namespaces, verify user is a member
	if strings.HasPrefix(namespace, "group_") {
		isMember, err := s.agent.mkClient.IsWorkspaceMember(ctx, namespace, userID)
		if err != nil || !isMember {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
	}

	// Query for document-type nodes in the namespace
	query := `query Documents($namespace: string) {
		nodes(func: type(Document), orderdesc: created_at) @filter(eq(namespace, $namespace)) {
			uid
			name
			namespace
			created_at
			description
		}
	}`

	resp, err := s.agent.mkClient.GetGraphClient().Query(ctx, query, map[string]string{"$namespace": namespace})
	if err != nil {
		s.logger.Error("Failed to query documents", zap.Error(err))
		http.Error(w, "Failed to query documents", http.StatusInternalServerError)
		return
	}

	var result struct {
		Nodes []struct {
			UID        string `json:"uid"`
			Name       string `json:"name"`
			Namespace  string `json:"namespace"`
			CreatedAt  string `json:"created_at"`
			Description string `json:"description"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		s.logger.Error("Failed to unmarshal documents", zap.Error(err))
		http.Error(w, "Failed to parse documents", http.StatusInternalServerError)
		return
	}

	// Convert to DocumentInfo
	documents := make([]DocumentInfo, 0, len(result.Nodes))
	for _, node := range result.Nodes {
		timestamp, _ := time.Parse(time.RFC3339, node.CreatedAt)

		// Parse entity count from description if available
		entityCount := 0
		if node.Description != "" {
			fmt.Sscanf(node.Description, "Document with %d entities", &entityCount)
		}

		documents = append(documents, DocumentInfo{
			ID:          node.UID,
			Name:        node.Name,
			Namespace:   node.Namespace,
			CreatedAt:   timestamp,
			EntityCount: entityCount,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DocumentListResponse{
		Documents: documents,
		Total:     len(documents),
	})
}

// handleDeleteDocument deletes a document and its associated data
// DELETE /api/documents/{id}
func (s *Server) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	vars := mux.Vars(r)
	documentUID := vars["id"]

	if documentUID == "" {
		http.Error(w, "Document ID is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Get the document node to verify ownership
	node, err := s.agent.mkClient.GetGraphClient().GetNode(ctx, documentUID)
	if err != nil {
		s.logger.Error("Failed to get document", zap.Error(err))
		http.Error(w, "Document not found", http.StatusNotFound)
		return
	}

	// SECURITY: Verify the user owns this document (namespace check)
	expectedNamespace := fmt.Sprintf("user_%s", userID)
	if node.Namespace != expectedNamespace {
		// Also check if it's a group namespace where user is a member
		if strings.HasPrefix(node.Namespace, "group_") {
			isMember, err := s.agent.mkClient.IsWorkspaceMember(ctx, node.Namespace, userID)
			if err != nil || !isMember {
				http.Error(w, "Access denied", http.StatusForbidden)
				return
			}
		} else {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
	}

	// Delete the document node (this cascades to delete edges)
	if err := s.agent.mkClient.GetGraphClient().DeleteNode(ctx, documentUID, node.Namespace); err != nil {
		s.logger.Error("Failed to delete document", zap.Error(err))
		http.Error(w, "Failed to delete document", http.StatusInternalServerError)
		return
	}

	// Note: Chunks in Qdrant would need to be deleted separately in a production system
	// For now, the graph node deletion removes the document from the knowledge graph

	s.logger.Info("Document deleted",
		zap.String("document_id", documentUID),
		zap.String("document_name", node.Name),
		zap.String("deleted_by", userID))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "deleted",
		"id":      documentUID,
		"message": fmt.Sprintf("Document '%s' deleted successfully", node.Name),
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

			// SECURITY: Verify user has access to group namespace
			if strings.HasPrefix(namespace, "group_") {
				isMember, err := s.agent.mkClient.IsWorkspaceMember(context.Background(), namespace, userID)
				if err != nil {
					s.logger.Error("Failed to verify workspace membership", zap.Error(err))
					wsMu.Lock()
					conn.WriteJSON(map[string]interface{}{
						"type": "error",
						"payload": map[string]string{
							"error": "Failed to verify access",
						},
					})
					wsMu.Unlock()
					continue
				}
				if !isMember {
					s.logger.Warn("WebSocket access denied: user not in workspace",
						zap.String("user", userID),
						zap.String("workspace", namespace))
					wsMu.Lock()
					conn.WriteJSON(map[string]interface{}{
						"type": "error",
						"payload": map[string]string{
							"error": "You are not a member of this workspace",
						},
					})
					wsMu.Unlock()
					continue
				}
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

			// SECURITY: Verify user has access to group namespace
			if strings.HasPrefix(namespace, "group_") {
				isMember, err := s.agent.mkClient.IsWorkspaceMember(context.Background(), namespace, userID)
				if err != nil || !isMember {
					s.logger.Warn("WebSocket typing access denied: user not in workspace",
						zap.String("user", userID),
						zap.String("workspace", namespace))
					continue
				}
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

	// SECURITY: Validate group name to prevent injection and other attacks
	if req.Name == "" {
		http.Error(w, "Group name is required", http.StatusBadRequest)
		return
	}
	// Limit name length to prevent abuse
	if len(req.Name) > 100 {
		http.Error(w, "Group name must be 100 characters or less", http.StatusBadRequest)
		return
	}
	// Check for suspicious patterns (potential injection)
	if strings.ContainsAny(req.Name, "\x00\n\r<>\"'`)") {
		http.Error(w, "Group name contains invalid characters", http.StatusBadRequest)
		return
	}
	// Validate description
	if len(req.Description) > 500 {
		http.Error(w, "Description must be 500 characters or less", http.StatusBadRequest)
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

	// SECURITY: Acquire distributed lock BEFORE checking admin status
	// This prevents TOCTOU race conditions between admin check and member addition
	var lock *GroupOperationLock
	if s.groupLock != nil {
		var err error
		lock, err = s.groupLock.AcquireGroupLock(r.Context(), groupNamespace, "add_member")
		if err != nil {
			s.logger.Warn("Could not acquire group lock, operation may have conflicts",
				zap.Error(err),
				zap.String("group", groupNamespace))
			// Continue without lock if Redis is unavailable (fail open for availability)
		} else {
			defer lock.Release()
		}
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

	// 2. Check if user exists in Redis (primary user store)
	exists, err := s.agent.RedisClient.Exists(r.Context(), "user:"+req.Username).Result()
	if err != nil {
		s.logger.Error("Failed to check user existence", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if exists == 0 {
		http.Error(w, "User not found", http.StatusBadRequest) // Generic message to prevent enumeration
		return
	}

	// 3. Get user role and ensure they exist in DGraph
	userRole, _ := s.agent.RedisClient.Get(r.Context(), "user_role:"+req.Username).Result()
	if userRole == "" {
		userRole = "user"
	}
	if err := s.agent.mkClient.EnsureUserNode(r.Context(), req.Username, userRole); err != nil {
		s.logger.Error("Failed to ensure user node", zap.Error(err))
		http.Error(w, "Failed to prepare user", http.StatusInternalServerError)
		return
	}

	// 4. Add Member
	if err := s.agent.mkClient.AddGroupMember(r.Context(), groupNamespace, req.Username); err != nil {
		s.logger.Error("Failed to add member", zap.Error(err))
		http.Error(w, "Failed to add member", http.StatusInternalServerError) // Generic message
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

// handleListUsers returns all registered users (for invitation/member selection)
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if s.agent.RedisClient == nil {
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()

	// Get all user keys from Redis
	keys, err := s.agent.RedisClient.Keys(ctx, "user:*").Result()
	if err != nil {
		s.logger.Error("Failed to fetch users", zap.Error(err))
		http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
		return
	}

	var users []map[string]string
	for _, key := range keys {
		username := key[5:] // Remove "user:" prefix

		// Get user role
		role, err := s.agent.RedisClient.Get(ctx, "user_role:"+username).Result()
		if err != nil {
			role = "user"
		}

		users = append(users, map[string]string{
			"username": username,
			"role":     role,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users": users,
	})
}

func (s *Server) handleRemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	vars := mux.Vars(r)
	groupID := vars["id"]
	targetUser := vars["username"]

	// SECURITY: Acquire distributed lock BEFORE any checks
	// This prevents TOCTOU race conditions in admin verification and member removal
	var lock *GroupOperationLock
	if s.groupLock != nil {
		var err error
		lock, err = s.groupLock.AcquireGroupLock(r.Context(), groupID, "remove_member")
		if err != nil {
			s.logger.Warn("Could not acquire group lock, operation may have conflicts",
				zap.Error(err),
				zap.String("group", groupID))
			// Continue without lock if Redis is unavailable
		} else {
			defer lock.Release()
		}
	}

	// Check if user is admin
	isAdmin, err := s.agent.mkClient.IsGroupAdmin(r.Context(), groupID, userID)
	if err != nil {
		s.logger.Error("Failed to check admin status", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// SECURITY: Only admins can remove other members; users can only remove themselves
	if !isAdmin {
		if userID != targetUser {
			s.logger.Warn("Unauthorized group member removal attempt",
				zap.String("actor", userID),
				zap.String("target", targetUser),
				zap.String("group", groupID))
			http.Error(w, "Forbidden: Only admins can remove other members", http.StatusForbidden)
			return
		}
		// User is removing themselves (leaving group) - continue
	}

	// SECURITY: Prevent last admin from leaving the group
	if userID == targetUser && isAdmin {
		members, err := s.agent.mkClient.GetGroupMembers(r.Context(), groupID)
		if err == nil {
			adminCount := 0
			for _, m := range members {
				if role, ok := m["role"].(string); ok && role == "admin" {
					adminCount++
				}
			}
			if adminCount <= 1 {
				http.Error(w, "Cannot leave: you are the last admin", http.StatusBadRequest)
				return
			}
		}
	}

	if err := s.agent.mkClient.RemoveGroupMember(r.Context(), groupID, targetUser); err != nil {
		s.logger.Error("Failed to remove member", zap.Error(err))
		http.Error(w, "Failed to remove member", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := GetUserID(ctx)
	vars := mux.Vars(r)
	groupID := vars["id"]

	// SECURITY: Check system admin OR group admin
	// System admins can delete any group; group admins can only delete their own groups
	userRole := GetUserRole(ctx)
	isAuthorized := false

	if userRole == "admin" {
		// System admin can delete any group
		isAuthorized = true
	} else {
		// Check if user is group admin
		isGroupAdmin, err := s.agent.mkClient.IsGroupAdmin(ctx, groupID, userID)
		if err == nil && isGroupAdmin {
			isAuthorized = true
		}
	}

	if !isAuthorized {
		s.logger.Warn("Unauthorized group deletion attempt",
			zap.String("user", userID),
			zap.String("role", userRole),
			zap.String("group", groupID))
		http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
		return
	}

	if err := s.agent.mkClient.DeleteGroup(ctx, groupID, userID); err != nil {
		s.logger.Error("Failed to delete group", zap.Error(err))
		http.Error(w, "Failed to delete group", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleGetGroupMembers returns the members of a group
func (s *Server) handleGetGroupMembers(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	vars := mux.Vars(r)
	groupID := vars["id"]

	// Check if user is a member or admin of this group
	groups, err := s.agent.mkClient.ListGroups(r.Context(), userID)
	if err != nil {
		s.logger.Error("Failed to check group membership", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Check if the user belongs to this group
	isMember := false
	for _, g := range groups {
		if ns, ok := g["namespace"].(string); ok && ns == groupID {
			isMember = true
			break
		}
		if name, ok := g["name"].(string); ok && name == groupID {
			isMember = true
			break
		}
	}
	if !isMember {
		http.Error(w, "Forbidden: You are not a member of this group", http.StatusForbidden)
		return
	}

	// Get group members via direct kernel access
	members, err := s.agent.mkClient.GetGroupMembers(r.Context(), groupID)
	if err != nil {
		s.logger.Error("Failed to get group members", zap.Error(err))
		http.Error(w, "Failed to get group members", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"group_id": groupID,
		"members":  members,
	})
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
	if err := s.agent.mkClient.EnsureUserNode(ctx, req.Username, "user"); err != nil {
		s.logger.Error("Failed to ensure user node", zap.Error(err))
		// Continue? If node missing, AddMember might fail or auto-create.
	}

	// Persist Role in Redis
	s.agent.RedisClient.Set(ctx, "user_role:"+req.Username, "user", 0)

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

	// Check if user exists in Redis (primary user store)
	exists, err := s.agent.RedisClient.Exists(r.Context(), "user:"+req.Username).Result()
	if err != nil {
		s.logger.Error("Failed to check user existence in Redis", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if exists == 0 {
		http.Error(w, fmt.Sprintf("User '%s' not found", req.Username), http.StatusBadRequest)
		return
	}

	// Get user role from Redis
	userRole, _ := s.agent.RedisClient.Get(r.Context(), "user_role:"+req.Username).Result()
	if userRole == "" {
		userRole = "user"
	}

	// Ensure user node exists in DGraph before creating invitation
	if err := s.agent.mkClient.EnsureUserNode(r.Context(), req.Username, userRole); err != nil {
		s.logger.Error("Failed to ensure user node", zap.Error(err), zap.String("username", req.Username))
		http.Error(w, "Failed to prepare user for invitation", http.StatusInternalServerError)
		return
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

func (s *Server) handleGetWorkspaceSentInvitations(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	vars := mux.Vars(r)
	workspaceNS := vars["id"]

	// Check if user is a member of this workspace
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

	invitations, err := s.agent.mkClient.GetWorkspaceSentInvitations(r.Context(), workspaceNS)
	if err != nil {
		s.logger.Error("Failed to get workspace invitations", zap.Error(err))
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

	// SECURITY: Acquire distributed lock BEFORE checking admin status
	// This prevents TOCTOU race where concurrent requests could leave workspace with zero admins
	var lock *GroupOperationLock
	if s.groupLock != nil {
		var err error
		lock, err = s.groupLock.AcquireGroupLock(r.Context(), workspaceNS, "remove_member")
		if err != nil {
			s.logger.Warn("Could not acquire workspace lock, operation may have conflicts",
				zap.String("workspace", workspaceNS),
				zap.Error(err))
			// Continue without lock if Redis is unavailable (fail open for availability)
		} else {
			defer lock.Release()
		}
	}

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
	// SECURITY: This check is now atomic with removal due to distributed lock
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
		http.Error(w, "Failed to remove member", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Member removed from workspace",
		zap.String("workspace", workspaceNS),
		zap.String("removed_user", targetUser),
		zap.String("by", userID))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "removed",
	})
}

// rateLimitMiddleware applies rate limiting policies
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.agent.PolicyManager == nil || s.agent.PolicyManager.RateLimiter == nil {
			next.ServeHTTP(w, r)
			return
		}

		userID := GetUserID(r.Context())
		if userID == "" {
			// Skip rate limiting for unauthenticated users (or limit by IP if needed)
			next.ServeHTTP(w, r)
			return
		}

		// MVP: Default everyone to Pro tier
		// Future: Look up user tier from Redis/Graph
		tier := policy.TierPro

		result, err := s.agent.PolicyManager.RateLimiter.Allow(r.Context(), userID, tier, r.URL.Path)
		if err != nil {
			s.logger.Error("Rate limit check failed", zap.Error(err))
			// Fail open
			next.ServeHTTP(w, r)
			return
		}

		if !result.Allowed {
			w.Header().Set("Retry-After", fmt.Sprintf("%.0f", result.RetryAfter.Seconds()))
			http.Error(w, fmt.Sprintf("Rate limit exceeded. Try again in %s.", result.RetryAfter), 429)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Group operation rate limiting constants
const (
	// Group operations are privileged and should be strictly rate limited
	GroupOperationRateLimit = 10  // requests per minute
	GroupOperationBurst     = 3   // burst allowance
	GroupOperationWindow    = time.Minute
)

// groupOperationRateLimiter applies stricter rate limiting for group operations
// SECURITY: Prevents abuse of group management functionality (DoS, enumeration)
func (s *Server) groupOperationRateLimiter() mux.MiddlewareFunc {
	// Simple in-memory rate limiter using map and channel cleanup
	// For production, use Redis-backed rate limiting
	type userRateLimit struct {
		count    int
		resetAt  time.Time
		lastSeen time.Time
	}

	limiter := make(map[string]*userRateLimit)
	var mu sync.Mutex

	// Cleanup goroutine
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			now := time.Now()
			for userID, rl := range limiter {
				if now.After(rl.resetAt.Add(5 * time.Minute)) {
					delete(limiter, userID)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserID(r.Context())
			if userID == "" {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			mu.Lock()
			defer mu.Unlock()

			now := time.Now()
			rl, exists := limiter[userID]

			if !exists || now.After(rl.resetAt) {
				// First request or window expired
				limiter[userID] = &userRateLimit{
					count:   1,
					resetAt: now.Add(GroupOperationWindow),
				}
				next.ServeHTTP(w, r)
				return
			}

			// Check if under rate limit
			if rl.count < GroupOperationRateLimit {
				rl.count++
				rl.lastSeen = now
				next.ServeHTTP(w, r)
				return
			}

			// Rate limit exceeded
			retryAfter := rl.resetAt.Sub(now)
			w.Header().Set("Retry-After", fmt.Sprintf("%.0f", retryAfter.Seconds()))
			s.logger.Warn("Group operation rate limit exceeded",
				zap.String("user", userID),
				zap.Int("count", rl.count))

			http.Error(w, fmt.Sprintf("Too many group operations. Try again in %s.",
				retryAfter.Round(time.Second)), http.StatusTooManyRequests)
		})
	}
}

// MCP Handlers

// handleMCPJSONRPC handles the main MCP JSON-RPC endpoint
func (s *Server) handleMCPJSONRPC(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		JSONRPC string                 `json:"jsonrpc"`
		ID      interface{}            `json:"id,omitempty"`
		Method  string                 `json:"method"`
		Params  map[string]interface{} `json:"params,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("Failed to decode MCP request", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      nil,
			"error": map[string]interface{}{
				"code":    -32700,
				"message": "Parse error",
			},
		})
		return
	}

	// Handle different MCP methods
	var result interface{}
	var err error

	switch req.Method {
	case "initialize":
		result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]bool{},
			},
			"serverInfo": map[string]string{
				"name":    "reflective-memory-kernel",
				"version": "1.0.0",
			},
		}
	case "tools/list":
		result = map[string]interface{}{
			"tools": []map[string]interface{}{
				{
					"name":        "memory_store",
					"description": "Store a memory in the knowledge graph",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"content": map[string]interface{}{
								"type":        "string",
								"description": "The content to store",
							},
							"node_type": map[string]interface{}{
								"type":        "string",
								"description": "Type of node (Fact, Entity, Concept, etc.)",
								"default":     "Fact",
							},
						},
						"required": []string{"content"},
					},
				},
				{
					"name":        "memory_search",
					"description": "Search memories in the knowledge graph",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"query": map[string]interface{}{
								"type":        "string",
								"description": "Search query",
							},
							"limit": map[string]interface{}{
								"type":        "integer",
								"description": "Maximum results",
								"default":     10,
							},
						},
						"required": []string{"query"},
					},
				},
				{
					"name":        "chat",
					"description": "Chat with the AI assistant with memory context",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"message": map[string]interface{}{
								"type":        "string",
								"description": "Message to send",
							},
						},
						"required": []string{"message"},
					},
				},
			},
		}
	case "tools/call":
		name, _ := req.Params["name"].(string)
		arguments, _ := req.Params["arguments"].(map[string]interface{})
		result, err = s.handleMCPToolCallInner(r.Context(), name, arguments)
	case "ping":
		result = map[string]interface{}{"status": "ok"}
	default:
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"error": map[string]interface{}{
				"code":    -32601,
				"message": fmt.Sprintf("method not found: %s", req.Method),
			},
		})
		return
	}

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req.ID,
	}

	if err != nil {
		response["error"] = map[string]interface{}{
			"code":    -32603,
			"message": err.Error(),
		}
	} else {
		response["result"] = result
	}

	json.NewEncoder(w).Encode(response)
}

// handleMCPGetTools handles GET /api/mcp/tools
func (s *Server) handleMCPGetTools(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tools := []map[string]interface{}{
		{
			"name":        "memory_search",
			"description": "Search memories in the knowledge graph",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "chat",
			"description": "Chat with the AI assistant with memory context",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message": map[string]interface{}{
						"type":        "string",
						"description": "Message to send",
					},
				},
				"required": []string{"message"},
			},
		},
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"tools": tools,
	})
}

// handleMCPInvokeTool handles POST /api/mcp/tools/call
func (s *Server) handleMCPInvokeTool(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid request body",
		})
		return
	}

	result, err := s.handleMCPToolCallInner(r.Context(), req.Name, req.Arguments)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"result": result,
	})
}

// handleMCPToolCallInner implements the actual tool calling logic
func (s *Server) handleMCPToolCallInner(ctx context.Context, name string, arguments map[string]interface{}) (interface{}, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return nil, fmt.Errorf("unauthorized")
	}

	namespace := fmt.Sprintf("user_%s", userID)

	s.logger.Info("MCP tool called",
		zap.String("tool", name),
		zap.String("user", userID),
		zap.String("namespace", namespace))

	switch name {
	case "memory_search":
		query, _ := arguments["query"].(string)
		if query == "" {
			return nil, fmt.Errorf("query is required")
		}

		// Search memories using MKClient
		nodes, err := s.agent.mkClient.SearchNodes(ctx, namespace, query)
		if err != nil {
			return nil, fmt.Errorf("search failed: %w", err)
		}

		return map[string]interface{}{
			"query":   query,
			"count":   len(nodes),
			"results": nodes,
		}, nil

	case "chat":
		message, _ := arguments["message"].(string)
		if message == "" {
			return nil, fmt.Errorf("message is required")
		}

		// Use the Consult method for chat
		req := &graph.ConsultationRequest{
			Namespace: namespace,
			Query:     message,
		}
		response, err := s.agent.mkClient.Consult(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("chat failed: %w", err)
		}

		return map[string]interface{}{
			"response":  response.SynthesizedBrief,
			"facts":     len(response.RelevantFacts),
			"insights":  len(response.Insights),
			"confidence": response.Confidence,
		}, nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// NIMTestRequest represents a request to test NVIDIA NIM API key
type NIMTestRequest struct {
	APIKey string `json:"api_key"`
}

// NIMTestResponse represents the response from NIM API test
type NIMTestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Model   string `json:"model,omitempty"`
}

// handleTestNIM tests a NVIDIA NIM API key by making a simple chat request
// POST /api/test/nim
func (s *Server) handleTestNIM(w http.ResponseWriter, r *http.Request) {
	var req NIMTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.APIKey == "" {
		http.Error(w, "API key is required", http.StatusBadRequest)
		return
	}

	// Create a test NIM client
	testClient := NewNIMClient(NIMConfig{
		APIKey:  req.APIKey,
		Timeout: 30 * time.Second,
	}, s.logger.Named("nim_test"))

	// Test with a simple chat request
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	testMessages := []ChatMessage{
		{Role: "user", Content: "Hello, this is a connection test."},
	}

	resp, err := testClient.Chat(ctx, testMessages)
	if err != nil {
		s.logger.Warn("NIM API key test failed", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(NIMTestResponse{
			Success: false,
			Message: fmt.Sprintf("Connection failed: %v", err),
		})
		return
	}

	s.logger.Info("NIM API key test succeeded",
		zap.String("model", resp.Model),
		zap.Int("tokens", resp.Usage.TotalTokens))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(NIMTestResponse{
		Success: true,
		Message: "Connection successful!",
		Model:   resp.Model,
	})
}

// ============================================================================
// USER SETTINGS HANDLERS (Per-User Encrypted API Key Storage)
// ============================================================================

// UserSettingsRequest represents user settings save request
type UserSettingsRequest struct {
	NimApiKey       string `json:"nim_api_key,omitempty"`
	OpenaiApiKey    string `json:"openai_api_key,omitempty"`
	AnthropicApiKey string `json:"anthropic_api_key,omitempty"`
	GlmApiKey       string `json:"glm_api_key,omitempty"`
	Theme           string `json:"theme,omitempty"`
}

// UserSettingsResponse represents user settings response (never returns actual keys)
type UserSettingsResponse struct {
	HasNimKey            bool   `json:"has_nim_key"`
	HasOpenaiKey         bool   `json:"has_openai_key"`
	HasAnthropicKey      bool   `json:"has_anthropic_key"`
	HasGlmKey            bool   `json:"has_glm_key"`
	Theme                string `json:"theme"`
	NotificationsEnabled bool   `json:"notifications_enabled"`
	UpdatedAt            string `json:"updated_at"`
}

// handleGetUserSettings retrieves current user's settings
// GET /api/user/settings
// Returns key status (configured/not configured) but never the actual keys
func (s *Server) handleGetUserSettings(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())

	// Get settings from DGraph
	settings, err := s.agent.mkClient.GetUserSettings(r.Context(), userID)
	if err != nil {
		s.logger.Error("Failed to get user settings", zap.Error(err))
		http.Error(w, "Failed to retrieve settings", http.StatusInternalServerError)
		return
	}

	// Build response (never return actual API keys, only status)
	// Note: DGraph may store empty strings as "[]" for list-type predicates
	resp := UserSettingsResponse{
		HasNimKey:            settings.NimApiKeyEncrypted != "" && settings.NimApiKeyEncrypted != "[]",
		HasOpenaiKey:         settings.OpenaiApiKeyEncrypted != "" && settings.OpenaiApiKeyEncrypted != "[]",
		HasAnthropicKey:      settings.AnthropicApiKeyEncrypted != "" && settings.AnthropicApiKeyEncrypted != "[]",
		HasGlmKey:            settings.GlmApiKeyEncrypted != "" && settings.GlmApiKeyEncrypted != "[]",
		Theme:                settings.Theme,
		NotificationsEnabled: settings.NotificationsEnabled,
		UpdatedAt:            settings.UpdatedAt.Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleSaveUserSettings saves user settings with encrypted API keys
// PUT /api/user/settings
func (s *Server) handleSaveUserSettings(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())

	var req UserSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get existing settings to preserve unchanged fields
	existingSettings, err := s.agent.mkClient.GetUserSettings(r.Context(), userID)
	if err != nil {
		s.logger.Warn("Failed to get existing settings, creating new", zap.Error(err))
		existingSettings = &graph.UserSettings{
			UserID:               userID,
			Theme:                "dark",
			NotificationsEnabled: true,
		}
	}

	// Encrypt and store API keys
	settings := &graph.UserSettings{
		UserID:               userID,
		Theme:                existingSettings.Theme,
		NotificationsEnabled: existingSettings.NotificationsEnabled,
		UpdatedAt:            time.Now(),
	}

	// Update theme if provided
	if req.Theme != "" {
		settings.Theme = req.Theme
	}

	// Encrypt and store NIM API key
	if req.NimApiKey != "" {
		if s.crypto == nil {
			s.logger.Warn("Crypto not initialized, cannot encrypt API key")
			http.Error(w, "Encryption service unavailable", http.StatusServiceUnavailable)
			return
		}
		encrypted, err := s.crypto.Encrypt(req.NimApiKey)
		if err != nil {
			s.logger.Error("Failed to encrypt NIM API key", zap.Error(err))
			http.Error(w, "Failed to encrypt API key", http.StatusInternalServerError)
			return
		}
		settings.NimApiKeyEncrypted = encrypted
		s.logger.Info("NIM API key encrypted for user",
			zap.String("user", userID),
			zap.String("key_preview", MaskAPIKey(req.NimApiKey)))
	} else {
		settings.NimApiKeyEncrypted = existingSettings.NimApiKeyEncrypted
	}

	// Encrypt and store OpenAI API key
	if req.OpenaiApiKey != "" {
		if s.crypto == nil {
			http.Error(w, "Encryption service unavailable", http.StatusServiceUnavailable)
			return
		}
		encrypted, err := s.crypto.Encrypt(req.OpenaiApiKey)
		if err != nil {
			s.logger.Error("Failed to encrypt OpenAI API key", zap.Error(err))
			http.Error(w, "Failed to encrypt API key", http.StatusInternalServerError)
			return
		}
		settings.OpenaiApiKeyEncrypted = encrypted
		s.logger.Info("OpenAI API key encrypted for user",
			zap.String("user", userID),
			zap.String("key_preview", MaskAPIKey(req.OpenaiApiKey)))
	} else {
		settings.OpenaiApiKeyEncrypted = existingSettings.OpenaiApiKeyEncrypted
	}

	// Encrypt and store Anthropic API key
	if req.AnthropicApiKey != "" {
		if s.crypto == nil {
			http.Error(w, "Encryption service unavailable", http.StatusServiceUnavailable)
			return
		}
		encrypted, err := s.crypto.Encrypt(req.AnthropicApiKey)
		if err != nil {
			s.logger.Error("Failed to encrypt Anthropic API key", zap.Error(err))
			http.Error(w, "Failed to encrypt API key", http.StatusInternalServerError)
			return
		}
		settings.AnthropicApiKeyEncrypted = encrypted
		s.logger.Info("Anthropic API key encrypted for user",
			zap.String("user", userID),
			zap.String("key_preview", MaskAPIKey(req.AnthropicApiKey)))
	} else {
		settings.AnthropicApiKeyEncrypted = existingSettings.AnthropicApiKeyEncrypted
	}

	// Encrypt and store GLM API key
	if req.GlmApiKey != "" {
		if s.crypto == nil {
			http.Error(w, "Encryption service unavailable", http.StatusServiceUnavailable)
			return
		}
		encrypted, err := s.crypto.Encrypt(req.GlmApiKey)
		if err != nil {
			s.logger.Error("Failed to encrypt GLM API key", zap.Error(err))
			http.Error(w, "Failed to encrypt API key", http.StatusInternalServerError)
			return
		}
		settings.GlmApiKeyEncrypted = encrypted
		s.logger.Info("GLM API key encrypted for user",
			zap.String("user", userID),
			zap.String("key_preview", MaskAPIKey(req.GlmApiKey)))
	} else {
		settings.GlmApiKeyEncrypted = existingSettings.GlmApiKeyEncrypted
	}

	// Save to DGraph
	if err := s.agent.mkClient.StoreUserSettings(r.Context(), userID, settings); err != nil {
		s.logger.Error("Failed to store user settings", zap.Error(err))
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	s.logger.Info("User settings saved",
		zap.String("user", userID),
		zap.Bool("has_nim_key", settings.NimApiKeyEncrypted != ""),
		zap.Bool("has_openai_key", settings.OpenaiApiKeyEncrypted != ""),
		zap.Bool("has_anthropic_key", settings.AnthropicApiKeyEncrypted != ""),
		zap.Bool("has_glm_key", settings.GlmApiKeyEncrypted != ""))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "saved",
		"message": "Settings saved successfully",
	})
}

// handleDeleteUserAPIKey deletes a specific API key from user settings
// DELETE /api/user/settings/keys/{provider}
func (s *Server) handleDeleteUserAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	vars := mux.Vars(r)
	provider := vars["provider"]

	// Validate provider
	validProviders := map[string]bool{
		"nim":       true,
		"openai":    true,
		"anthropic": true,
		"glm":       true,
	}
	if !validProviders[provider] {
		http.Error(w, "Invalid provider", http.StatusBadRequest)
		return
	}

	// Delete the key
	if err := s.agent.mkClient.DeleteUserAPIKey(r.Context(), userID, provider); err != nil {
		s.logger.Error("Failed to delete user API key",
			zap.Error(err),
			zap.String("provider", provider),
			zap.String("user", userID))
		http.Error(w, "Failed to delete API key", http.StatusInternalServerError)
		return
	}

	s.logger.Info("User API key deleted",
		zap.String("user", userID),
		zap.String("provider", provider))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "deleted",
		"message": fmt.Sprintf("%s API key deleted successfully", strings.ToUpper(provider)),
	})
}
