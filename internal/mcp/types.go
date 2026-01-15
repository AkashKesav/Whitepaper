// Package mcp implements the Model Context Protocol (MCP) server for RMK.
package mcp

import (
	"context"
)

// MCPRequest represents a JSON-RPC 2.0 request from MCP client
type MCPRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id,omitempty"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// MCPResponse represents a JSON-RPC 2.0 response to MCP client
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

// InitializeParams are sent during MCP initialization
type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities     map[string]interface{} `json:"capabilities"`
	ClientInfo       map[string]string       `json:"clientInfo,omitempty"`
}

// InitializeResponse is sent back to the client
type InitializeResponse struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities     map[string]interface{} `json:"capabilities"`
	ServerInfo       map[string]string       `json:"serverInfo"`
}

// ListToolsResponse returns available tools
type ListToolsResponse struct {
	Tools []ToolDefinition `json:"tools"`
}

// ToolDefinition defines an MCP tool
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// CallToolParams are parameters for tool execution
type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// CallToolResponse is the result of tool execution
type CallToolResponse struct {
	Content     []interface{} `json:"content"`
	IsError     bool           `json:"isError,omitempty"`
	Metadata    map[string]interface{} `json:"_meta,omitempty"`
}

// ToolContent represents structured content in tool response
type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ToolHandler handles tool execution
type ToolHandler func(ctx context.Context, args map[string]interface{}) (interface{}, error)

// Tool wraps a ToolDefinition with its handler
type Tool struct {
	Definition ToolDefinition
	Handler    ToolHandler
}
