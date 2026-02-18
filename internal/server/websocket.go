// Package server provides high-performance HTTP/WebSocket server using gnet
// This file provides WebSocket support using gorilla/websocket protocol
package server

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/panjf2000/gnet/v2"
	"github.com/reflective-memory-kernel/internal/jsonx"
	"go.uber.org/zap"
)

const (
	// WebSocket protocol versions
	WebSocketVersion = 13

	// WebSocket frame opcodes
	OpcodeContinuation = 0x0
	OpcodeText         = 0x1
	OpcodeBinary       = 0x2
	OpcodeClose        = 0x8
	OpcodePing         = 0x9
	OpcodePong         = 0xA

	// WebSocket frame masks
	FinBit     = 0x80
	MaskBit    = 0x80
	LenMask16  = 0x7E
	LenMask64  = 0x7F
	MaxLen16   = 0xFFFF
	MaxLen64   = 0x7FFFFFFFFFFFFFFF

	// Close codes
	CloseNormalClosure           = 1000
	CloseGoingAway               = 1001
	CloseProtocolError           = 1002
	CloseUnsupportedData         = 1003
	CloseNoStatusReceived        = 1005
	CloseAbnormalClosure         = 1006
	CloseInvalidFramePayloadData = 1007
	ClosePolicyViolation         = 1008
	CloseMessageTooBig           = 1009
	CloseMandatoryExtension      = 1010
	CloseInternalServerErr       = 1011
	CloseServiceRestart          = 1012
	CloseTryAgainLater           = 1013
	CloseTLSHandshake            = 1015

	// Default read buffer size
	DefaultReadBufferSize  = 4096
	DefaultWriteBufferSize = 4096

	// WebSocket magic GUID
	websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
)

// Conn represents a WebSocket connection
type Conn struct {
	conn        gnet.Conn
	engine      *Engine
	connID      string
	mu          sync.RWMutex
	writeMu     sync.Mutex
	readBuffer  []byte
	writeBuffer []byte

	// Connection state
	handshakeComplete bool
	isServer          bool

	// Message handler
	messageHandler func(*Conn, []byte)
	closeHandler  func(*Conn, int, string)

	// Configuration
	readBufferSize  int
	writeBufferSize int
	compression     bool

	// Ping/pong
	pingHandler   func(*Conn, string)
	pongHandler   func(*Conn, string)
	pingInterval  time.Duration
	pingTimeout   time.Duration
	lastPongTime  time.Time

	// Logger
	logger *zap.Logger

	// User data
	data map[string]interface{}
}

// Upgrader converts HTTP connections to WebSocket connections
type Upgrader struct {
	// Handshake timeout
	HandshakeTimeout time.Duration

	// Read and write buffer sizes
	ReadBufferSize  int
	WriteBufferSize int

	// Subprotocols
	Subprotocols []string

	// Compression
	Compression bool

	// Check origin
	CheckOrigin func(r *Request) bool

	// Error handler
	Error func(w *Response, r *Request, status int, reason error)

	// Logger
	Logger *zap.Logger
}

// NewUpgrader creates a new WebSocket upgrader
func NewUpgrader() *Upgrader {
	return &Upgrader{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   DefaultReadBufferSize,
		WriteBufferSize:  DefaultWriteBufferSize,
		CheckOrigin: func(r *Request) bool {
			// Allow all origins by default for development
			return true
		},
	}
}

