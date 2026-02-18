package server

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Chain creates a middleware chain from multiple middleware functions
func Chain(middlewares ...MiddlewareFunc) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// Logger is a middleware that logs requests
func Logger(logger interface{ Info(string, ...interface{}) }) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(req *Request) *Response {
			start := time.Now()
			resp := next(req)
			duration := time.Since(start)

			logger.Info("[HTTP]",
				"method", req.Method,
				"path", req.Path,
				"status", resp.StatusCode,
				"duration", duration,
				"remote", req.RemoteAddr,
			)

			return resp
		}
	}
}

// Recovery is a middleware that recovers from panics
func Recovery() MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(req *Request) *Response {
			defer func() {
				if r := recover(); r != nil {
					// Log the panic (in production, use proper logger)
					// For now, return an internal server error
				}
			}()
			return next(req)
		}
	}
}

// CORS is a middleware that handles CORS
func CORS(opts *CORSOptions) MiddlewareFunc {
	if opts == nil {
		opts = DefaultCORSOptions()
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(req *Request) *Response {
			// Handle preflight requests
			if req.Method == "OPTIONS" {
				return CORSResponse(opts, req)
			}

			// Add CORS headers to all responses
			resp := next(req)

			if resp.Headers == nil {
				resp.Headers = make(map[string]string)
			}

			origin := req.Header("Origin")
			allowedOrigin := "*"
			if len(opts.AllowedOrigins) > 0 && opts.AllowedOrigins[0] != "*" {
				for _, allowed := range opts.AllowedOrigins {
					if allowed == origin {
						allowedOrigin = origin
						break
					}
				}
			} else if origin != "" {
				allowedOrigin = origin
			}

			resp.Headers["Access-Control-Allow-Origin"] = allowedOrigin
			if opts.AllowCredentials {
				resp.Headers["Access-Control-Allow-Credentials"] = "true"
			}
			if len(opts.ExposedHeaders) > 0 {
				resp.Headers["Access-Control-Expose-Headers"] = strings.Join(opts.ExposedHeaders, ", ")
			}

			return resp
		}
	}
}

// BasicAuthMiddleware is a middleware that handles basic authentication
func BasicAuthMiddleware(username, password string) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(req *Request) *Response {
			user, pass, ok := req.BasicAuth()
			if !ok || user != username || pass != password {
				resp := Unauthorized("Authentication required")
				if resp.Headers == nil {
					resp.Headers = make(map[string]string)
				}
				resp.Headers["WWW-Authenticate"] = `Basic realm="Restricted"`
				return resp
			}
			return next(req)
		}
	}
}

// Timeout is a middleware that adds a timeout to requests
func Timeout(timeout time.Duration) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(req *Request) *Response {
			type result struct {
				resp *Response
			}

			done := make(chan result, 1)
			go func() {
				done <- result{resp: next(req)}
			}()

			select {
			case r := <-done:
				return r.resp
			case <-time.After(timeout):
				return ErrorResponse(408, "Request timeout")
			}
		}
	}
}

// RateLimiter is a simple rate limiter middleware
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	limit    int
	window   time.Duration
}

type visitor struct {
	requests  []time.Time
	lastSeen  time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
		window:   window,
	}

	// Cleanup old visitors periodically
	go rl.cleanup()

	return rl
}

// Allow checks if a request is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		v = &visitor{requests: make([]time.Time, 0)}
		rl.visitors[ip] = v
	}

	v.lastSeen = time.Now()

	// Remove old requests outside the window
	cutoff := time.Now().Add(-rl.window)
	validRequests := make([]time.Time, 0)
	for _, t := range v.requests {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}
	v.requests = validRequests

	// Check if limit exceeded
	if len(v.requests) >= rl.limit {
		return false
	}

	v.requests = append(v.requests, time.Now())
	return true
}

// cleanup removes stale visitors
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-time.Hour)
		for ip, v := range rl.visitors {
			if v.lastSeen.Before(cutoff) {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimit middleware
func (rl *RateLimiter) Middleware() MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(req *Request) *Response {
			ip := req.RemoteIP()
			if ip == "" {
				ip = req.RemoteAddr
			}

			if !rl.Allow(ip) {
				return ErrorResponse(429, "Too many requests")
			}

			return next(req)
		}
	}
}

// Gzip is a middleware that compresses responses with gzip
func Gzip(level int) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(req *Request) *Response {
			// Check if client accepts gzip
			if !strings.Contains(req.Header("Accept-Encoding"), "gzip") {
				return next(req)
			}

			// Get response
			resp := next(req)

			// Don't compress already compressed content types
			contentType := resp.Headers["Content-Type"]
			skipCompress := strings.Contains(contentType, "image/") ||
				strings.Contains(contentType, "video/") ||
				strings.Contains(contentType, "application/zip") ||
				strings.Contains(contentType, "application/gzip")

			if skipCompress || len(resp.Body) < 512 {
				return resp
			}

			// Compress body
			var buf strings.Builder
			w, _ := gzip.NewWriterLevel(&buf, level)
			w.Write(resp.Body)
			w.Close()

			resp.Body = []byte(buf.String())
			resp.Headers["Content-Encoding"] = "gzip"
			resp.Headers["Vary"] = "Accept-Encoding"

			// Update content length
			resp.Headers["Content-Length"] = strconv.Itoa(len(resp.Body))

			return resp
		}
	}
}

