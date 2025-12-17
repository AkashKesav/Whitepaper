package precortex

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
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

// CachedEntry represents a cached query-response pair with its embedding
type CachedEntry struct {
	Query     string    `json:"query"`
	Response  string    `json:"response"`
	Embedding []float32 `json:"embedding"`
	CreatedAt time.Time `json:"created_at"`
}

// SemanticCache provides vector-based semantic caching using Ollama embeddings
type SemanticCache struct {
	cacheManager *cache.Manager
	redisClient  *redis.Client
	embedder     Embedder
	threshold    float64
	logger       *zap.Logger

	// In-memory vector index for fast similarity search
	// Key: namespace, Value: list of cached entries
	vectorIndex map[string][]CachedEntry
	indexMu     sync.RWMutex
}

// NewSemanticCache creates a new semantic cache with vector similarity support
func NewSemanticCache(cacheManager *cache.Manager, embedder Embedder, threshold float64, logger *zap.Logger) *SemanticCache {
	if threshold <= 0 || threshold > 1 {
		threshold = DefaultSimilarityThreshold
	}

	sc := &SemanticCache{
		cacheManager: cacheManager,
		embedder:     embedder,
		threshold:    threshold,
		logger:       logger,
		vectorIndex:  make(map[string][]CachedEntry),
	}

	logger.Info("Semantic cache initialized",
		zap.Float64("threshold", threshold),
		zap.Bool("embedder_available", embedder != nil))

	return sc
}

// SetRedisClient sets the Redis client for persistent vector storage
func (sc *SemanticCache) SetRedisClient(client *redis.Client) {
	sc.redisClient = client
	sc.logger.Info("Redis client configured for semantic cache persistence")
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

	// 2. If we have an embedder, try vector similarity search
	if sc.embedder != nil {
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

	// If we have an embedder, also store the vector for similarity search
	if sc.embedder != nil {
		go sc.storeVector(ctx, namespace, query, response)
	}
}

// vectorSearch performs semantic similarity search
func (sc *SemanticCache) vectorSearch(ctx context.Context, namespace, query string) (string, float32, bool) {
	// Generate embedding for query
	queryVec, err := sc.embedder.Embed(query)
	if err != nil {
		sc.logger.Warn("Failed to embed query for search", zap.Error(err))
		return "", 0, false
	}

	// Search in-memory index
	sc.indexMu.RLock()
	entries, exists := sc.vectorIndex[namespace]
	sc.indexMu.RUnlock()

	if !exists || len(entries) == 0 {
		// Try loading from Redis if available
		if sc.redisClient != nil {
			entries = sc.loadFromRedis(ctx, namespace)
			if len(entries) > 0 {
				sc.indexMu.Lock()
				sc.vectorIndex[namespace] = entries
				sc.indexMu.Unlock()
			}
		}
	}

	if len(entries) == 0 {
		return "", 0, false
	}

	// Find the most similar entry
	var bestMatch *CachedEntry
	var bestSimilarity float32 = 0

	for i := range entries {
		similarity := cosineSimilarity(queryVec, entries[i].Embedding)
		if similarity > bestSimilarity {
			bestSimilarity = similarity
			bestMatch = &entries[i]
		}
	}

	// Check if similarity exceeds threshold
	if bestMatch != nil && bestSimilarity >= float32(sc.threshold) {
		sc.logger.Debug("Vector search found match",
			zap.String("matched_query", bestMatch.Query[:min(30, len(bestMatch.Query))]),
			zap.Float32("similarity", bestSimilarity))
		return bestMatch.Response, bestSimilarity, true
	}

	return "", bestSimilarity, false
}

// storeVector stores a query embedding for future similarity search
func (sc *SemanticCache) storeVector(ctx context.Context, namespace, query, response string) {
	// Generate embedding
	vec, err := sc.embedder.Embed(query)
	if err != nil {
		sc.logger.Warn("Failed to embed query for storage", zap.Error(err))
		return
	}

	entry := CachedEntry{
		Query:     query,
		Response:  response,
		Embedding: vec,
		CreatedAt: time.Now(),
	}

	// Store in memory index
	sc.indexMu.Lock()
	entries := sc.vectorIndex[namespace]

	// Check if we already have this exact query
	for i, e := range entries {
		if normalizeQuery(e.Query) == normalizeQuery(query) {
			// Update existing entry
			entries[i] = entry
			sc.vectorIndex[namespace] = entries
			sc.indexMu.Unlock()
			sc.persistToRedis(ctx, namespace, entries)
			return
		}
	}

	// Add new entry, respecting max size
	entries = append(entries, entry)
	if len(entries) > MaxCachedVectors {
		// Remove oldest entries (FIFO)
		entries = entries[len(entries)-MaxCachedVectors:]
	}
	sc.vectorIndex[namespace] = entries
	sc.indexMu.Unlock()

	// Persist to Redis for durability
	sc.persistToRedis(ctx, namespace, entries)

	sc.logger.Debug("Stored vector in semantic cache",
		zap.String("namespace", namespace),
		zap.Int("embedding_dims", len(vec)),
		zap.Int("total_cached", len(entries)))
}

// persistToRedis saves the vector index to Redis for persistence
func (sc *SemanticCache) persistToRedis(ctx context.Context, namespace string, entries []CachedEntry) {
	if sc.redisClient == nil {
		return
	}

	key := fmt.Sprintf("semantic_vectors:%s", namespace)
	data, err := json.Marshal(entries)
	if err != nil {
		sc.logger.Warn("Failed to marshal entries for Redis", zap.Error(err))
		return
	}

	if err := sc.redisClient.Set(ctx, key, data, CacheTTL).Err(); err != nil {
		sc.logger.Warn("Failed to persist vectors to Redis", zap.Error(err))
	}
}

// loadFromRedis loads the vector index from Redis
func (sc *SemanticCache) loadFromRedis(ctx context.Context, namespace string) []CachedEntry {
	if sc.redisClient == nil {
		return nil
	}

	key := fmt.Sprintf("semantic_vectors:%s", namespace)
	data, err := sc.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			sc.logger.Warn("Failed to load vectors from Redis", zap.Error(err))
		}
		return nil
	}

	var entries []CachedEntry
	if err := json.Unmarshal([]byte(data), &entries); err != nil {
		sc.logger.Warn("Failed to unmarshal vectors from Redis", zap.Error(err))
		return nil
	}

	sc.logger.Debug("Loaded vectors from Redis",
		zap.String("namespace", namespace),
		zap.Int("count", len(entries)))

	return entries
}

// Stats returns cache statistics
func (sc *SemanticCache) Stats() map[string]interface{} {
	sc.indexMu.RLock()
	defer sc.indexMu.RUnlock()

	totalVectors := 0
	for _, entries := range sc.vectorIndex {
		totalVectors += len(entries)
	}

	return map[string]interface{}{
		"namespaces":      len(sc.vectorIndex),
		"total_vectors":   totalVectors,
		"threshold":       sc.threshold,
		"embedder_active": sc.embedder != nil,
	}
}

// cosineSimilarity computes the cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
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
