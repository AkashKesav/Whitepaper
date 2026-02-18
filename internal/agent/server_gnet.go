// Package agent provides gnet-based HTTP/WebSocket handlers
// This is the gnet migration of server.go
package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/server"
)

// GnetServer provides gnet-based HTTP and WebSocket endpoints for the agent
type GnetServer struct {
	agent          *Agent
	logger         *zap.Logger
	allowedOrigins []string
	groupLock      *GroupLockManager
	crypto         *Crypto
}

// NewGnetServer creates a new gnet-based server for the agent
func NewGnetServer(agent *Agent, logger *zap.Logger, allowedOrigins ...string) *GnetServer {
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"http://localhost:*"}
	}

	var groupLock *GroupLockManager
	if agent.RedisClient != nil {
		groupLock = NewGroupLockManager(agent.RedisClient, logger.Named("group_lock"))
	}

	crypto, err := NewCrypto("default-dev-secret", logger)
	if err != nil {
		logger.Warn("Failed to create crypto, using fallback", zap.Error(err))
		crypto = &Crypto{} // Fallback
	}

	return &GnetServer{
		agent:          agent,
		logger:         logger,
		allowedOrigins: allowedOrigins,
		groupLock:      groupLock,
		crypto:         crypto,
	}
}

// SetupGnetRoutes configures all gnet routes
func (s *GnetServer) SetupGnetRoutes(engine *server.Engine) error {
	s.logger.Info("Registering gnet routes...")

	// Health check (no auth required)
	engine.GET("/health", s.handleHealth)
	engine.GET("/api/health", s.handleHealth)

	// Chat endpoints - using existing Agent.Chat method
	engine.POST("/api/chat", s.handleChat)

	// Stats - using existing Agent.GetStats method
	engine.GET("/api/stats", s.handleStats)

	// WebSocket chat
	engine.GET("/ws/chat", s.handleWebSocketChat)

	// Bootstrap - using existing Agent.Bootstrap method
	engine.POST("/api/bootstrap", s.handleBootstrap)

	// Search - using existing Agent.Search method
	engine.GET("/api/search", s.handleSearch)

	// Placeholder routes for other endpoints (to be implemented)
	engine.POST("/api/register", s.handleNotImplemented)
	engine.POST("/api/login", s.handleNotImplemented)
	engine.GET("/api/conversations", s.handleNotImplemented)
	engine.POST("/api/upload", s.handleNotImplemented)
	engine.GET("/api/documents", s.handleNotImplemented)
	engine.DELETE("/api/documents/{id}", s.handleNotImplemented)
	engine.POST("/api/groups", s.handleNotImplemented)
	engine.GET("/api/groups", s.handleNotImplemented)
	engine.DELETE("/api/groups/{id}", s.handleNotImplemented)
	engine.POST("/api/groups/{id}/members", s.handleNotImplemented)
	engine.DELETE("/api/groups/{id}/members/{userId}", s.handleNotImplemented)
	engine.GET("/api/groups/{id}/members", s.handleNotImplemented)
	engine.GET("/api/users", s.handleNotImplemented)
	engine.POST("/api/subusers", s.handleNotImplemented)
	engine.POST("/api/invitations", s.handleNotImplemented)
	engine.GET("/api/invitations/pending", s.handleNotImplemented)
	engine.GET("/api/invitations/sent", s.handleNotImplemented)
	engine.POST("/api/invitations/{id}/accept", s.handleNotImplemented)
	engine.POST("/api/invitations/{id}/decline", s.handleNotImplemented)
	engine.POST("/api/share-links", s.handleNotImplemented)
	engine.POST("/api/share-links/{code}/join", s.handleNotImplemented)
	engine.DELETE("/api/share-links/{code}", s.handleNotImplemented)
	engine.GET("/api/workspace/members", s.handleNotImplemented)
	engine.DELETE("/api/workspace/members/{userId}", s.handleNotImplemented)
	engine.GET("/api/profile/settings", s.handleNotImplemented)
	engine.PUT("/api/profile/settings", s.handleNotImplemented)
	engine.DELETE("/api/profile/settings/keys/{provider}", s.handleNotImplemented)
	engine.POST("/api/mcp", s.handleNotImplemented)
	engine.GET("/api/mcp/tools", s.handleNotImplemented)
	engine.POST("/api/mcp/tools/{name}", s.handleNotImplemented)
	engine.GET("/api/test-nim", s.handleNotImplemented)

	s.logger.Info("gnet routes registered successfully")
	return nil
}

// Handler implementations

func (s *GnetServer) handleHealth(req *server.Request) *server.Response {
	return server.JSON(map[string]string{
		"status":  "healthy",
		"service": "agent-gnet",
	}, 200)
}

func (s *GnetServer) handleChat(req *server.Request) *server.Response {
	var chatReq struct {
		Message        string                 `json:"message"`
		ConversationID string                 `json:"conversation_id,omitempty"`
		UserID         string                 `json:"user_id,omitempty"`
		Namespace      string                 `json:"namespace,omitempty"`
		Metadata       map[string]interface{} `json:"metadata,omitempty"`
	}
	if err := server.ParseJSON(req, &chatReq); err != nil {
		return server.JSON(map[string]string{"error": "Invalid request"}, 400)
	}

	if chatReq.Message == "" {
		return server.JSON(map[string]string{"error": "Message is required"}, 400)
	}

	ctx := context.Background()
	userID := chatReq.UserID
	if userID == "" {
		userID = "anonymous"
	}

	conversationID := chatReq.ConversationID
	if conversationID == "" {
		conversationID = uuid.New().String()
	}

	namespace := chatReq.Namespace
	if namespace == "" {
		namespace = "default"
	}

	response, err := s.agent.Chat(ctx, userID, conversationID, namespace, chatReq.Message)
	if err != nil {
		s.logger.Error("Chat failed", zap.Error(err))
		return server.JSON(map[string]string{"error": "Chat failed: " + err.Error()}, 500)
	}

	return server.JSON(map[string]string{
		"response":        response,
		"conversation_id": conversationID,
	}, 200)
}