// Upgrade upgrades an HTTP request to a WebSocket connection
func (u *Upgrader) Upgrade(engine *Engine, req *Request, handler func(*Conn, []byte)) (*Conn, *Response) {
	// Check WebSocket version
	if req.Header("Sec-WebSocket-Version") != strconv.Itoa(WebSocketVersion) {
		return nil, &Response{
			StatusCode: 400,
			Headers: map[string]string{
				"Sec-WebSocket-Version": strconv.Itoa(WebSocketVersion),
			},
		}
	}

	// Check origin
	if !u.CheckOrigin(req) {
		return nil, Forbidden("WebSocket origin denied")
	}

	// Get WebSocket key
	key := req.Header("Sec-WebSocket-Key")
	if key == "" {
		return nil, BadRequest("Missing Sec-WebSocket-Key")
	}

	// Select subprotocol if requested
	subprotocol := u.selectSubprotocol(req.Header("Sec-WebSocket-Protocol"))

	// Create handshake response
	acceptKey := u.computeAcceptKey(key)

	headers := map[string]string{
		"Upgrade":              "websocket",
		"Connection":           "Upgrade",
		"Sec-WebSocket-Accept": acceptKey,
	}

	if subprotocol != "" {
		headers["Sec-WebSocket-Protocol"] = subprotocol
	}

	// Create WebSocket connection
	wsConn := &Conn{
		conn:             req.conn,
		engine:           engine,
		connID:           req.conn.RemoteAddr().String() + "-" + time.Now().Format("20060102150405"),
		readBuffer:       make([]byte, u.ReadBufferSize),
		writeBuffer:      make([]byte, u.WriteBufferSize),
		isServer:         true,
		readBufferSize:   u.ReadBufferSize,
		writeBufferSize:  u.WriteBufferSize,
		compression:      u.Compression,
		messageHandler:   handler,
		handshakeComplete: true,
		logger:           u.Logger,
		data:             make(map[string]interface{}),
	}

	return wsConn, &Response{
		StatusCode: 101,
		Headers:    headers,
		KeepAlive:  false,
	}
}

// selectSubprotocol selects the best matching subprotocol
func (u *Upgrader) selectSubprotocol(header string) string {
	if len(u.Subprotocols) == 0 {
		return ""
	}

	requested := parseSubprotocols(header)
	for _, offered := range u.Subprotocols {
		for _, requested := range requested {
			if offered == requested {
				return offered
			}
		}
	}
	return ""
}

// parseSubprotocols parses the Sec-WebSocket-Protocol header
func parseSubprotocols(header string) []string {
	var protocols []string
	parts := splitHeader(header)
	for _, part := range parts {
		protocols = append(protocols, trimQuotes(part))
	}
	return protocols
}

