// Package server provides high-performance HTTP/WebSocket server using gnet
// This package implements an event-driven HTTP server on top of gnet's
// event-loop architecture for superior performance compared to net/http.
package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/panjf2000/gnet/v2"
	"github.com/panjf2000/gnet/v2/pkg/logging"
	"go.uber.org/zap"
)

// Engine is the gnet-based HTTP server engine
type Engine struct {
	gnet.BuiltinEventEngine

	// Configuration
	addr           string
	network        string  // "tcp" or "tcp4" or "tcp6"
	multicore      bool
	readonly       bool
	lingerTimeout  time.Duration

	// Server state
	router         *Router
	middleware     []MiddlewareFunc
	logger         *zap.Logger
	server         gnet.Engine
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	shutdownWG     sync.WaitGroup

	// Statistics
	activeConns    atomic.Int64
	totalReq       atomic.Int64

	// TLS configuration
	tlsConfig      *tls.Config

	// Options
	options        *Options
}

// Options configures the Engine
type Options struct {
	// Network type (tcp, tcp4, tcp6)
	Network string

	// Enable multi-core event loops (default: true)
	Multicore bool

	// Read-only mode (default: false)
	ReadOnly bool

	// Linger timeout for connections (default: 1s)
	LingerTimeout time.Duration

	// TLS configuration
	TLSConfig *tls.Config

	// Logger
	Logger *zap.Logger

	// Maximum number of concurrent connections
	MaxConnections int

	// Read buffer size per connection
	ReadBufferSize int

	// Write buffer size per connection
	WriteBufferSize int

	// Connection timeout
	ConnTimeout time.Duration

	// Enable HTTP/2
	HTTP2 bool

	// Enable WebSocket
	WebSocket bool

	// Static file serving
	StaticDir string

	// Static file route prefix
	StaticPrefix string
}

// DefaultOptions returns default options for the engine
func DefaultOptions() *Options {
	return &Options{
		Network:        "tcp",
		Multicore:      true,
		ReadOnly:       false,
		LingerTimeout:  time.Second,
		ReadBufferSize: 4096,
		WriteBufferSize: 4096,
		ConnTimeout:    120 * time.Second,
		HTTP2:          false, // HTTP/2 not yet supported in custom parser
		WebSocket:      true,
	}
}

// New creates a new gnet-based HTTP engine
func New(addr string, opts *Options) *Engine {
	if opts == nil {
		opts = DefaultOptions()
	}

	// Set up logger
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Engine{
		addr:           addr,
		network:        opts.Network,
		multicore:      opts.Multicore,
		readonly:       opts.ReadOnly,
		lingerTimeout:  opts.LingerTimeout,
		router:         NewRouter(),
		middleware:     make([]MiddlewareFunc, 0),
		logger:         logger,
		shutdownCtx:    ctx,
		shutdownCancel: cancel,
		tlsConfig:      opts.TLSConfig,
		options:        opts,
	}
}

// OnOpen handles connection open events
func (e *Engine) OnOpen(c gnet.Conn) ([]byte, gnet.Action) {
	e.activeConns.Add(1)
	e.logger.Debug("connection opened",
		zap.String("remote", c.RemoteAddr().String()),
		zap.Int64("active", e.activeConns.Load()))
	return nil, gnet.None
}

// OnClose handles connection close events
func (e *Engine) OnClose(c gnet.Conn, err error) gnet.Action {
	e.activeConns.Add(-1)
	e.logger.Debug("connection closed",
		zap.String("remote", c.RemoteAddr().String()),
		zap.Int64("active", e.activeConns.Load()),
		zap.Error(err))
	return gnet.None
}

