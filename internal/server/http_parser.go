package server

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/panjf2000/gnet/v2"
	"github.com/valyala/bytebufferpool"
)

// Request represents an HTTP request
type Request struct {
	// Basic request info
	Method      string
	Path        string
	RawPath     string
	QueryString string
	Proto       string

	// Headers
	Headers map[string]string

	// Body
	Body []byte

	// Parsed query values
	Query url.Values

	// URL parameters (from path like /user/{id})
	PathParams map[string]string

	// Client info
	RemoteAddr string
	Host       string

	// Connection
	conn gnet.Conn

	// Connection state
	State *connState

	// Cookies
	Cookies map[string]*http.Cookie

	// Basic auth
	basicAuth *BasicAuth

	// Request time
	Timestamp time.Time
}

// BasicAuth represents HTTP basic authentication credentials
type BasicAuth struct {
	Username string
	Password string
}

// Response represents an HTTP response
type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
	KeepAlive  bool

	// Internal state
	headersWritten bool
}

// ParseRequest parses an HTTP request from raw bytes
func ParseRequest(data []byte, conn gnet.Conn) (*Request, error) {
	req := &Request{
		Headers:    make(map[string]string),
		Query:      make(url.Values),
		PathParams: make(map[string]string),
		Cookies:    make(map[string]*http.Cookie),
		Timestamp:  time.Now(),
		conn:       conn,
	}

	// Split request line and headers
	idx := bytes.Index(data, []byte("\r\n\r\n"))
	if idx == -1 {
		idx = bytes.Index(data, []byte("\n\n"))
		if idx == -1 {
			return nil, fmt.Errorf("incomplete request")
		}
		idx += 2
	} else {
		idx += 4
	}

	headerSection := data[:idx]
	if idx < len(data) {
		req.Body = data[idx:]
	}

	// Parse request line
	lines := bytes.SplitN(headerSection, []byte("\r\n"), 2)
	if len(lines) < 2 {
		lines = bytes.SplitN(headerSection, []byte("\n"), 2)
		if len(lines) < 2 {
			return nil, fmt.Errorf("invalid request line")
		}
	}

	requestLine := string(lines[0])
	parts := strings.Split(requestLine, " ")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid request line: %s", requestLine)
	}

	req.Method = parts[0]
	fullPath := parts[1]
	if len(parts) > 2 {
		req.Proto = parts[2]
	} else {
		req.Proto = "HTTP/1.1"
	}

	// Parse path and query
	if idx := strings.Index(fullPath, "?"); idx != -1 {
		req.Path = fullPath[:idx]
		req.QueryString = fullPath[idx+1:]
		req.RawPath = req.Path
	} else {
		req.Path = fullPath
		req.RawPath = fullPath
	}

	// Parse query string
	if req.QueryString != "" {
		query, err := url.ParseQuery(req.QueryString)
		if err == nil {
			req.Query = query
		}
	}

	// Parse headers
	headerLines := bytes.Split(lines[1], []byte("\r\n"))
	for _, line := range headerLines {
		line = bytes.TrimLeft(line, " \t")
		if len(line) == 0 {
			continue
		}

		if idx := bytes.IndexByte(line, ':'); idx != -1 {
			key := string(line[:idx])
			value := string(bytes.TrimLeft(line[idx+1:], " \t"))
			req.Headers[strings.ToLower(key)] = value

			// Parse host header
			if strings.ToLower(key) == "host" {
				req.Host = value
			}

			// Parse cookies
			if strings.ToLower(key) == "cookie" {
				req.parseCookies(value)
			}

			// Parse basic auth
			if strings.ToLower(key) == "authorization" {
				req.parseAuthorization(value)
			}
		}
	}

	// Set remote address
	if conn != nil {
		req.RemoteAddr = conn.RemoteAddr().String()
	}

	return req, nil
}

