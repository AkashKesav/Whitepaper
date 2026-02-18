package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// Handler is an interface for HTTP handlers
type Handler interface {
	ServeHTTP(*Request) *Response
}

// ServeHTTP implements the Handler interface
func (f HandlerFunc) ServeHTTP(r *Request) *Response {
	return f(r)
}

// HandlerAdapter converts a standard http.HandlerFunc to our HandlerFunc
func HandlerAdapter(h http.HandlerFunc) HandlerFunc {
	return func(req *Request) *Response {
		// Create standard http.Request from our Request
		stdReq, err := stdRequestFromRequest(req)
		if err != nil {
			return InternalServerError("failed to create standard request")
		}

		// Create response recorder
		w := &responseRecorder{headers: make(map[string]string)}

		// Call the standard handler
		h(w, stdReq)

		// Convert to our Response
		return &Response{
			StatusCode: w.status,
			Headers:    w.headers,
			Body:       w.body.Bytes(),
			KeepAlive:  true,
		}
	}
}

// stdRequestFromRequest converts our Request to a standard http.Request
func stdRequestFromRequest(req *Request) (*http.Request, error) {
	r, err := http.NewRequest(req.Method, req.Path, bytes.NewReader(req.Body))
	if err != nil {
		return nil, err
	}

	// Copy headers
	for k, v := range req.Headers {
		r.Header.Set(k, v)
	}

	// Set query
	if req.QueryString != "" {
		r.URL.RawQuery = req.QueryString
	}

	// Set remote address
	r.RemoteAddr = req.RemoteAddr

	return r, nil
}

// responseRecorder records the response
type responseRecorder struct {
	status  int
	headers map[string]string
	body    limitBuffer
}

type limitBuffer struct {
	buf []byte
}

func (l *limitBuffer) Write(p []byte) (int, error) {
	l.buf = append(l.buf, p...)
	return len(p), nil
}

func (l *limitBuffer) Bytes() []byte {
	return l.buf
}

func (w *responseRecorder) Header() http.Header {
	h := make(http.Header)
	for k, v := range w.headers {
		h.Set(k, v)
	}
	return h
}

func (w *responseRecorder) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	return w.body.Write(b)
}

func (w *responseRecorder) WriteHeader(statusCode int) {
	w.status = statusCode
}

// JSONHandler creates a handler that responds with JSON
func JSONHandler(data interface{}) HandlerFunc {
	return func(req *Request) *Response {
		return JSON(data, 200)
	}
}

// StaticHandler creates a handler that serves static content
func StaticHandler(content string, contentType string) HandlerFunc {
	return func(req *Request) *Response {
		return &Response{
			StatusCode: 200,
			Headers: map[string]string{
				"Content-Type":   contentType,
				"Content-Length": strconv.Itoa(len(content)),
			},
			Body:      []byte(content),
			KeepAlive: true,
		}
	}
}

// FileHandler creates a handler that serves a file
type FileHandler struct {
	root    string
	indexes []string
}

// NewFileHandler creates a new file handler
func NewFileHandler(root string) *FileHandler {
	return &FileHandler{
		root:    root,
		indexes: []string{"index.html", "index.htm"},
	}
}

// ServeHTTP implements the Handler interface
func (h *FileHandler) ServeHTTP(req *Request) *Response {
	// Strip the leading slash from the path
	path := strings.TrimPrefix(req.Path, "/")
	if path == "" {
		path = "."
	}

	// Try to serve the file
	// This is a simplified implementation
	// In production, you'd want to use http.FileServer
	// with proper MIME type detection

	return &Response{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "text/html",
		},
		Body:      []byte("File: " + path),
		KeepAlive: true,
	}
}

// ProxyHandler creates a reverse proxy handler
type ProxyHandler struct {
	target    string
	transport http.RoundTripper
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(target string) *ProxyHandler {
	return &ProxyHandler{
		target:    target,
		transport: http.DefaultTransport,
	}
}

// ServeHTTP implements the Handler interface
func (h *ProxyHandler) ServeHTTP(req *Request) *Response {
	// Create the target URL
	targetURL := h.target + req.Path
	if req.QueryString != "" {
		targetURL += "?" + req.QueryString
	}

	// Create proxy request
	proxyReq, err := http.NewRequest(req.Method, targetURL, nil)
	if err != nil {
		return InternalServerError(err.Error())
	}

	// Copy headers
	for k, v := range req.Headers {
		proxyReq.Header.Set(k, v)
	}

	// Set X-Forwarded-For header
	if req.RemoteAddr != "" {
		proxyReq.Header.Set("X-Forwarded-For", req.RemoteAddr)
	}

	// Send the request
	client := &http.Client{Transport: h.transport}
	resp, err := client.Do(proxyReq)
	if err != nil {
		return BadGateway(err.Error())
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return InternalServerError(err.Error())
	}

	// Convert headers
	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
		KeepAlive:  true,
	}
}