// OnTraffic handles incoming data on a connection
func (e *Engine) OnTraffic(c gnet.Conn) gnet.Action {
	// Increment request counter (approximate)
	e.totalReq.Add(1)

	// Read all available data
	buf, _ := c.Next(-1)

	// Parse HTTP request
	req, err := ParseRequest(buf, c)
	if err != nil {
		e.logger.Error("failed to parse request",
			zap.String("remote", c.RemoteAddr().String()),
			zap.Error(err))
		return e.writeErrorResponse(c, 400, "Bad Request")
	}

	// Attach connection to request
	req.conn = c

	// Store connection state
	if state, ok := c.Context().(*connState); ok {
		req.State = state
	} else {
		req.State = &connState{
			CreatedAt:   time.Now(),
			LastSeenAt:  time.Now(),
			RemoteAddr:  c.RemoteAddr().String(),
		}
		c.SetContext(req.State)
	}

	// Route the request
	resp := e.routeRequest(req)

	// Write response
	return e.writeResponse(c, resp)
}

// OnTick handles periodic tick events
func (e *Engine) OnTick() (delay time.Duration, action gnet.Action) {
	return time.Second * 10, gnet.None
}

// connState maintains per-connection state
type connState struct {
	CreatedAt   time.Time
	LastSeenAt  time.Time
	RemoteAddr  string
	UserAgent   string
	Headers     map[string]string
}

// routeRequest routes the request through middleware and handlers
func (e *Engine) routeRequest(req *Request) *Response {
	// Build middleware chain
	handler := e.router.Route(req)

	if handler == nil {
		return NotFound(req)
	}

	// Apply middleware chain
	for i := len(e.middleware) - 1; i >= 0; i-- {
		handler = e.middleware[i](handler)
	}

	// Execute handler
	return handler(req)
}

// writeResponse writes an HTTP response to the connection
func (e *Engine) writeResponse(c gnet.Conn, resp *Response) gnet.Action {
	if resp == nil {
		return gnet.Close
	}

	// Build response bytes
	data := resp.Build()

	// Write to connection
	if _, err := c.Write(data); err != nil {
		e.logger.Error("failed to write response",
			zap.String("remote", c.RemoteAddr().String()),
			zap.Error(err))
		return gnet.Close
	}

	// Check if we should keep alive or close
	if !resp.KeepAlive || resp.StatusCode >= 400 {
		return gnet.Close
	}

	return gnet.None
}

// writeErrorResponse writes an error response
func (e *Engine) writeErrorResponse(c gnet.Conn, status int, message string) gnet.Action {
	resp := &Response{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type":   "text/plain",
			"Content-Length": fmt.Sprintf("%d", len(message)),
			"Connection":     "close",
		},
		Body:     []byte(message),
		KeepAlive: false,
	}
	return e.writeResponse(c, resp)
}

// Use adds middleware to the engine
func (e *Engine) Use(mw MiddlewareFunc) {
	e.middleware = append(e.middleware, mw)
}

// Handle adds a route to the engine
func (e *Engine) Handle(method, path string, handler HandlerFunc) {
	e.router.Add(method, path, handler)
}

// GET adds a GET route
func (e *Engine) GET(path string, handler HandlerFunc) {
	e.Handle("GET", path, handler)
}

// POST adds a POST route
func (e *Engine) POST(path string, handler HandlerFunc) {
	e.Handle("POST", path, handler)
}

// PUT adds a PUT route
func (e *Engine) PUT(path string, handler HandlerFunc) {
	e.Handle("PUT", path, handler)
}

// DELETE adds a DELETE route
func (e *Engine) DELETE(path string, handler HandlerFunc) {
	e.Handle("DELETE", path, handler)
}

// PATCH adds a PATCH route
func (e *Engine) PATCH(path string, handler HandlerFunc) {
	e.Handle("PATCH", path, handler)
}

// OPTIONS adds an OPTIONS route
func (e *Engine) OPTIONS(path string, handler HandlerFunc) {
	e.Handle("OPTIONS", path, handler)
}

// HEAD adds a HEAD route
func (e *Engine) HEAD(path string, handler HandlerFunc) {
	e.Handle("HEAD", path, handler)
}

// HandleFunc adds a route that matches any method
func (e *Engine) HandleFunc(path string, handler HandlerFunc) {
	e.router.AddAny(path, handler)
}

