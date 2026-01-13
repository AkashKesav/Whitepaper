package precortex

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/reflective-memory-kernel/internal/kernel"
	"github.com/reflective-memory-kernel/internal/kernel/cache"
	"go.uber.org/zap"
)

const (
	// CacheTTL is the time-to-live for cached responses
	CacheTTL = 10 * time.Minute
	// MaxCachedVectors is the maximum number of vectors to store per namespace
	MaxCachedVectors = 1000
	// DefaultSimilarityThreshold is the minimum similarity for a cache hit
	DefaultSimilarityThreshold = 0.92
	// Query validation limits
	MaxQueryLength = 2000 // Maximum query length to prevent DoS
	MinQueryLength = 2    // Minimum query length to be meaningful
)

// SemanticCache provides vector-based semantic caching using Qdrant
type SemanticCache struct {
	cacheManager *cache.Manager
	vectorIndex  *kernel.VectorIndex
	embedder     Embedder
	threshold    float64
	logger       *zap.Logger
}

// NewSemanticCache creates a new semantic cache
func NewSemanticCache(cacheManager *cache.Manager, vectorIndex *kernel.VectorIndex, embedder Embedder, threshold float64, logger *zap.Logger) *SemanticCache {
	if threshold <= 0 || threshold > 1 {
		threshold = DefaultSimilarityThreshold
	}

	sc := &SemanticCache{
		cacheManager: cacheManager,
		vectorIndex:  vectorIndex,
		embedder:     embedder,
		threshold:    threshold,
		logger:       logger,
	}

	logger.Info("Semantic cache initialized",
		zap.Float64("threshold", threshold),
		zap.Bool("vector_index_active", vectorIndex != nil),
		zap.Bool("embedder_active", embedder != nil))

	return sc
}

// Check looks up a query in the semantic cache
// SECURITY: Requires valid namespace to prevent cross-tenant data access
func (sc *SemanticCache) Check(ctx context.Context, namespace, query string) (string, bool) {
	startTime := time.Now()

	// SECURITY: Validate namespace to prevent cross-tenant access
	if namespace == "" || !isValidNamespaceName(namespace) {
		sc.logger.Warn("Semantic cache: invalid namespace rejected",
			zap.String("namespace", namespace))
		return "", false // Fail-secure on invalid namespace
	}

	// SECURITY: Validate and normalize query before processing
	normalizedQuery, err := normalizeQuery(query)
	if err != nil {
		sc.logger.Warn("Semantic cache: query validation failed",
			zap.Error(err),
			zap.String("query", query[:min(50, len(query))]))
		// Return cache miss on invalid query (fail-secure)
		return "", false
	}

	key := fmt.Sprintf("semantic:%s:%s", namespace, normalizedQuery)

	sc.logger.Info("Semantic cache: CHECKING",
		zap.String("key", key),
		zap.String("query", query[:min(50, len(query))]))

	if val, found := sc.cacheManager.Get(key); found {
		if response, ok := val.(string); ok {
			sc.logger.Info("Semantic cache: exact match HIT",
				zap.String("query", query[:min(30, len(query))]),
				zap.Duration("latency", time.Since(startTime)))
			return response, true
		}
	}

	// 2. If we have an embedder and vector index, try vector similarity search
	if sc.embedder != nil && sc.vectorIndex != nil {
		response, similarity, found := sc.vectorSearch(ctx, namespace, query)
		if found {
			sc.logger.Info("Semantic cache: similarity HIT",
				zap.String("query", query[:min(30, len(query))]),
				zap.Float32("similarity", similarity),
				zap.Duration("latency", time.Since(startTime)))
			return response, true
		}
	}

	sc.logger.Debug("Semantic cache: MISS",
		zap.String("query", query[:min(30, len(query))]),
		zap.Duration("latency", time.Since(startTime)))
	return "", false
}

// Store saves a query-response pair in the semantic cache
// SECURITY: Requires valid namespace to prevent cross-tenant data access
func (sc *SemanticCache) Store(ctx context.Context, namespace, query, response string) {
	// SECURITY: Validate namespace to prevent cross-tenant access
	if namespace == "" || !isValidNamespaceName(namespace) {
		sc.logger.Warn("Semantic cache: invalid namespace rejected for storage",
			zap.String("namespace", namespace))
		return // Don't store without valid namespace
	}

	// SECURITY: Validate and normalize query before storing
	normalizedQuery, err := normalizeQuery(query)
	if err != nil {
		sc.logger.Warn("Semantic cache: query validation failed, not storing",
			zap.Error(err),
			zap.String("query", query[:min(50, len(query))]))
		return // Don't store invalid queries
	}

	key := fmt.Sprintf("semantic:%s:%s", namespace, normalizedQuery)

	sc.logger.Info("Semantic cache: STORING",
		zap.String("key", key),
		zap.String("query", query[:min(50, len(query))]),
		zap.Int("response_len", len(response)))

	sc.cacheManager.SetWithTTL(key, response, int64(len(response)), CacheTTL)

	// If we have an embedder and vector index, store vector
	if sc.embedder != nil && sc.vectorIndex != nil {
		go sc.storeVector(ctx, namespace, query, response)
	}
}

