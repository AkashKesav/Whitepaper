// Package agent provides MCP route integration for the agent server
package agent

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// MCPRequestHandler is an interface for handling MCP requests
// This avoids direct import of the mcp package to prevent import cycles
type MCPRequestHandler interface {
	HandleRequest(ctx context.Context, req MCPRequest) (MCPResponse, error)
}

// MCPRequest represents a JSON-RPC 2.0 request (minimal definition)
type MCPRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id,omitempty"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// MCPResponse represents a JSON-RPC 2.0 response (minimal definition)
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// SetupMCPRoutes adds MCP routes to the agent server
// This allows external initialization of MCP server without import cycle
func (s *Server) SetupMCPRoutes(r *mux.Router, mcpHandler MCPRequestHandler) {
	s.logger.Info("Registering MCP routes")

	// Create JWT middleware
	jwtMiddleware, err := NewJWTMiddleware(s.logger)
	if err != nil {
		s.logger.Error("Failed to create JWT middleware for MCP routes", zap.Error(err))
		return
	}

	// Protected routes wrapper
	protected := func(h http.HandlerFunc) http.Handler {
		return jwtMiddleware.Middleware(h)
	}

	// MCP endpoint - handles JSON-RPC 2.0 requests
	r.Handle("/api/mcp", protected(s.handleMCPRequest(mcpHandler))).Methods("POST")

	// Tools list endpoint (convenience method)
	r.Handle("/api/mcp/tools", protected(s.handleMCPToolsList(mcpHandler))).Methods("GET")

	// Tool call endpoint (convenience method)
	r.Handle("/api/mcp/tools/call", protected(s.handleMCPToolCall(mcpHandler))).Methods("POST")

	s.logger.Info("MCP routes registered successfully")
}

// handleMCPRequest handles MCP JSON-RPC requests
func (s *Server) handleMCPRequest(mcpHandler MCPRequestHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req MCPRequest
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

		// Delegate to MCP handler
		resp, err := mcpHandler.HandleRequest(r.Context(), req)
		if err != nil {
			resp = MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &MCPError{
					Code:    -32603,
					Message: err.Error(),
				},
			}
		}

		json.NewEncoder(w).Encode(resp)
	}
}

// handleMCPToolsList handles the tools/list endpoint
func (s *Server) handleMCPToolsList(mcpHandler MCPRequestHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/list",
		}

		resp, _ := mcpHandler.HandleRequest(r.Context(), req)
		json.NewEncoder(w).Encode(resp)
	}
}

// handleMCPToolCall handles the tools/call endpoint
func (s *Server) handleMCPToolCall(mcpHandler MCPRequestHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}

		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			s.logger.Error("Failed to decode tool call request", zap.Error(err))
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      nil,
				"error": map[string]interface{}{
					"code":    -32602,
					"message": "Invalid params",
				},
			})
			return
		}

		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params: map[string]interface{}{
				"name":      params.Name,
				"arguments": params.Arguments,
			},
		}

		resp, err := mcpHandler.HandleRequest(r.Context(), req)
		if err != nil {
			resp = MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &MCPError{
					Code:    -32603,
					Message: err.Error(),
				},
			}
		}

		json.NewEncoder(w).Encode(resp)
	}
}
