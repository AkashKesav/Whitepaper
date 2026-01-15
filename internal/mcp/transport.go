// Package mcp implements transport layers for MCP communication
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"

	"go.uber.org/zap"
)

// Transport handles MCP communication
type Transport interface {
	Serve(ctx context.Context, handler RequestHandler) error
}

// RequestHandler handles incoming MCP requests
type RequestHandler interface {
	HandleRequest(ctx context.Context, req MCPRequest) (MCPResponse, error)
}

// StdioTransport implements stdio-based transport for Claude Desktop
type StdioTransport struct {
	reader *bufio.Reader
	writer io.Writer
	logger *zap.Logger
	mu     sync.Mutex
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport(logger *zap.Logger) *StdioTransport {
	return &StdioTransport{
		reader: bufio.NewReader(os.Stdin),
		writer: os.Stdout,
		logger: logger,
	}
}

// Serve starts the stdio transport
func (t *StdioTransport) Serve(ctx context.Context, handler RequestHandler) error {
	decoder := json.NewDecoder(t.reader)
	encoder := json.NewEncoder(t.writer)

	t.logger.Info("MCP stdio transport starting")

	for {
		select {
		case <-ctx.Done():
			t.logger.Info("MCP stdio transport shutting down")
			return ctx.Err()
		default:
		}

		var req MCPRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				t.logger.Info("EOF received, shutting down")
				return nil
			}
			t.logger.Debug("Failed to decode request, skipping", zap.Error(err))
			continue
		}

		t.logger.Debug("Received MCP request", zap.String("method", req.Method), zap.Any("id", req.ID))

		resp, err := handler.HandleRequest(ctx, req)
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

		t.mu.Lock()
		if err := encoder.Encode(resp); err != nil {
			t.logger.Error("Failed to encode response", zap.Error(err))
			t.mu.Unlock()
			return err
		}
		t.mu.Unlock()

		t.logger.Debug("Sent MCP response", zap.Any("id", resp.ID))
	}
}

// HTTPTransport implements HTTP-based transport for web clients
type HTTPTransport struct {
	addr   string
	server *http.Server
	logger *zap.Logger
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(addr string, logger *zap.Logger) *HTTPTransport {
	return &HTTPTransport{
		addr:   addr,
		logger: logger,
	}
}

// Serve starts the HTTP transport
func (t *HTTPTransport) Serve(ctx context.Context, handler RequestHandler) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req MCPRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		resp, err := handler.HandleRequest(r.Context(), req)
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
	})

	t.server = &http.Server{
		Addr:    t.addr,
		Handler: mux,
	}

	t.logger.Info("MCP HTTP transport starting", zap.String("addr", t.addr))

	// Start server in background
	go func() {
		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.logger.Error("HTTP transport error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	t.logger.Info("MCP HTTP transport shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5)
	defer cancel()
	return t.server.Shutdown(shutdownCtx)
}
