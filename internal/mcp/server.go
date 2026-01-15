// Package mcp implements the Model Context Protocol (MCP) server for RMK
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/reflective-memory-kernel/internal/agent"
	"go.uber.org/zap"
)

// Server implements the MCP server
type Server struct {
	logger   *zap.Logger
	agent    *agent.Agent
	handlers map[string]ToolHandler
	tools    []Tool
	serverInfo ServerInfo
}

// ServerInfo contains server metadata
type ServerInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Protocol    string `json:"protocol"`
	Capabilities Capabilities `json:"capabilities"`
}

// Capabilities declares server capabilities
type Capabilities struct {
	Tools     *ToolCapabilities     `json:"tools,omitempty"`
	Resources *ResourceCapabilities `json:"resources,omitempty"`
	Prompts   *PromptCapabilities   `json:"prompts,omitempty"`
}

// ToolCapabilities for tools support
type ToolCapabilities struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourceCapabilities for resources support
type ResourceCapabilities struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptCapabilities for prompts support
type PromptCapabilities struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ServerConfig configures the MCP server
type ServerConfig struct {
	Logger *zap.Logger
	Agent  *agent.Agent
	Name   string
	Version string
}

// NewServer creates a new MCP server
func NewServer(config ServerConfig) *Server {
	name := config.Name
	if name == "" {
		name = "reflective-memory-kernel"
	}
	version := config.Version
	if version == "" {
		version = "1.0.0"
	}

	s := &Server{
		logger: config.Logger,
		agent:  config.Agent,
		handlers: make(map[string]ToolHandler),
		tools: ToolSchemas(),
		serverInfo: ServerInfo{
			Name:     name,
			Version:  version,
			Protocol: "2024-11-05",
			Capabilities: Capabilities{
				Tools: &ToolCapabilities{},
			},
		},
	}

	// Register all tool handlers
	s.registerHandlers()

	return s
}

// registerHandlers registers tool handlers from schemas
func (s *Server) registerHandlers() {
	// Get raw handler functions
	rawHandlers := RegisterHandlers()

	deps := &HandlerDependencies{
		Agent: s.agent,
		Logger: s.logger,
	}

	// Wrap raw handlers to bind dependencies
	for toolName, handler := range rawHandlers {
		// Create closure capturing dependencies
		depsCopy := deps
		s.handlers[toolName] = func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return handler(ctx, depsCopy, args)
		}
	}
}

// HandleRequest processes an incoming MCP JSON-RPC request
func (s *Server) HandleRequest(ctx context.Context, req MCPRequest) (MCPResponse, error) {
	s.logger.Debug("MCP request received",
		zap.String("method", req.Method),
		zap.Any("id", req.ID))

	var result interface{}
	var err error

	switch req.Method {
	case "initialize":
		result, err = s.handleInitialize(ctx, req)
	case "initialized":
		// Client notification - no response needed
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
		}, nil
	case "tools/list":
		result, err = s.handleListTools(ctx, req)
	case "tools/call":
		result, err = s.handleToolCall(ctx, req)
	case "notifications/initialized":
		// Client notification after initialization
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
		}, nil
	case "notifications/cancelled":
		// Request cancellation notification
		s.logger.Debug("Request cancelled", zap.Any("id", req.ID))
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
		}, nil
	case "ping":
		result = map[string]interface{}{"status": "ok"}
	default:
		err = &MCPErrorObj{
			Code:    -32601,
			Message: fmt.Sprintf("method not found: %s", req.Method),
		}
	}

	if err != nil {
		s.logger.Error("MCP request error",
			zap.String("method", req.Method),
			zap.Error(err))

		// Check if it's already an MCPErrorObj
		if mcpErr, ok := err.(*MCPErrorObj); ok {
			return MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &MCPError{
					Code:    mcpErr.Code,
					Message: mcpErr.Message,
					Data:    mcpErr.Data,
				},
			}, mcpErr
		}

		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32603,
				Message: err.Error(),
			},
		}, err
	}

	return MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}, nil
}