// splitHeader splits a header by commas
func splitHeader(header string) []string {
	var parts []string
	current := ""
	inQuotes := false

	for _, c := range header {
		switch c {
		case ',':
			if !inQuotes {
				parts = append(parts, current)
				current = ""
				continue
			}
		case '"':
			inQuotes = !inQuotes
		}
		current += string(c)
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

// trimQuotes removes quotes from a string
func trimQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// computeAcceptKey computes the WebSocket accept key
func (u *Upgrader) computeAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key))
	h.Write([]byte(websocketGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// WriteMessage writes a message to the WebSocket connection
func (c *Conn) WriteMessage(opcode byte, data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	frame := c.buildFrame(opcode, data, true)
	_, err := c.conn.Write(frame)
	return err
}

// WriteText writes a text message
func (c *Conn) WriteText(text string) error {
	return c.WriteMessage(OpcodeText, []byte(text))
}

// WriteJSON writes a JSON message
func (c *Conn) WriteJSON(v interface{}) error {
	data, err := jsonx.Marshal(v)
	if err != nil {
		return err
	}
	return c.WriteMessage(OpcodeText, data)
}

// WriteBinary writes a binary message
func (c *Conn) WriteBinary(data []byte) error {
	return c.WriteMessage(OpcodeBinary, data)
}

// buildFrame builds a WebSocket frame
func (c *Conn) buildFrame(opcode byte, data []byte, fin bool) []byte {
	frame := make([]byte, 0, 10+len(data))

	// First byte: FIN + opcode
	firstByte := byte(0)
	if fin {
		firstByte |= FinBit
	}
	firstByte |= opcode
	frame = append(frame, firstByte)

	// Second byte: MASK + length
	length := len(data)
	var secondByte byte

	if length < 126 {
		secondByte = byte(length) | MaskBit
		frame = append(frame, secondByte)
	} else if length < 65536 {
		secondByte = 126 | MaskBit
		frame = append(frame, secondByte)
		lengthBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(lengthBytes, uint16(length))
		frame = append(frame, lengthBytes...)
	} else {
		secondByte = 127 | MaskBit
		frame = append(frame, secondByte)
		lengthBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(lengthBytes, uint64(length))
		frame = append(frame, lengthBytes...)
	}

	// Masking key (server->client is not masked, but we'll include it for compatibility)
	mask := make([]byte, 4)
	frame = append(frame, mask...)

	// Payload (masked)
	masked := make([]byte, length)
	for i := 0; i < length; i++ {
		masked[i] = data[i] ^ mask[i%4]
	}
	frame = append(frame, masked...)

	return frame
}

// parseFrame parses a WebSocket frame
func (c *Conn) parseFrame(data []byte) (opcode byte, payload []byte, ok bool) {
	if len(data) < 2 {
		return 0, nil, false
	}

	// First byte: FIN + opcode
	firstByte := data[0]
	fin := (firstByte & FinBit) != 0
	opcode = firstByte & 0x0F

	// Second byte: MASK + length
	secondByte := data[1]
	hasMask := (secondByte & MaskBit) != 0
	length := int(secondByte & 0x7F)

	offset := 2

	// Read extended length
	if length == 126 {
		if len(data) < offset+2 {
			return 0, nil, false
		}
		length = int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2
	} else if length == 127 {
		if len(data) < offset+8 {
			return 0, nil, false
		}
		length = int(binary.BigEndian.Uint64(data[offset : offset+8]))
		offset += 8
	}

	// Check if we have enough data
	if len(data) < offset+length {
		return 0, nil, false
	}

	payload = data[offset : offset+length]

	// Unmask if needed
	if hasMask {
		mask := data[offset-4 : offset]
		for i := 0; i < len(payload); i++ {
			payload[i] ^= mask[i%4]
		}
	}

	return opcode, payload, fin
}

// ReadMessage reads a message from the WebSocket connection
func (c *Conn) ReadMessage() (opcode byte, payload []byte, err error) {
	// This is a simplified implementation
	// In production, you'd want to buffer partial frames
	return 0, nil, nil
}

// Close closes the WebSocket connection
func (c *Conn) Close(code int, reason string) error {
	message := formatCloseMessage(code, reason)
	return c.WriteMessage(OpcodeClose, message)
}

// formatCloseMessage formats a close message
func formatCloseMessage(code int, reason string) []byte {
	buf := make([]byte, 2+len(reason))
	binary.BigEndian.PutUint16(buf[:2], uint16(code))
	copy(buf[2:], reason)
	return buf
}

// SetPingHandler sets the ping handler
func (c *Conn) SetPingHandler(h func(*Conn, string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pingHandler = h
}

// SetPongHandler sets the pong handler
func (c *Conn) SetPongHandler(h func(*Conn, string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pongHandler = h
}

// SetCloseHandler sets the close handler
func (c *Conn) SetCloseHandler(h func(*Conn, int, string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeHandler = h
}

// Ping sends a ping
func (c *Conn) Ping() error {
	return c.WriteMessage(OpcodePing, []byte{})
}

// PingWithPayload sends a ping with payload
func (c *Conn) PingWithPayload(payload []byte) error {
	return c.WriteMessage(OpcodePing, payload)
}

// Pong sends a pong
func (c *Conn) Pong() error {
	return c.WriteMessage(OpcodePong, []byte{})
}

// PongWithPayload sends a pong with payload
func (c *Conn) PongWithPayload(payload []byte) error {
	return c.WriteMessage(OpcodePong, payload)
}

// RemoteAddr returns the remote address
func (c *Conn) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}

// ID returns the connection ID
func (c *Conn) ID() string {
	return c.connID
}

// Set sets a value in the connection data
func (c *Conn) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.data == nil {
		c.data = make(map[string]interface{})
	}
	c.data[key] = value
}

// Get gets a value from the connection data
func (c *Conn) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[key]
	return v, ok
}

// IsClosed returns true if the connection is closed
func (c *Conn) IsClosed() bool {
	return c.conn == nil
}

// LocalAddr returns the local address
func (c *Conn) LocalAddr() string {
	if c.conn == nil {
		return ""
	}
	return c.conn.LocalAddr().String()
}

// Context returns the connection context
func (c *Conn) Context() interface{} {
	if c.conn == nil {
		return nil
	}
	return c.conn.Context()
}

// WebSocketHandler creates a handler for WebSocket connections
func WebSocketHandler(engine *Engine, upgrader *Upgrader, handler func(*Conn, []byte)) HandlerFunc {
	return func(req *Request) *Response {
		// Check if this is a WebSocket upgrade request
		if !req.IsWebSocket() {
			return BadRequest("Not a WebSocket request")
		}

		// Upgrade the connection
		wsConn, resp := upgrader.Upgrade(engine, req, handler)
		if resp != nil && resp.StatusCode != 101 {
			return resp
		}

		// Store the WebSocket connection in the engine
		// In production, you'd want to track this connection for cleanup

		_ = wsConn // Connection is now established

		// Return the switch protocols response
		return resp
	}
}

// WebSocketMessageHandler creates a simple message-based WebSocket handler
func WebSocketMessageHandler(engine *Engine, upgrader *Upgrader, onMessage func([]byte) []byte) HandlerFunc {
	return func(req *Request) *Response {
		// Check if this is a WebSocket upgrade request
		if !req.IsWebSocket() {
			return BadRequest("Not a WebSocket request")
		}

		// Upgrade the connection
		wsConn, resp := upgrader.Upgrade(engine, req, func(conn *Conn, data []byte) {
			// Handle incoming message
			response := onMessage(data)
			if response != nil {
				conn.WriteMessage(OpcodeText, response)
			}
		})

		if resp != nil && resp.StatusCode != 101 {
			return resp
		}

		// Start reading loop
		go func() {
			// In production, you'd want to read from the connection
			// and call the message handler for each message
			_ = wsConn
		}()

		return resp
	}
}

// JSONWebSocketHandler creates a JSON-based WebSocket handler
func JSONWebSocketHandler(engine *Engine, upgrader *Upgrader, onMessage func(map[string]interface{}) map[string]interface{}) HandlerFunc {
	return func(req *Request) *Response {
		// Check if this is a WebSocket upgrade request
		if !req.IsWebSocket() {
			return BadRequest("Not a WebSocket request")
		}

		// Upgrade the connection
		wsConn, resp := upgrader.Upgrade(engine, req, func(conn *Conn, data []byte) {
			// Parse JSON
			var msg map[string]interface{}
			if err := jsonx.Unmarshal(data, &msg); err != nil {
				conn.WriteJSON(map[string]interface{}{
					"error": "invalid JSON",
				})
				return
			}

			// Handle message
			response := onMessage(msg)
			if response != nil {
				conn.WriteJSON(response)
			}
		})

		if resp != nil && resp.StatusCode != 101 {
			return resp
		}

		_ = wsConn
		return resp
	}
}

// ParseCloseMessage parses a close message
func ParseCloseMessage(data []byte) (code int, reason string, err error) {
	if len(data) < 2 {
		return 0, "", io.ErrUnexpectedEOF
	}

	code = int(binary.BigEndian.Uint16(data[:2]))
	reason = string(data[2:])

	// Validate code
	if !isValidCloseCode(code) {
		return code, "", fmt.Errorf("invalid close code: %d", code)
	}

	return code, reason, nil
}

// isValidCloseCode checks if a close code is valid
func isValidCloseCode(code int) bool {
	switch code {
	case CloseNormalClosure, CloseGoingAway, CloseProtocolError, CloseUnsupportedData,
		CloseInvalidFramePayloadData, ClosePolicyViolation, CloseMessageTooBig,
		CloseMandatoryExtension, CloseInternalServerErr, CloseServiceRestart,
		CloseTryAgainLater:
		return true
	default:
		return code >= 3000 && code <= 4999
	}
}

// IsCloseFrame checks if a frame is a close frame
func IsCloseFrame(opcode byte) bool {
	return opcode == OpcodeClose
}

// IsDataFrame checks if a frame is a data frame
func IsDataFrame(opcode byte) bool {
	return opcode == OpcodeText || opcode == OpcodeBinary
}

// IsControlFrame checks if a frame is a control frame
func IsControlFrame(opcode byte) bool {
	return opcode >= OpcodeClose
}

// CloseStatusBytes builds the bytes for a close status
func CloseStatusBytes(status int) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(status))
	return buf
}