// parseCookies parses the Cookie header
func (r *Request) parseCookies(cookieHeader string) {
	parts := strings.Split(cookieHeader, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if idx := strings.Index(part, "="); idx != -1 {
			name := part[:idx]
			value := part[idx+1:]
			r.Cookies[name] = &http.Cookie{
				Name:  name,
				Value: value,
			}
		}
	}
}

// parseAuthorization parses the Authorization header
func (r *Request) parseAuthorization(authHeader string) {
	if strings.HasPrefix(authHeader, "Basic ") {
		encoded := authHeader[6:]
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err == nil {
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) == 2 {
				r.basicAuth = &BasicAuth{
					Username: parts[0],
					Password: parts[1],
				}
			}
		}
	}
}

// BasicAuth returns the basic auth credentials
func (r *Request) BasicAuth() (username, password string, ok bool) {
	if r.basicAuth != nil {
		return r.basicAuth.Username, r.basicAuth.Password, true
	}
	return "", "", false
}

// Header returns a header value
func (r *Request) Header(key string) string {
	return r.Headers[strings.ToLower(key)]
}

// SetHeader sets a header value
func (r *Request) SetHeader(key, value string) {
	r.Headers[strings.ToLower(key)] = value
}

// Cookie returns a cookie value
func (r *Request) Cookie(name string) (*http.Cookie, bool) {
	c, ok := r.Cookies[name]
	return c, ok
}

// Param returns a path parameter value
func (r *Request) Param(key string) string {
	if r.PathParams == nil {
		return ""
	}
	return r.PathParams[key]
}

// Query returns a query parameter value
func (r *Request) QueryParam(key string) string {
	return r.Query.Get(key)
}

// RemoteIP returns the remote IP address
func (r *Request) RemoteIP() string {
	if r.RemoteAddr == "" {
		return ""
	}

	// Remove port
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
		return r.RemoteAddr[:idx]
	}

	return r.RemoteAddr
}

// UserAgent returns the User-Agent header
func (r *Request) UserAgent() string {
	return r.Header("user-agent")
}

// ContentType returns the Content-Type header
func (r *Request) ContentType() string {
	return r.Header("content-type")
}

// ContentLength returns the Content-Length header value
func (r *Request) ContentLength() int64 {
	lengthStr := r.Header("content-length")
	if lengthStr == "" {
		return int64(len(r.Body))
	}
	length, _ := strconv.ParseInt(lengthStr, 10, 64)
	return length
}

// IsWebSocket checks if the request is a WebSocket upgrade
func (r *Request) IsWebSocket() bool {
	return strings.ToLower(r.Header("upgrade")) == "websocket"
}

// IsSecure checks if the request is over HTTPS
func (r *Request) IsSecure() bool {
	// Check for X-Forwarded-Proto header
	if proto := r.Header("x-forwarded-proto"); proto == "https" {
		return true
	}
	return false
}

// WithContext sets the connection state
func (r *Request) WithContext(state *connState) {
	r.State = state
}

// Build builds an HTTP response
func (resp *Response) Build() []byte {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	// Write status line
	buf.WriteString(resp.ProtoString())
	buf.WriteByte(' ')
	buf.WriteString(strconv.Itoa(resp.StatusCode))
	buf.WriteByte(' ')
	buf.WriteString(http.StatusText(resp.StatusCode))
	buf.WriteString("\r\n")

	// Write headers
	if resp.Headers != nil {
		for key, value := range resp.Headers {
			buf.WriteString(key)
			buf.WriteString(": ")
			buf.WriteString(value)
			buf.WriteString("\r\n")
		}
	}

	// Set default headers
	if resp.Headers != nil {
		if _, ok := resp.Headers["content-length"]; !ok && len(resp.Body) > 0 {
			buf.WriteString("Content-Length: ")
			buf.WriteString(strconv.Itoa(len(resp.Body)))
			buf.WriteString("\r\n")
		}
		if _, ok := resp.Headers["connection"]; !ok {
			if resp.KeepAlive {
				buf.WriteString("Connection: keep-alive\r\n")
			} else {
				buf.WriteString("Connection: close\r\n")
			}
		}
	}

	buf.WriteString("\r\n")

	// Write body
	if len(resp.Body) > 0 {
		buf.Write(resp.Body)
	}

	// Get the bytes
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())

	return result
}