// MCPErrorObj represents an error object
type MCPErrorObj struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *MCPErrorObj) Error() string {
	return e.Message
}

// handleInitialize handles the initialize request
func (s *Server) handleInitialize(ctx context.Context, req MCPRequest) (interface{}, error) {
	// Parse initialize params if provided
	var params InitializeParams
	if req.Params != nil {
		paramsData, _ := json.Marshal(req.Params)
		_ = json.Unmarshal(paramsData, &params)
	}

	s.logger.Info("MCP server initialized",
		zap.String("protocol", params.ProtocolVersion),
		zap.Any("client", params.ClientInfo))

	response := InitializeResponse{
		ProtocolVersion: "2024-11-05",
		Capabilities: map[string]interface{}{
			"tools": map[string]bool{
				"listChanged": false,
			},
		},
		ServerInfo: map[string]string{
			"name":    s.serverInfo.Name,
			"version": s.serverInfo.Version,
		},
	}

	return response, nil
}

// handleListTools handles the tools/list request
func (s *Server) handleListTools(ctx context.Context, req MCPRequest) (interface{}, error) {
	tools := make([]ToolDefinition, 0, len(s.tools))
	for _, tool := range s.tools {
		tools = append(tools, tool.Definition)
	}

	s.logger.Debug("Tools listed", zap.Int("count", len(tools)))

	return ListToolsResponse{
		Tools: tools,
	}, nil
}

// handleToolCall handles the tools/call request
func (s *Server) handleToolCall(ctx context.Context, req MCPRequest) (interface{}, error) {
	// Parse tool call parameters
	var params CallToolParams
	if req.Params == nil {
		return nil, &MCPErrorObj{
			Code:    -32602,
			Message: "invalid params: missing call parameters",
		}
	}

	paramsData, err := json.Marshal(req.Params)
	if err != nil {
		return nil, &MCPErrorObj{
			Code:    -32602,
			Message: "invalid params: failed to parse parameters",
		}
	}

	if err := json.Unmarshal(paramsData, &params); err != nil {
		return nil, &MCPErrorObj{
			Code:    -32602,
			Message: fmt.Sprintf("invalid params: %v", err),
		}
	}

	if params.Name == "" {
		return nil, &MCPErrorObj{
			Code:    -32602,
			Message: "invalid params: missing tool name",
		}
	}

	// Get handler
	handler, ok := s.handlers[params.Name]
	if !ok {
		return nil, &MCPErrorObj{
			Code:    -32601,
			Message: fmt.Sprintf("tool not found: %s", params.Name),
		}
	}

	// Prepare arguments
	args := params.Arguments
	if args == nil {
		args = make(map[string]interface{})
	}

	s.logger.Info("Tool called via MCP",
		zap.String("tool", params.Name),
		zap.Int("args", len(args)))

	// Execute handler
	result, err := handler(ctx, args)
	if err != nil {
		// Check if it's an MCPErrorObj
		if mcpErr, ok := err.(*MCPErrorObj); ok {
			return nil, mcpErr
		}
		return nil, &MCPErrorObj{
			Code:    -32603,
			Message: fmt.Sprintf("tool execution failed: %v", err),
		}
	}

	// Format response as MCP content
	response := CallToolResponse{
		Content: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": formatResult(result),
			},
		},
		IsError: false,
	}

	return response, nil
}

// formatResult formats a result for display
func formatResult(result interface{}) string {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", result)
	}
	return string(data)
}

// GetToolNames returns all registered tool names
func (s *Server) GetToolNames() []string {
	names := make([]string, 0, len(s.tools))
	for _, tool := range s.tools {
		names = append(names, tool.Definition.Name)
	}
	return names
}

// GetTool returns a tool by name
func (s *Server) GetTool(name string) *Tool {
	for _, tool := range s.tools {
		if tool.Definition.Name == name {
			return &tool
		}
	}
	return nil
}
