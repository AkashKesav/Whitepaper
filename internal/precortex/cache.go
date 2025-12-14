package precortex

import (
	"context"
	"fmt"
	"time"

	"github.com/reflective-memory-kernel/internal/kernel/cache"
	"go.uber.org/zap"
)

// SemanticCache provides vector-based semantic caching
type SemanticCache struct {
	cacheManager *cache.Manager
	embedder     Embedder
	threshold    float64
	logger       *zap.Logger

	// Simple exact-match cache for now (will add HNSW vector index later)
	exactCache map[string]string
}

// NewSemanticCache creates a new semantic cache
func NewSemanticCache(cacheManager *cache.Manager, embedder Embedder, threshold float64, logger *zap.Logger) *SemanticCache {
	return &SemanticCache{
		cacheManager: cacheManager,
		embedder:     embedder,
		threshold:    threshold,
		logger:       logger,
		exactCache:   make(map[string]string),
	}
}

// Check looks up a query in the semantic cache
func (sc *SemanticCache) Check(ctx context.Context, namespace, query string) (string, bool) {
	// 1. Try exact match first (fastest)
	key := fmt.Sprintf("semantic:%s:%s", namespace, normalizeQuery(query))
	if val, found := sc.cacheManager.Get(key); found {
		if response, ok := val.(string); ok {
			return response, true
		}
	}

	// 2. If we have an embedder, try vector similarity
	if sc.embedder != nil {
		return sc.vectorSearch(ctx, namespace, query)
	}

	return "", false
}

// Store saves a query-response pair in the semantic cache
func (sc *SemanticCache) Store(ctx context.Context, namespace, query, response string) {
	key := fmt.Sprintf("semantic:%s:%s", namespace, normalizeQuery(query))

	// Store with 5-minute TTL
	sc.cacheManager.SetWithTTL(key, response, int64(len(response)), 5*time.Minute)

	// If we have an embedder, also store the vector
	if sc.embedder != nil {
		sc.storeVector(ctx, namespace, query, response)
	}
}

// vectorSearch performs semantic similarity search
func (sc *SemanticCache) vectorSearch(ctx context.Context, namespace, query string) (string, bool) {
	// Generate embedding for query
	queryVec, err := sc.embedder.Embed(query)
	if err != nil {
		sc.logger.Warn("Failed to embed query", zap.Error(err))
		return "", false
	}

	// TODO: Search HNSW index for similar vectors
	// For now, this is a placeholder
	_ = queryVec // Use the vector when HNSW is implemented

	return "", false
}

// storeVector stores a query vector in the index
func (sc *SemanticCache) storeVector(ctx context.Context, namespace, query, response string) {
	vec, err := sc.embedder.Embed(query)
	if err != nil {
		sc.logger.Warn("Failed to embed query for storage", zap.Error(err))
		return
	}

	// TODO: Add to HNSW index
	_ = vec // Use when HNSW is implemented
}

// normalizeQuery normalizes a query for exact matching
func normalizeQuery(query string) string {
	// Simple normalization - lowercase and trim
	// In production, add more sophisticated normalization
	return query
}
