// Package agent provides centralized namespace authorization
// SECURITY: Prevents namespace bypass attacks through access control
package agent

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2"
	"go.uber.org/zap"
)

// NamespaceAuthorizer provides centralized namespace access control
// with caching and constant-time operations to prevent enumeration attacks
type NamespaceAuthorizer struct {
	graphClient GraphClientInterface
	cache       *lru.Cache[string, *CachedAccessResult]
	logger      *zap.Logger
	mu          sync.RWMutex

	// For constant-time comparison on negative results
	negativeResultDelay time.Duration
}

// CachedAccessResult stores cached access decisions with TTL
type CachedAccessResult struct {
	Allowed     bool
	ExpiresAt   time.Time
	CacheKey    string
}

// GraphClientInterface defines the minimal interface for namespace authorization
type GraphClientInterface interface {
	// Check if user is member of a group namespace
	IsGroupMember(ctx context.Context, groupNamespace, userID string) (bool, error)
	// Check if user owns a user namespace
	OwnsUserNamespace(ctx context.Context, userNamespace, userID string) (bool, error)
}

// NamespaceType identifies the type of namespace
type NamespaceType string

const (
	NamespaceTypeUser  NamespaceType = "user"
	NamespaceTypeGroup NamespaceType = "group"
	NamespaceTypeInvalid NamespaceType = "invalid"
)

// NewNamespaceAuthorizer creates a new centralized namespace authorizer
func NewNamespaceAuthorizer(graphClient GraphClientInterface, logger *zap.Logger) *NamespaceAuthorizer {
	cache, _ := lru.New[string, *CachedAccessResult](500) // 500 cached decisions

	return &NamespaceAuthorizer{
		graphClient:         graphClient,
		cache:               cache,
		logger:              logger.Named("namespace_auth"),
		negativeResultDelay: 50 * time.Millisecond, // Normalize timing for denied access
	}
}