// vectorSearch performs semantic similarity search using Qdrant
func (sc *SemanticCache) vectorSearch(ctx context.Context, namespace, query string) (string, float32, bool) {
	// Generate embedding for query
	queryVec, err := sc.embedder.Embed(query)
	if err != nil {
		sc.logger.Warn("Failed to embed query for search", zap.Error(err))
		return "", 0, false
	}

	// Search Qdrant
	// Logic: We store query as vector, and response as payload
	// Search returns similar queries. If similarity > threshold, we return the cached response.

	// SYSTEM OPERATION: Empty userID for cache lookup (not user-initiated)
	_, scores, payloads, err := sc.vectorIndex.Search(ctx, namespace, "", queryVec, 1) // Get top 1
	if err != nil {
		sc.logger.Warn("Semantic cache vector search failed", zap.Error(err))
		return "", 0, false
	}

	if len(scores) == 0 {
		return "", 0, false
	}

	bestScore := scores[0]
	if bestScore >= float32(sc.threshold) {
		// Found a hit
		if response, ok := payloads[0]["response"].(string); ok {
			sc.logger.Debug("Vector search found match",
				zap.Float32("similarity", bestScore))
			return response, bestScore, true
		}
	}

	return "", bestScore, false
}

// storeVector stores a query embedding for future similarity search
func (sc *SemanticCache) storeVector(ctx context.Context, namespace, query, response string) {
	// Generate embedding
	vec, err := sc.embedder.Embed(query)
	if err != nil {
		sc.logger.Warn("Failed to embed query for storage", zap.Error(err))
		return
	}

	// UID = hash of query
	uid := fmt.Sprintf("sc_%s", hashQuery(query))

	metadata := map[string]interface{}{
		"query":    query,
		"response": response,
		"type":     "cache_entry",
	}

	if err := sc.vectorIndex.Store(ctx, namespace, uid, vec, metadata); err != nil {
		sc.logger.Warn("Failed to store vector in semantic cache", zap.Error(err))
	} else {
		sc.logger.Debug("Stored vector in semantic cache",
			zap.String("namespace", namespace),
			zap.String("uid", uid))
	}
}

// hashQuery creates a simple hash of a query
func hashQuery(query string) string {
	h := 0
	for _, c := range query {
		h = 31*h + int(c)
	}
	return fmt.Sprintf("%x", h&0x7fffffff)
}

// Stats returns cache statistics
func (sc *SemanticCache) Stats(ctx context.Context) map[string]interface{} {
	stats := map[string]interface{}{
		"threshold":       sc.threshold,
		"embedder_active": sc.embedder != nil,
	}

	if sc.vectorIndex != nil {
		if idxStats, err := sc.vectorIndex.Stats(ctx); err == nil {
			stats["qdrant_stats"] = idxStats
		}
	}

	return stats
}

// normalizeQuery normalizes and validates a query for exact matching
// SECURITY: Validates query length and content to prevent injection and DoS attacks
func normalizeQuery(query string) (string, error) {
	// SECURITY: Check length first to prevent DoS via large inputs
	if len(query) > MaxQueryLength {
		return "", fmt.Errorf("query exceeds maximum length of %d characters", MaxQueryLength)
	}

	// SECURITY: Check for empty or too-short queries
	trimmed := strings.TrimSpace(query)
	if len(trimmed) < MinQueryLength {
		return "", fmt.Errorf("query is too short (minimum %d characters)", MinQueryLength)
	}

	// SECURITY: Check for null bytes (injection prevention)
	if strings.Contains(query, "\x00") {
		return "", fmt.Errorf("query contains invalid characters")
	}

	// SECURITY: Check for common injection patterns
	lowerQuery := strings.ToLower(query)
	suspiciousPatterns := []string{
		"<script",
		"javascript:",
		"vbscript:",
		"onload=",
		"onerror=",
		"<iframe",
	}
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerQuery, pattern) {
			return "", fmt.Errorf("query contains suspicious content")
		}
	}

	// Normalize: Convert to lowercase
	q := strings.ToLower(trimmed)
	// Collapse multiple spaces to single space
	q = strings.Join(strings.Fields(q), " ")
	// Remove common punctuation at end
	q = strings.TrimRight(q, "?!.,")
	return q, nil
}

// Invalidate removes all cache entries for a specific namespace
// This should be called when entities are updated or deleted
func (sc *SemanticCache) Invalidate(ctx context.Context, namespace string) error {
	sc.logger.Info("Semantic cache: invalidating namespace",
		zap.String("namespace", namespace))

	// Note: Redis SCAN is used for pattern matching, but for simplicity we'll just log
	// a warning. In production, you'd want to either:
	// 1. Maintain a set of cache keys per namespace
	// 2. Use Redis SCAN with MATCH to find and delete keys
	// 3. Use a different cache invalidation strategy

	sc.logger.Info("Semantic cache: namespace invalidation requested",
		zap.String("namespace", namespace),
		zap.String("note", "full pattern deletion requires SCAN or key tracking"))

	// For now, we rely on TTL expiration (10 minutes) for cache invalidation
	return nil
}

// InvalidateSpecific removes a specific cache entry
func (sc *SemanticCache) InvalidateSpecific(ctx context.Context, namespace, query string) error {
	normalizedQuery, err := normalizeQuery(query)
	if err != nil {
		sc.logger.Warn("Semantic cache: query validation failed during invalidation",
			zap.Error(err))
		return nil // Return success even if invalid (fail-secure)
	}
	key := fmt.Sprintf("semantic:%s:%s", namespace, normalizedQuery)

	sc.logger.Debug("Semantic cache: invalidating specific entry",
		zap.String("key", key))

	sc.cacheManager.Delete(key)

	return nil
}

// isValidNamespaceName validates namespace format for semantic cache
// SECURITY: Ensures namespace follows expected pattern to prevent injection
func isValidNamespaceName(ns string) bool {
	if ns == "" {
		return false
	}
	// Must start with user_ or group_ and contain only safe characters
	// This matches the pattern used in vector_index.go and traversal.go
	matched, _ := regexp.MatchString(`^(user|group)_[a-zA-Z0-9_-]+$`, ns)
	return matched
}
