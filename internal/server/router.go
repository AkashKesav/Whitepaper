package server

import (
	"fmt"
	"strings"
	"sync"
)

// Router handles HTTP routing with support for:
// - Exact path matching
// - Wildcard patterns
// - Path parameters (e.g., /user/{id})
// - Method-specific routes
type Router struct {
	mu       sync.RWMutex
	routes   map[string]*routeNode // method -> route tree
	anyRoutes *routeNode           // routes that match any method
}

// routeNode represents a node in the routing tree
type routeNode struct {
	// Exact path match
	exact map[string]*routeNode

	// Wildcard parameter (e.g., {id})
	wildcard *wildcardNode

	// Handler for this node
	handler HandlerFunc

	// Middleware for this node
	middleware []MiddlewareFunc
}

// wildcardNode represents a wildcard parameter node
type wildcardNode struct {
	name     string         // parameter name
	handler  HandlerFunc    // handler to execute
	children *routeNode     // child routes
	middleware []MiddlewareFunc
}

// HandlerFunc is a function that handles an HTTP request
type HandlerFunc func(*Request) *Response

// MiddlewareFunc is a function that wraps a handler
type MiddlewareFunc func(HandlerFunc) HandlerFunc

// NewRouter creates a new router
func NewRouter() *Router {
	return &Router{
		routes:   make(map[string]*routeNode),
		anyRoutes: &routeNode{
			exact: make(map[string]*routeNode),
		},
	}
}

// Add adds a route for a specific method
func (r *Router) Add(method, path string, handler HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Normalize method
	method = strings.ToUpper(method)

	// Get or create method-specific route tree
	root, ok := r.routes[method]
	if !ok {
		root = &routeNode{
			exact: make(map[string]*routeNode),
		}
		r.routes[method] = root
	}

	// Add the route
	r.addRoute(root, path, handler)
}

// AddAny adds a route that matches any method
func (r *Router) AddAny(path string, handler HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.addRoute(r.anyRoutes, path, handler)
}

// addRoute adds a route to the given root node
func (r *Router) addRoute(root *routeNode, path string, handler HandlerFunc) {
	segments := r.splitPath(path)
	current := root

	for i, segment := range segments {
		isLast := i == len(segments)-1
		isWildcard := strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")

		if isWildcard {
			// Extract parameter name
			paramName := segment[1 : len(segment)-1]

			// Create or get wildcard node
			if current.wildcard == nil {
				current.wildcard = &wildcardNode{
					name:     paramName,
					children: &routeNode{exact: make(map[string]*routeNode)},
				}
			}

			if isLast {
				current.wildcard.handler = handler
			} else {
				current = current.wildcard.children
			}
		} else {
			// Create or get exact node
			next, ok := current.exact[segment]
			if !ok {
				next = &routeNode{
					exact: make(map[string]*routeNode),
				}
				current.exact[segment] = next
			}

			if isLast {
				next.handler = handler
			} else {
				current = next
			}
		}
	}
}

// Route finds the handler for a given request
func (r *Router) Route(req *Request) HandlerFunc {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try method-specific routes first
	if root, ok := r.routes[req.Method]; ok {
		if handler := r.findRoute(root, req); handler != nil {
			return handler
		}
	}

	// Try any-method routes
	if handler := r.findRoute(r.anyRoutes, req); handler != nil {
		return handler
	}

	return nil
}

// findRoute recursively finds a handler for the request
func (r *Router) findRoute(node *routeNode, req *Request) HandlerFunc {
	segments := r.splitPath(req.Path)
	params := make(map[string]string)
	handler := r.findRouteRecursive(node, segments, 0, params)

	if handler != nil && len(params) > 0 {
		req.PathParams = params
	}

	return handler
}

// findRouteRecursive recursively searches for a matching route
func (r *Router) findRouteRecursive(node *routeNode, segments []string, index int, params map[string]string) HandlerFunc {
	// Check if we've reached the end of the path
	if index >= len(segments) {
		return node.handler
	}

	segment := segments[index]

	// Try exact match first
	if next, ok := node.exact[segment]; ok {
		if handler := r.findRouteRecursive(next, segments, index+1, params); handler != nil {
			return handler
		}
	}

	// Try wildcard match
	if node.wildcard != nil {
		params[node.wildcard.name] = segment
		if index == len(segments)-1 {
			return node.wildcard.handler
		}
		if handler := r.findRouteRecursive(node.wildcard.children, segments, index+1, params); handler != nil {
			return handler
		}
		// Remove param if no match found
		delete(params, node.wildcard.name)
	}

	return nil
}

// splitPath splits a path into segments
func (r *Router) splitPath(path string) []string {
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "/")
}

// Group creates a route group with a common prefix
type Group struct {
	router     *Router
	prefix     string
	middleware []MiddlewareFunc
}

