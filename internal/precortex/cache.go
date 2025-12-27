package precortex

import (
	"context"
	"fmt"
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
func (sc *SemanticCache) Check(ctx context.Context, namespace, query string) (string, bool) {
	startTime := time.Now()

	// 1. Try exact match first (fastest path)
	normalizedQuery := normalizeQuery(query)
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
func (sc *SemanticCache) Store(ctx context.Context, namespace, query, response string) {
	// Store exact match in fast cache
	normalizedQuery := normalizeQuery(query)
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

	_, scores, payloads, err := sc.vectorIndex.Search(ctx, namespace, queryVec, 1) // Get top 1
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

// normalizeQuery normalizes a query for exact matching
func normalizeQuery(query string) string {
	// Convert to lowercase
	q := strings.ToLower(query)
	// Trim whitespace
	q = strings.TrimSpace(q)
	// Collapse multiple spaces to single space
	q = strings.Join(strings.Fields(q), " ")
	// Remove common punctuation at end
	q = strings.TrimRight(q, "?!.,")
	return q
}