// ProtoString returns the HTTP protocol string
func (resp *Response) ProtoString() string {
	return "HTTP/1.1"
}

// SetHeader sets a response header
func (resp *Response) SetHeader(key, value string) {
	if resp.Headers == nil {
		resp.Headers = make(map[string]string)
	}
	resp.Headers[key] = value
}

// SetContentType sets the Content-Type header
func (resp *Response) SetContentType(contentType string) {
	resp.SetHeader("Content-Type", contentType)
}

// SetCookie sets a cookie header
func (resp *Response) SetCookie(cookie *http.Cookie) {
	resp.SetHeader("Set-Cookie", cookie.String())
}

// Write writes data to the response body
func (resp *Response) Write(data []byte) (int, error) {
	resp.Body = append(resp.Body, data...)
	return len(data), nil
}

// WriteString writes a string to the response body
func (resp *Response) WriteString(s string) (int, error) {
	return resp.Write([]byte(s))
}

// Writer returns an io.Writer for the response
func (resp *Response) Writer() io.Writer {
	return resp
}

// JSONResponse creates a JSON response
func JSON(data interface{}, status int) *Response {
	resp := &Response{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}

	if data != nil {
		// Use sonic for fast JSON serialization
		if bytes, err := JSONMarshal(data); err == nil {
			resp.Body = bytes
			resp.Headers["Content-Length"] = strconv.Itoa(len(bytes))
		} else {
			resp.StatusCode = 500
			resp.Body = []byte(`{"error":"internal server error"}`)
		}
	}

	return resp
}

// TextResponse creates a text response
func Text(data string, status int) *Response {
	return &Response{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type":   "text/plain; charset=utf-8",
			"Content-Length": strconv.Itoa(len(data)),
		},
		Body:      []byte(data),
		KeepAlive: true,
	}
}

// HTMLResponse creates an HTML response
func HTML(data string, status int) *Response {
	return &Response{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type":   "text/html; charset=utf-8",
			"Content-Length": strconv.Itoa(len(data)),
		},
		Body:      []byte(data),
		KeepAlive: true,
	}
}

// FileResponse creates a file response with appropriate content type
func FileResponse(data []byte, contentType string, status int) *Response {
	return &Response{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type":   contentType,
			"Content-Length": strconv.Itoa(len(data)),
		},
		Body:      data,
		KeepAlive: true,
	}
}

// Redirect creates a redirect response
func Redirect(url string, status int) *Response {
	return &Response{
		StatusCode: status,
		Headers: map[string]string{
			"Location": url,
		},
		KeepAlive: false,
	}
}

// NotFound returns a 404 response
func NotFound(req *Request) *Response {
	return &Response{
		StatusCode: 404,
		Headers: map[string]string{
			"Content-Type": "text/plain",
		},
		Body:      []byte("404 Not Found"),
		KeepAlive: false,
	}
}

// MethodNotAllowed returns a 405 response
func MethodNotAllowed(req *Request) *Response {
	return &Response{
		StatusCode: 405,
		Headers: map[string]string{
			"Content-Type": "text/plain",
			"Allow":        "GET, POST, PUT, DELETE, OPTIONS",
		},
		Body:      []byte("405 Method Not Allowed"),
		KeepAlive: false,
	}
}

// BadRequest returns a 400 response
func BadRequest(message string) *Response {
	return &Response{
		StatusCode: 400,
		Headers: map[string]string{
			"Content-Type": "text/plain",
		},
		Body:      []byte(message),
		KeepAlive: false,
	}
}