// Deflate is a middleware that compresses responses with deflate
func Deflate(level int) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(req *Request) *Response {
			if !strings.Contains(req.Header("Accept-Encoding"), "deflate") {
				return next(req)
			}

			resp := next(req)

			contentType := resp.Headers["Content-Type"]
			skipCompress := strings.Contains(contentType, "image/") ||
				strings.Contains(contentType, "video/") ||
				strings.Contains(contentType, "application/zip")

			if skipCompress || len(resp.Body) < 512 {
				return resp
			}

			w, _ := flate.NewWriter(nil, level)
			w.Write(resp.Body)
			w.Close()

			resp.Headers["Content-Encoding"] = "deflate"
			resp.Headers["Vary"] = "Accept-Encoding"

			return resp
		}
	}
}

// Headers is a middleware that adds custom headers to all responses
func Headers(headers map[string]string) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(req *Request) *Response {
			resp := next(req)

			if resp.Headers == nil {
				resp.Headers = make(map[string]string)
			}

			for key, value := range headers {
				resp.Headers[key] = value
			}

			return resp
		}
	}
}

// SecurityHeaders adds common security headers
func SecurityHeaders() MiddlewareFunc {
	return Headers(map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"X-XSS-Protection":        "1; mode=block",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"Permissions-Policy":      "geolocation=(), microphone=(), camera=()",
	})
}

// RealIP gets the real IP from the request, checking X-Real-IP and X-Forwarded-For headers
func RealIP() MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(req *Request) *Response {
			if ip := req.Header("X-Real-IP"); ip != "" {
				req.RemoteAddr = ip
			} else if ip := req.Header("X-Forwarded-For"); ip != "" {
				// Take the first IP from the list
				if idx := strings.Index(ip, ","); idx != -1 {
					req.RemoteAddr = strings.TrimSpace(ip[:idx])
				} else {
					req.RemoteAddr = ip
				}
			}

			return next(req)
		}
	}
}

// TrustProxy is a middleware that trusts proxy headers
func TrustProxy(trustedProxies []string) MiddlewareFunc {
	trustedSet := make(map[string]bool)
	for _, proxy := range trustedProxies {
		trustedSet[proxy] = true
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(req *Request) *Response {
			// Only use proxy headers if the remote address is trusted
			host, _, err := net.SplitHostPort(req.RemoteAddr)
			if err != nil {
				host = req.RemoteAddr
			}

			if trustedSet[host] || trustedSet["*"] {
				if ip := req.Header("X-Real-IP"); ip != "" {
					req.RemoteAddr = ip
				}
			}

			return next(req)
		}
	}
}

// StaticFile is a middleware that serves static files
type StaticFile struct {
	root     string
	indexPath string
}

// NewStaticFile creates a new static file middleware
func NewStaticFile(root, indexPath string) *StaticFile {
	if indexPath == "" {
		indexPath = "index.html"
	}
	return &StaticFile{
		root:     root,
		indexPath: indexPath,
	}
}

// Serve serves static files
func (s *StaticFile) Serve(next HandlerFunc) HandlerFunc {
	return func(req *Request) *Response {
		// If the request is for a file, try to serve it
		if req.Method == "GET" {
			// Try to find the file
			// This is a simple implementation - for production,
			// you'd want to add proper file serving logic
			// with proper MIME types and caching
		}

		return next(req)
	}
}

// MethodOverride is a middleware that checks for the X-HTTP-Method-Override header
func MethodOverride() MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(req *Request) *Response {
			if method := req.Header("X-HTTP-Method-Override"); method != "" {
				req.Method = method
			}
			return next(req)
		}
	}
}

// RequestID is a middleware that adds a unique request ID to each request
func RequestID() MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(req *Request) *Response {
			// Generate a unique ID (simple implementation)
			// In production, use a proper UUID generator
			reqID := req.Header("X-Request-ID")
			if reqID == "" {
				reqID = "req-" + time.Now().Format("20060102150405") + "-" + string(rune(time.Now().UnixNano()))
			}

			resp := next(req)
			if resp.Headers == nil {
				resp.Headers = make(map[string]string)
			}
			resp.Headers["X-Request-ID"] = reqID

			return resp
		}
	}
}

// MaxBytes is a middleware that limits the size of request bodies
func MaxBytes(maxBytes int64) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(req *Request) *Response {
			if int64(len(req.Body)) > maxBytes {
				return ErrorResponse(413, "Request entity too large")
			}
			return next(req)
		}
	}
}

// StripSlashes removes trailing slashes from the request path
func StripSlashes(new bool) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(req *Request) *Response {
			path := req.Path
			if len(path) > 1 && path[len(path)-1] == '/' {
				if new {
					// Permanently redirect to the path without trailing slash
					return Redirect(path[:len(path)-1], 301)
				}
				req.Path = path[:len(path)-1]
			}
			return next(req)
		}
	}
}

// RequestSize is a middleware that limits the size of request bodies based on content type
func RequestSize(maxBytes int64) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(req *Request) *Response {
			contentLength := req.ContentLength()
			if contentLength > maxBytes {
				return ErrorResponse(413, "Request entity too large")
			}
			return next(req)
		}
	}
}

// CombinedReadWriteCloser wraps a reader and writer together
type CombinedReadWriteCloser struct {
	io.Reader
	io.Writer
}

func (c *CombinedReadWriteCloser) Close() error {
	return nil
}