// Group creates a new route group
func (r *Router) Group(prefix string) *Group {
	return &Group{
		router: r,
		prefix: strings.TrimSuffix(prefix, "/"),
	}
}

// Use adds middleware to the group
func (g *Group) Use(mw MiddlewareFunc) *Group {
	g.middleware = append(g.middleware, mw)
	return g
}

// Handle adds a route to the group
func (g *Group) Handle(method, path string, handler HandlerFunc) *Group {
	fullPath := g.prefix + "/" + strings.TrimPrefix(path, "/")
	if g.prefix == "" {
		fullPath = "/" + strings.TrimPrefix(path, "/")
	}

	// Apply group middleware
	for i := len(g.middleware) - 1; i >= 0; i-- {
		handler = g.middleware[i](handler)
	}

	g.router.Add(method, fullPath, handler)
	return g
}

// GET adds a GET route to the group
func (g *Group) GET(path string, handler HandlerFunc) *Group {
	return g.Handle("GET", path, handler)
}

// POST adds a POST route to the group
func (g *Group) POST(path string, handler HandlerFunc) *Group {
	return g.Handle("POST", path, handler)
}

// PUT adds a PUT route to the group
func (g *Group) PUT(path string, handler HandlerFunc) *Group {
	return g.Handle("PUT", path, handler)
}

// DELETE adds a DELETE route to the group
func (g *Group) DELETE(path string, handler HandlerFunc) *Group {
	return g.Handle("DELETE", path, handler)
}

// PATCH adds a PATCH route to the group
func (g *Group) PATCH(path string, handler HandlerFunc) *Group {
	return g.Handle("PATCH", path, handler)
}

// OPTIONS adds an OPTIONS route to the group
func (g *Group) OPTIONS(path string, handler HandlerFunc) *Group {
	return g.Handle("OPTIONS", path, handler)
}

// HEAD adds a HEAD route to the group
func (g *Group) HEAD(path string, handler HandlerFunc) *Group {
	return g.Handle("HEAD", path, handler)
}

// Subgroup creates a subgroup with an additional prefix
func (g *Group) Subgroup(prefix string) *Group {
	return &Group{
		router:     g.router,
		prefix:     g.prefix + "/" + strings.TrimPrefix(prefix, "/"),
		middleware: append([]MiddlewareFunc{}, g.middleware...),
	}
}

// String returns a string representation of the router
func (r *Router) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("Router{\n")

	for method, root := range r.routes {
		sb.WriteString(fmt.Sprintf("  %s:\n", method))
		r.printNode(root, "    ", &sb)
	}

	if len(r.anyRoutes.exact) > 0 || r.anyRoutes.wildcard != nil {
		sb.WriteString("  ANY:\n")
		r.printNode(r.anyRoutes, "    ", &sb)
	}

	sb.WriteString("}")
	return sb.String()
}

// printNode recursively prints a route node
func (r *Router) printNode(node *routeNode, indent string, sb *strings.Builder) {
	for path, child := range node.exact {
		sb.WriteString(fmt.Sprintf("%s%s\n", indent, path))
		r.printNode(child, indent+"  ", sb)
	}
	if node.wildcard != nil {
		sb.WriteString(fmt.Sprintf("%s{%s}\n", indent, node.wildcard.name))
		if node.wildcard.handler != nil {
			sb.WriteString(fmt.Sprintf("%s  -> handler\n", indent))
		}
		r.printNode(node.wildcard.children, indent+"  ", sb)
	}
	if node.handler != nil {
		sb.WriteString(fmt.Sprintf("%s-> handler\n", indent))
	}
}

// Routes returns all registered routes
func (r *Router) Routes() []RouteInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var routes []RouteInfo
	for method, root := range r.routes {
		r.collectRoutes(root, method, "", &routes)
	}
	r.collectRoutes(r.anyRoutes, "ANY", "", &routes)
	return routes
}

// RouteInfo represents information about a route
type RouteInfo struct {
	Method  string
	Path    string
	Handler string
}

// collectRoutes recursively collects route information
func (r *Router) collectRoutes(node *routeNode, method, path string, routes *[]RouteInfo) {
	for segment, child := range node.exact {
		newPath := path + "/" + segment
		if child.handler != nil {
			*routes = append(*routes, RouteInfo{
				Method:  method,
				Path:    newPath,
				Handler: fmt.Sprintf("%p", child.handler),
			})
		}
		r.collectRoutes(child, method, newPath, routes)
	}
	if node.wildcard != nil {
		newPath := path + "/{" + node.wildcard.name + "}"
		if node.wildcard.handler != nil {
			*routes = append(*routes, RouteInfo{
				Method:  method,
				Path:    newPath,
				Handler: fmt.Sprintf("%p", node.wildcard.handler),
			})
		}
		r.collectRoutes(node.wildcard.children, method, newPath, routes)
	}
}