// Unauthorized returns a 401 response
func Unauthorized(message string) *Response {
	return &Response{
		StatusCode: 401,
		Headers: map[string]string{
			"Content-Type": "text/plain",
			"WWW-Authenticate": "Basic realm=\"Restricted\"",
		},
		Body:      []byte(message),
		KeepAlive: false,
	}
}

// Forbidden returns a 403 response
func Forbidden(message string) *Response {
	return &Response{
		StatusCode: 403,
		Headers: map[string]string{
			"Content-Type": "text/plain",
		},
		Body:      []byte(message),
		KeepAlive: false,
	}
}

// InternalServerError returns a 500 response
func InternalServerError(message string) *Response {
	return &Response{
		StatusCode: 500,
		Headers: map[string]string{
			"Content-Type": "text/plain",
		},
		Body:      []byte(message),
		KeepAlive: false,
	}
}

// OK returns a 200 OK response with data
func OK(data interface{}) *Response {
	return JSON(data, 200)
}

// Created returns a 201 Created response with data
func Created(data interface{}) *Response {
	return JSON(data, 201)
}

// NoContent returns a 204 No Content response
func NoContent() *Response {
	return &Response{
		StatusCode: 204,
		KeepAlive:  true,
	}
}

// Accepted returns a 202 Accepted response
func Accepted(data interface{}) *Response {
	return JSON(data, 202)
}

// Empty returns an empty 200 OK response
func Empty() *Response {
	return &Response{
		StatusCode: 200,
		Body:       []byte{},
		KeepAlive:  true,
	}
}

// ErrorResponse returns an error response
func ErrorResponse(status int, message string) *Response {
	return JSON(map[string]interface{}{
		"error": message,
	}, status)
}

// ValidationError returns a validation error response
func ValidationError(errors map[string]string) *Response {
	return JSON(map[string]interface{}{
		"error":   "validation failed",
		"details": errors,
	}, 400)
}

// CORSOptions represents CORS configuration
type CORSOptions struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// DefaultCORSOptions returns default CORS options
func DefaultCORSOptions() *CORSOptions {
	return &CORSOptions{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           86400,
	}
}

// CORSResponse creates a CORS preflight response
func CORSResponse(opts *CORSOptions, req *Request) *Response {
	if opts == nil {
		opts = DefaultCORSOptions()
	}

	origin := req.Header("Origin")
	allowedOrigin := "*"
	if len(opts.AllowedOrigins) > 0 && opts.AllowedOrigins[0] != "*" {
		for _, allowed := range opts.AllowedOrigins {
			if allowed == origin || allowed == "*" {
				allowedOrigin = origin
				break
			}
		}
	} else {
		allowedOrigin = origin
		if allowedOrigin == "" {
			allowedOrigin = "*"
		}
	}

	resp := &Response{
		StatusCode: 204,
		Headers: map[string]string{
			"Access-Control-Allow-Origin":      allowedOrigin,
			"Access-Control-Allow-Methods":     strings.Join(opts.AllowedMethods, ", "),
			"Access-Control-Allow-Headers":     strings.Join(opts.AllowedHeaders, ", "),
			"Access-Control-Max-Age":           strconv.Itoa(opts.MaxAge),
		},
		KeepAlive: true,
	}

	if opts.AllowCredentials {
		resp.Headers["Access-Control-Allow-Credentials"] = "true"
	}

	if len(opts.ExposedHeaders) > 0 {
		resp.Headers["Access-Control-Expose-Headers"] = strings.Join(opts.ExposedHeaders, ", ")
	}

	return resp
}

// JSONMarshal marshals data to JSON using sonic for high performance
func JSONMarshal(data interface{}) ([]byte, error) {
	return sonic.Marshal(data)
}

// JSONUnmarshal unmarshals JSON data using sonic for high performance
func JSONUnmarshal(data []byte, v interface{}) error {
	return sonic.Unmarshal(data, v)
}