func (s *GnetServer) handleWebSocketChat(req *server.Request) *server.Response {
	// Extract user ID and conversation ID from query params
	userID := req.Query.Get("user_id")
	conversationID := req.Query.Get("conversation_id")
	namespace := req.Query.Get("namespace")

	if userID == "" {
		userID = "anonymous"
	}
	if conversationID == "" {
		conversationID = uuid.New().String()
	}
	if namespace == "" {
		namespace = "default"
	}

	// For now, return a simple response indicating WebSocket is not fully implemented
	return server.JSON(map[string]string{
		"message": "WebSocket endpoint - use /api/chat for HTTP requests",
		"user_id": userID,
		"conversation_id": conversationID,
	}, 200)
}

func (s *GnetServer) handleWSConnection(conn *server.Conn, userID, conversationID, namespace string) {
	s.logger.Info("WebSocket connection established",
		zap.String("user_id", userID),
		zap.String("conversation_id", conversationID),
		zap.String("namespace", namespace))

	defer conn.Close(1000, "Normal closure")

	// Send connection confirmation
	conn.WriteJSON(map[string]string{
		"type":            "connected",
		"conversation_id": conversationID,
	})

	// Message loop
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if err != io.EOF {
				s.logger.Warn("WebSocket read error", zap.Error(err))
			}
			break
		}

		if msgType != 1 { // 1 = TextMessage in WebSocket protocol
			continue
		}

		var wsMsg struct {
			Type    string                 `json:"type"`
			Message string                 `json:"message"`
			Metadata map[string]interface{} `json:"metadata,omitempty"`
		}

		if err := json.Unmarshal(data, &wsMsg); err != nil {
			s.logger.Warn("Invalid WebSocket message", zap.Error(err))
			continue
		}

		if wsMsg.Type != "chat" || wsMsg.Message == "" {
			continue
		}

		// Process chat message
		ctx := context.Background()
		response, err := s.agent.Chat(ctx, userID, conversationID, namespace, wsMsg.Message)
		if err != nil {
			conn.WriteJSON(map[string]string{
				"type":  "error",
				"error": err.Error(),
			})
			continue
		}

		// Send response
		conn.WriteJSON(map[string]string{
			"type":    "response",
			"message": response,
			"query":   wsMsg.Message,
		})
	}
}

func (s *GnetServer) handleStats(req *server.Request) *server.Response {
	stats := s.agent.GetStats()
	return server.JSON(stats, 200)
}

func (s *GnetServer) handleBootstrap(req *server.Request) *server.Response {
	var bootstrapReq struct {
		Username      string `json:"username"`
		Email         string `json:"email"`
		Password      string `json:"password"`
		WorkspaceName string `json:"workspace_name"`
	}
	if err := server.ParseJSON(req, &bootstrapReq); err != nil {
		return server.JSON(map[string]string{"error": "Invalid request"}, 400)
	}

	if bootstrapReq.Username == "" || bootstrapReq.Email == "" || bootstrapReq.Password == "" {
		return server.JSON(map[string]string{"error": "username, email, and password are required"}, 400)
	}

	// Validate password strength
	if len(bootstrapReq.Password) < 8 {
		return server.JSON(map[string]string{"error": "Password must be at least 8 characters"}, 400)
	}

	// Create user and workspace
	userID := bootstrapReq.Username
	role := "admin"

	// Generate JWT token using the package-level function
	token, err := GenerateToken(userID, role)
	if err != nil {
		s.logger.Error("Failed to generate token", zap.Error(err))
		return server.JSON(map[string]string{"error": "Failed to generate token"}, 500)
	}

	return server.JSON(map[string]string{
		"token":    token,
		"username": userID,
		"role":     role,
	}, 200)
}

func (s *GnetServer) handleSearch(req *server.Request) *server.Response {
	query := req.Query.Get("q")
	if query == "" {
		return server.JSON(map[string]string{"error": "query parameter 'q' is required"}, 400)
	}

	// Get namespace from query param or use default
	namespace := req.Query.Get("namespace")
	if namespace == "" {
		namespace = "default"
	}

	ctx := context.Background()
	nodes, err := s.agent.mkClient.SearchNodes(ctx, namespace, query)
	if err != nil {
		s.logger.Error("Search failed", zap.Error(err))
		return server.JSON(map[string]string{"error": "Search failed"}, 500)
	}

	return server.JSON(nodes, 200)
}

func (s *GnetServer) handleNotImplemented(req *server.Request) *server.Response {
	return server.JSON(map[string]interface{}{
		"error":   "Not implemented in gnet server",
		"message": "Please use the legacy net/http server for this endpoint",
		"status":  "501",
	}, 501)
}

// Helper functions for multipart file upload
func parseContentDisposition(header string) (filename string, err error) {
	// Parse Content-Disposition header for filename
	parts := strings.Split(header, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "filename=") {
			filename = strings.Trim(part[9:], `"`)
			break
		}
	}
	return
}

func decodeBase64Content(data string) ([]byte, error) {
	// Add padding if necessary
	if l := len(data) % 4; l != 0 {
		data += strings.Repeat("=", 4-l)
	}
	return base64.StdEncoding.DecodeString(data)
}