// Start starts the gnet server
func (e *Engine) Start() error {
	// Configure gnet options
	gnetOpts := []gnet.Option{
		gnet.WithMulticore(e.multicore),
		gnet.WithLogLevel(logging.ErrorLevel), // Reduce noise
		gnet.WithLogger(newGnetLoggerAdapter(e.logger)),
	}

	// Start the server
	var err error
	if e.tlsConfig != nil {
		err = gnet.Run(e, fmt.Sprintf("%s://%s", "tls", e.addr), gnetOpts...)
	} else {
		err = gnet.Run(e, fmt.Sprintf("%s://%s", e.network, e.addr), gnetOpts...)
	}

	if err != nil {
		return fmt.Errorf("failed to start gnet server: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the server
func (e *Engine) Shutdown(ctx context.Context) error {
	e.logger.Info("shutting down server...")
	e.shutdownCancel()

	// Wait for connections to drain or timeout
	done := make(chan struct{})
	go func() {
		e.shutdownWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		e.logger.Info("all connections closed gracefully")
		return nil
	case <-ctx.Done():
		e.logger.Warn("shutdown timeout exceeded, forcing close")
		return ctx.Err()
	}
}

// Stats returns server statistics
func (e *Engine) Stats() ServerStats {
	return ServerStats{
		ActiveConnections: e.activeConns.Load(),
		TotalRequests:     e.totalReq.Load(),
		Addr:             e.addr,
	}
}

// ServerStats holds server statistics
type ServerStats struct {
	ActiveConnections int64
	TotalRequests     int64
	Addr             string
}

// gnetLoggerAdapter adapts zap logger to gnet's logger interface
type gnetLoggerAdapter struct {
	logger *zap.Logger
}

func newGnetLoggerAdapter(logger *zap.Logger) logging.Logger {
	return &gnetLoggerAdapter{logger: logger}
}

func (a *gnetLoggerAdapter) Errorf(format string, args ...interface{}) {
	a.logger.Error(fmt.Sprintf(format, args...))
}

func (a *gnetLoggerAdapter) Warnf(format string, args ...interface{}) {
	a.logger.Warn(fmt.Sprintf(format, args...))
}

func (a *gnetLoggerAdapter) Infof(format string, args ...interface{}) {
	a.logger.Info(fmt.Sprintf(format, args...))
}

func (a *gnetLoggerAdapter) Debugf(format string, args ...interface{}) {
	a.logger.Debug(fmt.Sprintf(format, args...))
}

func (a *gnetLoggerAdapter) Fatalf(format string, args ...interface{}) {
	a.logger.Fatal(fmt.Sprintf(format, args...))
}

// splitHostPort splits host and port, handling IPv6 addresses
func splitHostPort(addr string) (string, int) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// Try parsing as just a port
		if strings.HasPrefix(addr, ":") {
			return "", 0
		}
		// Assume host only, use default port
		return addr, 0
	}

	// Parse port
	p := 0
	if port != "" {
		fmt.Sscanf(port, "%d", &p)
	}

	return host, p
}

// Addr returns the server address
func (e *Engine) Addr() string {
	return e.addr
}

// Router returns the engine's router
func (e *Engine) Router() *Router {
	return e.router
}

// Serve is a convenience method to start the server and block
func (e *Engine) Serve() error {
	return e.Start()
}

// ListenAndServe starts the HTTP server
func ListenAndServe(addr string, handler HandlerFunc) error {
	engine := New(addr, nil)
	engine.HandleFunc("/", handler)
	return engine.Start()
}

// ListenAndServeTLS starts the HTTPS server
func ListenAndServeTLS(addr, certFile, keyFile string, handler HandlerFunc) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("failed to load TLS certificate: %w", err)
	}

	opts := DefaultOptions()
	opts.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	engine := New(addr, opts)
	engine.HandleFunc("/", handler)
	return engine.Start()
}
