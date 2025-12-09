// Package agent provides HTTP/WebSocket handlers for the Front-End Agent.
package agent

import (
	"encoding/json"
	"net/http"
	"sync"

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
	// REST API
	r.HandleFunc("/api/chat", s.handleChat).Methods("POST")
	r.HandleFunc("/api/stats", s.handleStats).Methods("GET")
	r.HandleFunc("/health", s.handleHealth).Methods("GET")

	r.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		pathTemplate, _ := route.GetPathTemplate()
		methods, _ := route.GetMethods()
		s.logger.Info("Route registered", zap.String("path", pathTemplate), zap.Strings("methods", methods))
		return nil
	})

	// WebSocket for real-time chat
	r.HandleFunc("/ws/chat", s.handleWebSocketChat)
}

// ChatRequest represents an incoming chat request
type ChatRequest struct {
	UserID         string `json:"user_id"`
	ConversationID string `json:"conversation_id,omitempty"`
	Message        string `json:"message"`
}

// ChatResponse represents a chat response
type ChatResponse struct {
	ConversationID string `json:"conversation_id"`
	Response       string `json:"response"`
	LatencyMs      int64  `json:"latency_ms,omitempty"`
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get user ID from: 1) request body, 2) cookie, 3) generate new
	userID := req.UserID
	if userID == "" {
		// Check for existing session cookie
		if cookie, err := r.Cookie("rmk_user_id"); err == nil && cookie.Value != "" {
			userID = cookie.Value
		} else {
			// Generate new persistent user ID
			userID = uuid.New().String()
		}
	}

	// Set persistent cookie (expires in 1 year)
	http.SetCookie(w, &http.Cookie{
		Name:     "rmk_user_id",
		Value:    userID,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 365, // 1 year
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// Get or generate conversation ID
	conversationID := req.ConversationID
	if conversationID == "" {
		// Check for session cookie for conversation continuity
		if cookie, err := r.Cookie("rmk_conversation_id"); err == nil && cookie.Value != "" {
			conversationID = cookie.Value
		} else {
			conversationID = uuid.New().String()
		}
	}

	// Set conversation cookie (session-based, persists until browser closes)
	http.SetCookie(w, &http.Cookie{
		Name:     "rmk_conversation_id",
		Value:    conversationID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	response, err := s.agent.Chat(r.Context(), userID, conversationID, req.Message)
	if err != nil {
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

// WebSocket message types
type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type WSChatPayload struct {
	Message string `json:"message"`
}

func (s *Server) handleWebSocketChat(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = uuid.New().String()
	}
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

			response, err := s.agent.Chat(nil, userID, conversationID, payload.Message)
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

		case "ping":
			wsMu.Lock()
			conn.WriteJSON(map[string]string{"type": "pong"})
			wsMu.Unlock()
		}
	}
}