// NotFoundHandler creates a 404 handler
func NotFoundHandler(message string) HandlerFunc {
	return func(req *Request) *Response {
		if message == "" {
			message = "404 Not Found"
		}
		return &Response{
			StatusCode: 404,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body:      []byte(message),
			KeepAlive: false,
		}
	}
}

// MethodNotAllowedHandler creates a 405 handler
func MethodNotAllowedHandler(allowed ...string) HandlerFunc {
	return func(req *Request) *Response {
		return &Response{
			StatusCode: 405,
			Headers: map[string]string{
				"Content-Type": "text/plain",
				"Allow":        strings.Join(allowed, ", "),
			},
			Body:      []byte("405 Method Not Allowed"),
			KeepAlive: false,
		}
	}
}

// ErrorHandler creates an error response handler
func ErrorHandler(statusCode int, message string) HandlerFunc {
	return func(req *Request) *Response {
		if message == "" {
			message = http.StatusText(statusCode)
		}
		return &Response{
			StatusCode: statusCode,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body:      []byte(message),
			KeepAlive: false,
		}
	}
}

// BadGateway returns a 502 Bad Gateway response
func BadGateway(message string) *Response {
	return &Response{
		StatusCode: 502,
		Headers: map[string]string{
			"Content-Type": "text/plain",
		},
		Body:      []byte("Bad Gateway: " + message),
		KeepAlive: false,
	}
}

// ServiceUnavailable returns a 503 Service Unavailable response
func ServiceUnavailable(message string) *Response {
	return &Response{
		StatusCode: 503,
		Headers: map[string]string{
			"Content-Type": "text/plain",
		},
		Body:      []byte("Service Unavailable: " + message),
		KeepAlive: false,
	}
}

// GatewayTimeout returns a 504 Gateway Timeout response
func GatewayTimeout(message string) *Response {
	return &Response{
		StatusCode: 504,
		Headers: map[string]string{
			"Content-Type": "text/plain",
		},
		Body:      []byte("Gateway Timeout: " + message),
		KeepAlive: false,
	}
}

// HealthCheckHandler creates a health check handler
func HealthCheckHandler(checks ...func() map[string]string) HandlerFunc {
	return func(req *Request) *Response {
		status := map[string]interface{}{
			"status": "healthy",
		}

		if len(checks) > 0 {
			details := make(map[string]string)
			allHealthy := true
			for _, check := range checks {
				result := check()
				for k, v := range result {
					details[k] = v
					if v != "ok" {
						allHealthy = false
					}
				}
			}
			status["details"] = details
			if !allHealthy {
				status["status"] = "unhealthy"
				return JSON(status, 503)
			}
		}

		return JSON(status, 200)
	}
}

// ReadyHandler creates a readiness check handler
func ReadyHandler(isReady func() bool) HandlerFunc {
	return func(req *Request) *Response {
		if isReady == nil || isReady() {
			return JSON(map[string]string{"status": "ready"}, 200)
		}
		return JSON(map[string]string{"status": "not ready"}, 503)
	}
}

// MetricsHandler creates a metrics handler (Prometheus format)
func MetricsHandler(getMetrics func() string) HandlerFunc {
	return func(req *Request) *Response {
		if getMetrics == nil {
			return Text("# No metrics available\n", 200)
		}
		return Text(getMetrics(), 200)
	}
}

// ParseJSON parses the request body as JSON
func ParseJSON(req *Request, v interface{}) error {
	return json.Unmarshal(req.Body, v)
}

// WriteJSON writes JSON to the response
func WriteJSON(resp *Response, v interface{}) error {
	data, err := JSONMarshal(v)
	if err != nil {
		return err
	}
	resp.Body = data
	resp.SetContentType("application/json")
	resp.SetHeader("Content-Length", strconv.Itoa(len(data)))
	return nil
}

// QueryBool parses a query parameter as boolean
func QueryBool(req *Request, key string, defaultValue bool) bool {
	val := req.QueryParam(key)
	if val == "" {
		return defaultValue
	}
	return strings.ToLower(val) == "true" || val == "1" || val == "yes"
}

// QueryInt parses a query parameter as integer
func QueryInt(req *Request, key string, defaultValue int) int {
	val := req.QueryParam(key)
	if val == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return i
}