// VerifyAccess checks if a user has access to a namespace
// Returns constant-time responses to prevent timing attacks
func (na *NamespaceAuthorizer) VerifyAccess(ctx context.Context, userID, namespace string) (bool, error) {
	startTime := time.Now()

	// SECURITY: Validate namespace format first
	if !isValidNamespaceFormat(namespace) {
		na.logger.Warn("Namespace access rejected: invalid format",
			zap.String("namespace", namespace),
			zap.String("user", userID))
		// Constant-time delay for invalid format
		na.normalizeTiming(startTime, false)
		return false, fmt.Errorf("invalid namespace format")
	}

	// SECURITY: Reject empty namespace
	if namespace == "" {
		na.logger.Warn("Namespace access rejected: empty namespace",
			zap.String("user", userID))
		na.normalizeTiming(startTime, false)
		return false, fmt.Errorf("namespace cannot be empty")
	}

	// SECURITY: Reject empty userID
	if userID == "" {
		na.normalizeTiming(startTime, false)
		return false, fmt.Errorf("userID cannot be empty")
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s", userID, namespace)
	if cached, ok := na.cache.Get(cacheKey); ok {
		if time.Now().Before(cached.ExpiresAt) {
			// Cache hit - return cached result
			na.normalizeTiming(startTime, cached.Allowed)
			return cached.Allowed, nil
		}
		// Expired - remove from cache
		na.cache.Remove(cacheKey)
	}

	// Determine namespace type
	nsType := parseNamespaceType(namespace)
	var allowed bool
	var err error

	// Check access based on namespace type
	switch nsType {
	case NamespaceTypeUser:
		// User namespace: check if user owns it
		allowed, err = na.graphClient.OwnsUserNamespace(ctx, namespace, userID)
	case NamespaceTypeGroup:
		// Group namespace: check if user is member
		allowed, err = na.graphClient.IsGroupMember(ctx, namespace, userID)
	default:
		// Invalid namespace type
		na.normalizeTiming(startTime, false)
		return false, fmt.Errorf("invalid namespace type")
	}

	if err != nil {
		// On error, deny access for security (fail-secure)
		na.logger.Warn("Namespace access check failed, denying access",
			zap.String("namespace", namespace),
			zap.String("user", userID),
			zap.Error(err))
		na.normalizeTiming(startTime, false)
		return false, fmt.Errorf("access check failed")
	}

	// Cache the result (5 minute TTL)
	cached := &CachedAccessResult{
		Allowed:   allowed,
		ExpiresAt: time.Now().Add(5 * time.Minute),
		CacheKey:  cacheKey,
	}
	na.cache.Add(cacheKey, cached)

	// Normalize timing before returning
	na.normalizeTiming(startTime, allowed)

	if allowed {
		na.logger.Debug("Namespace access granted",
			zap.String("namespace", namespace),
			zap.String("user", userID))
	} else {
		na.logger.Warn("Namespace access denied",
			zap.String("namespace", namespace),
			zap.String("user", userID))
	}

	return allowed, nil
}

// VerifyGroupAdmin checks if a user is an admin of a group
// Similar constant-time behavior as VerifyAccess
func (na *NamespaceAuthorizer) VerifyGroupAdmin(ctx context.Context, userID, groupNamespace string) (bool, error) {
	startTime := time.Now()

	// SECURITY: Validate namespace format
	if !isValidNamespaceFormat(groupNamespace) {
		na.normalizeTiming(startTime, false)
		return false, fmt.Errorf("invalid namespace format")
	}

	// SECURITY: Must be a group namespace
	nsType := parseNamespaceType(groupNamespace)
	if nsType != NamespaceTypeGroup {
		na.normalizeTiming(startTime, false)
		return false, fmt.Errorf("not a group namespace")
	}

	// Check cache first
	cacheKey := fmt.Sprintf("admin:%s:%s", userID, groupNamespace)
	if cached, ok := na.cache.Get(cacheKey); ok {
		if time.Now().Before(cached.ExpiresAt) {
			na.normalizeTiming(startTime, cached.Allowed)
			return cached.Allowed, nil
		}
		na.cache.Remove(cacheKey)
	}

	// Query graph client for admin status
	// This requires extending the interface or using a direct call
	// For now, we'll use a separate method that can be implemented
	isAdmin, err := na.checkGroupAdmin(ctx, groupNamespace, userID)
	if err != nil {
		na.normalizeTiming(startTime, false)
		return false, fmt.Errorf("admin check failed")
	}

	// Cache the result
	cached := &CachedAccessResult{
		Allowed:   isAdmin,
		ExpiresAt: time.Now().Add(5 * time.Minute),
		CacheKey:  cacheKey,
	}
	na.cache.Add(cacheKey, cached)

	na.normalizeTiming(startTime, isAdmin)
	return isAdmin, nil
}

// checkGroupAdmin performs the actual admin check
// This can be extended to call the appropriate graph client method
func (na *NamespaceAuthorizer) checkGroupAdmin(ctx context.Context, groupNamespace, userID string) (bool, error) {
	// The actual implementation would call the graph client
	// For now, return a placeholder that should be replaced
	// with the actual IsGroupAdmin call
	return false, fmt.Errorf("not implemented")
}

// normalizeTiming normalizes response time to prevent timing attacks
// Ensures both positive and negative results take similar time
func (na *NamespaceAuthorizer) normalizeTiming(startTime time.Time, allowed bool) {
	elapsed := time.Since(startTime)

	// Target minimum time for all responses
	targetTime := na.negativeResultDelay

	// If result was too fast, add delay
	if elapsed < targetTime {
		time.Sleep(targetTime - elapsed)
	}
	// If result was slow, return immediately (no need to slow down successful requests)
}

// isValidNamespaceFormat validates namespace format using regex
// Valid formats: user_<alphanumeric> or group_<alphanumeric>
func isValidNamespaceFormat(ns string) bool {
	if ns == "" {
		return false
	}
	matched, _ := regexp.MatchString(`^(user|group)_[a-zA-Z0-9_-]+$`, ns)
	return matched
}

// parseNamespaceType extracts the namespace type from the namespace string
func parseNamespaceType(ns string) NamespaceType {
	if len(ns) < 5 {
		return NamespaceTypeInvalid
	}

	prefix := ns[:5]
	switch prefix {
	case "user_":
		return NamespaceTypeUser
	case "group":
		if len(ns) >= 6 && ns[5] == '_' {
			return NamespaceTypeGroup
		}
	}

	return NamespaceTypeInvalid
}

// InvalidateCache clears cached access decisions for a specific user or namespace
// Useful for revocation scenarios
func (na *NamespaceAuthorizer) InvalidateCache(userID, namespace string) {
	na.mu.Lock()
	defer na.mu.Unlock()

	// Invalidate user-specific cache
	keys := na.cache.Keys()
	for _, key := range keys {
		// Invalidate if it matches the user or namespace
		if userID != "" && contains(key, userID+":") {
			na.cache.Remove(key)
		}
		if namespace != "" && contains(key, ":"+namespace) {
			na.cache.Remove(key)
		}
	}

	na.logger.Info("Namespace auth cache invalidated",
		zap.String("user", userID),
		zap.String("namespace", namespace))
}

// InvalidateAll clears all cached access decisions
func (na *NamespaceAuthorizer) InvalidateAll() {
	na.mu.Lock()
	defer na.mu.Unlock()

	na.cache.Purge()
	na.logger.Info("All namespace auth cache cleared")
}

// contains is a simple string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (
		s[:len(substr)] == substr ||
		s[len(s)-len(substr):] == substr ||
		findInString(s, substr)))
}

// findInString checks if substr exists anywhere in s
func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetNamespaceStats returns statistics about the authorization cache
func (na *NamespaceAuthorizer) GetNamespaceStats() map[string]interface{} {
	na.mu.RLock()
	defer na.mu.RUnlock()

	return map[string]interface{}{
		"cache_size":     na.cache.Len(),
		"cache_max":      500,
		"negative_delay": na.negativeResultDelay.String(),
	}
}