// NewCloseFrameBuilder builds a close frame
func NewCloseFrameBuilder(status int, reason string) []byte {
	buf := make([]byte, 2+len(reason))
	binary.BigEndian.PutUint16(buf[:2], uint16(status))
	copy(buf[2:], reason)
	return buf
}

// Hub maintains a set of active WebSocket connections
type Hub struct {
	connections map[string]*Conn
	broadcast   chan []byte
	register    chan *Conn
	unregister  chan *Conn
	mu          sync.RWMutex
	logger      *zap.Logger
}

// NewHub creates a new WebSocket hub
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		connections: make(map[string]*Conn),
		broadcast:   make(chan []byte, 256),
		register:    make(chan *Conn),
		unregister:  make(chan *Conn),
		logger:      logger,
	}
}

// Run starts the hub
func (h *Hub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			h.connections[conn.ID()] = conn
			h.mu.Unlock()
			h.logger.Debug("websocket registered", zap.String("id", conn.ID()))

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.connections[conn.ID()]; ok {
				delete(h.connections, conn.ID())
				conn.Close(CloseNormalClosure, "")
			}
			h.mu.Unlock()
			h.logger.Debug("websocket unregistered", zap.String("id", conn.ID()))

		case message := <-h.broadcast:
			h.mu.RLock()
			for _, conn := range h.connections {
				if err := conn.WriteMessage(OpcodeText, message); err != nil {
					h.logger.Error("websocket write error", zap.String("id", conn.ID()), zap.Error(err))
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Register registers a connection
func (h *Hub) Register(conn *Conn) {
	h.register <- conn
}

// Unregister unregisters a connection
func (h *Hub) Unregister(conn *Conn) {
	h.unregister <- conn
}

// Broadcast broadcasts a message to all connections
func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

// BroadcastText broadcasts a text message
func (h *Hub) BroadcastText(message string) {
	h.broadcast <- []byte(message)
}

// BroadcastJSON broadcasts a JSON message
func (h *Hub) BroadcastJSON(v interface{}) error {
	data, err := jsonx.Marshal(v)
	if err != nil {
		return err
	}
	h.broadcast <- data
	return nil
}

// Count returns the number of active connections
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// Send sends a message to a specific connection
func (h *Hub) Send(connID string, message []byte) error {
	h.mu.RLock()
	conn, ok := h.connections[connID]
	h.mu.RUnlock()

	if !ok {
		return fmt.Errorf("connection not found: %s", connID)
	}

	return conn.WriteMessage(OpcodeText, message)
}

// BufferPool is a pool of buffers for WebSocket frames
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool creates a new buffer pool
func NewBufferPool(size int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, size)
			},
		},
	}
}

// Get gets a buffer from the pool
func (p *BufferPool) Get() []byte {
	return p.pool.Get().([]byte)
}

// Put returns a buffer to the pool
func (p *BufferPool) Put(buf []byte) {
	p.pool.Put(buf[:cap(buf)])
}

// ReadAll reads all data from a reader
func ReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

// IsWebSocketUpgrade checks if a request is a WebSocket upgrade
func IsWebSocketUpgrade(req *Request) bool {
	return req.IsWebSocket()
}

// IsSameOrigin checks if two requests are from the same origin
func IsSameOrigin(r1, r2 *Request) bool {
	return r1.Header("Origin") == r2.Header("Origin")
}

// TokenAuthenticates returns true if the request has a valid token
// This is a placeholder for proper token authentication
func TokenAuthenticates(req *Request) bool {
	auth := req.Header("Authorization")
	return auth != ""
}