// QueryInt64 parses a query parameter as int64
func QueryInt64(req *Request, key string, defaultValue int64) int64 {
	val := req.QueryParam(key)
	if val == "" {
		return defaultValue
	}
	i, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return defaultValue
	}
	return i
}

// QueryFloat parses a query parameter as float
func QueryFloat(req *Request, key string, defaultValue float64) float64 {
	val := req.QueryParam(key)
	if val == "" {
		return defaultValue
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return defaultValue
	}
	return f
}

// QuerySlice parses a query parameter as a string slice
func QuerySlice(req *Request, key string) []string {
	val := req.QueryParam(key)
	if val == "" {
		return nil
	}
	return strings.Split(val, ",")
}

// SetCookie sets a cookie in the response
func SetCookie(resp *Response, name, value string, opts ...CookieOption) {
	cookie := &http.Cookie{
		Name:  name,
		Value: value,
	}

	for _, opt := range opts {
		opt(cookie)
	}

	resp.SetCookie(cookie)
}

// CookieOption is a function that configures a cookie
type CookieOption func(*http.Cookie)

// WithCookieDomain sets the cookie domain
func WithCookieDomain(domain string) CookieOption {
	return func(c *http.Cookie) {
		c.Domain = domain
	}
}

// WithCookiePath sets the cookie path
func WithCookiePath(path string) CookieOption {
	return func(c *http.Cookie) {
		c.Path = path
	}
}

// WithCookieMaxAge sets the cookie max age
func WithCookieMaxAge(maxAge int) CookieOption {
	return func(c *http.Cookie) {
		c.MaxAge = maxAge
	}
}

// WithCookieSecure sets the cookie secure flag
func WithCookieSecure(secure bool) CookieOption {
	return func(c *http.Cookie) {
		c.Secure = secure
	}
}

// WithCookieHTTPOnly sets the cookie httpOnly flag
func WithCookieHTTPOnly(httpOnly bool) CookieOption {
	return func(c *http.Cookie) {
		c.HttpOnly = httpOnly
	}
}

// WithCookieSameSite sets the cookie sameSite mode
func WithCookieSameSite(sameSite http.SameSite) CookieOption {
	return func(c *http.Cookie) {
		c.SameSite = sameSite
	}
}

// Mux is a simple HTTP request multiplexer
type Mux struct {
	routes   map[string]map[string]HandlerFunc // method -> path -> handler
	notFound HandlerFunc
}

// NewMux creates a new request multiplexer
func NewMux() *Mux {
	return &Mux{
		routes:   make(map[string]map[string]HandlerFunc),
		notFound: NotFoundHandler(""),
	}
}

// Handle registers a handler for a method and path
func (m *Mux) Handle(method, path string, handler HandlerFunc) {
	if m.routes[method] == nil {
		m.routes[method] = make(map[string]HandlerFunc)
	}
	m.routes[method][path] = handler
}

// GET registers a GET handler
func (m *Mux) GET(path string, handler HandlerFunc) {
	m.Handle("GET", path, handler)
}

// POST registers a POST handler
func (m *Mux) POST(path string, handler HandlerFunc) {
	m.Handle("POST", path, handler)
}

// PUT registers a PUT handler
func (m *Mux) PUT(path string, handler HandlerFunc) {
	m.Handle("PUT", path, handler)
}

// DELETE registers a DELETE handler
func (m *Mux) DELETE(path string, handler HandlerFunc) {
	m.Handle("DELETE", path, handler)
}

// PATCH registers a PATCH handler
func (m *Mux) PATCH(path string, handler HandlerFunc) {
	m.Handle("PATCH", path, handler)
}

// ServeHTTP implements the Handler interface
func (m *Mux) ServeHTTP(req *Request) *Response {
	if routes, ok := m.routes[req.Method]; ok {
		if handler, ok := routes[req.Path]; ok {
			return handler(req)
		}
	}
	return m.notFound(req)
}

// SetNotFound sets the not found handler
func (m *Mux) SetNotFound(handler HandlerFunc) {
	m.notFound = handler
}

// ServeContent serves content with proper content type
func ServeContent(contentType string, content []byte) HandlerFunc {
	return func(req *Request) *Response {
		return &Response{
			StatusCode: 200,
			Headers: map[string]string{
				"Content-Type":   contentType,
				"Content-Length": fmt.Sprintf("%d", len(content)),
			},
			Body:      content,
			KeepAlive: true,
		}
	}
}

// RedirectHandler creates a redirect handler
func RedirectHandler(url string, statusCode int) HandlerFunc {
	return func(req *Request) *Response {
		return &Response{
			StatusCode: statusCode,
			Headers: map[string]string{
				"Location": url,
			},
			KeepAlive: false,
		}
	}
}
