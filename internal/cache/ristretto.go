// Package cache provides high-performance L1 in-memory caching using Ristretto.
// This eliminates network hops to Redis for hot path lookups.
package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// L1Cache provides a two-tier caching system:
// - L1: In-memory Ristretto cache (microsecond latency)
// - L2: Redis cache (millisecond latency, shared across instances)
type L1Cache struct {
	l1        *ristretto.Cache[string, []byte]
	l2        *redis.Client
	ttl       time.Duration
	l1MaxCost int64
	logger    *zap.Logger
	metrics   CacheMetrics
	metricsMu sync.Mutex
}

// CacheMetrics tracks cache performance
type CacheMetrics struct {
	L1Hits      int64
	L1Misses    int64
	L2Hits      int64
	L2Misses    int64
	L1Evictions int64
}

// CacheEntry represents a cached item with metadata
type CacheEntry struct {
	Data      []byte
	Timestamp int64
}

// NewL1Cache creates a new two-tier cache
// l1MaxCost: maximum cost of items in L1 cache (default: 10,000)
// ttl: time-to-live for cache entries (default: 5 minutes)
func NewL1Cache(l1MaxCost int64, ttl time.Duration, redisClient *redis.Client, logger *zap.Logger) (*L1Cache, error) {
	if l1MaxCost == 0 {
		l1MaxCost = 10000
	}
	if ttl == 0 {
		ttl = 5 * time.Minute
	}

	// Configure Ristretto with policies
	cache, err := ristretto.NewCache(&ristretto.Config[string, []byte]{
		NumCounters: 8,              // Number of keys to track frequency
		MaxCost:     l1MaxCost,      // Maximum cost (number of items)
		BufferItems: 64,              // Size of buffer per shard
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create ristretto cache: %w", err)
	}

	// Note: OnEvicted callback removed in Ristretto v2
	// Use metrics tracking through Get/Set methods instead

	return &L1Cache{
		l1:        cache,
		l2:        redisClient,
		ttl:       ttl,
		l1MaxCost: l1MaxCost,
		logger:    logger.Named("l1cache"),
	}, nil
}

// Get retrieves a value from L1, falling back to L2 if needed
// Returns the value and true if found, nil and false otherwise
func (c *L1Cache) Get(ctx context.Context, key string) ([]byte, bool) {
	// Try L1 first (in-memory, fastest)
	val, found := c.l1.Get(key)
	if found {
		c.recordL1Hit()
		c.logger.Debug("L1 cache hit", zap.String("key", key))
		return val, true
	}

	c.recordL1Miss()

	// Try L2 (Redis)
	if c.l2 != nil {
		c.logger.Debug("L1 miss, checking L2", zap.String("key", key))
		data, err := c.l2.Get(ctx, key).Bytes()
		if err == nil && len(data) > 0 {
			c.recordL2Hit()
			// Promote to L1
			c.l1.Set(key, data, int64(len(data)))
			// Set expiry on the promoted item
			go c.expireAfter(key, c.ttl)
			return data, true
		}
		c.recordL2Miss()
	}

	return nil, false
}

// Set stores a value in both L1 and L2
func (c *L1Cache) Set(ctx context.Context, key string, data []byte) error {
	// Store in L1
	c.l1.Set(key, data, int64(len(data)))
	// Schedule expiry
	go c.expireAfter(key, c.ttl)

	// Store in L2 asynchronously
	if c.l2 != nil {
		go func() {
			if err := c.l2.Set(ctx, key, data, c.ttl).Err(); err != nil {
				c.logger.Warn("Failed to set L2 cache",
					zap.String("key", key),
					zap.Error(err))
			}
		}()
	}

	return nil
}

// expireAfter removes a key from cache after the specified duration
func (c *L1Cache) expireAfter(key string, ttl time.Duration) {
	time.Sleep(ttl)
	c.l1.Del(key)
}

// Delete removes a value from both L1 and L2
func (c *L1Cache) Delete(ctx context.Context, key string) error {
	// Delete from L1
	c.l1.Del(key)

	// Delete from L2
	if c.l2 != nil {
		if err := c.l2.Del(ctx, key).Err(); err != nil {
			return fmt.Errorf("L2 delete failed: %w", err)
		}
	}

	return nil
}

// GetOrCompute retrieves a value or computes it using the provided function
// This is a common pattern for lazy loading
func (c *L1Cache) GetOrCompute(ctx context.Context, key string, fn func() ([]byte, error)) ([]byte, error) {
	// Try L1 then L2
	if data, found := c.Get(ctx, key); found {
		return data, nil
	}

	// Compute the value
	data, err := fn()
	if err != nil {
		return nil, err
	}

	// Store in cache
	if err := c.Set(ctx, key, data); err != nil {
		c.logger.Warn("Failed to cache computed value",
			zap.String("key", key),
			zap.Error(err))
	}

	return data, nil
}

// WarmUp preloads the cache with data from the given loader function
func (c *L1Cache) WarmUp(ctx context.Context, keys []string, loader func(string) ([]byte, error)) int {
	success := 0
	for _, key := range keys {
		data, err := loader(key)
		if err != nil {
			c.logger.Warn("Failed to warm up cache key",
				zap.String("key", key),
				zap.Error(err))
			continue
		}
		if err := c.Set(ctx, key, data); err != nil {
			c.logger.Warn("Failed to set cache during warmup",
				zap.String("key", key),
				zap.Error(err))
			continue
		}
		success++
	}
	return success
}

// Clear clears L1 cache
func (c *L1Cache) Clear(ctx context.Context) error {
	c.l1.Clear()
	return nil
}

// Stats returns cache statistics
func (c *L1Cache) Stats() map[string]interface{} {
	c.metricsMu.Lock()
	defer c.metricsMu.Unlock()

	stats := map[string]interface{}{
		"l1_max_cost":    c.l1MaxCost,
		"l1_hits":         c.metrics.L1Hits,
		"l1_misses":       c.metrics.L1Misses,
		"l2_hits":         c.metrics.L2Hits,
		"l2_misses":       c.metrics.L2Misses,
		"hit_rate":        c.calculateHitRate(),
		"ttl_seconds":     c.ttl.Seconds(),
		"l2_available":    c.l2 != nil,
	}

	return stats
}

// calculateHitRate computes the overall cache hit rate
func (c *L1Cache) calculateHitRate() float64 {
	total := c.metrics.L1Hits + c.metrics.L1Misses
	if total == 0 {
		return 0
	}
	return float64(c.metrics.L1Hits) / float64(total)
}

// recordL1Hit records an L1 cache hit
func (c *L1Cache) recordL1Hit() {
	c.metricsMu.Lock()
	c.metrics.L1Hits++
	c.metricsMu.Unlock()
}

// recordL1Miss records an L1 cache miss
func (c *L1Cache) recordL1Miss() {
	c.metricsMu.Lock()
	c.metrics.L1Misses++
	c.metricsMu.Unlock()
}

// recordL2Hit records an L2 cache hit
func (c *L1Cache) recordL2Hit() {
	c.metricsMu.Lock()
	c.metrics.L2Hits++
	c.metricsMu.Unlock()
}

// recordL2Miss records an L2 cache miss
func (c *L1Cache) recordL2Miss() {
	c.metricsMu.Lock()
	c.metrics.L2Misses++
	c.metricsMu.Unlock()
}

// ResetMetrics resets all cache metrics
func (c *L1Cache) ResetMetrics() {
	c.metricsMu.Lock()
	c.metrics = CacheMetrics{}
	c.metricsMu.Unlock()
}

// Close closes the cache and releases resources
func (c *L1Cache) Close() error {
	c.l1.Close()
	return nil
}

// SemanticCache provides specialized caching for semantic similarity results
type SemanticCache struct {
	cache *L1Cache
}

// NewSemanticCache creates a new semantic cache
func NewSemanticCache(l1 *L1Cache) *SemanticCache {
	return &SemanticCache{
		cache: l1,
	}
}

// GetSimilarity retrieves cached similarity results for a query
func (s *SemanticCache) GetSimilarity(ctx context.Context, namespace, query string) (float64, bool) {
	key := fmt.Sprintf("sim:%s:%s", namespace, query)
	val, found := s.cache.Get(ctx, key)
	if !found {
		return 0, false
	}

	// Parse similarity score from string encoding
	strVal := string(val)
	var score float64
	fmt.Sscanf(strVal, "%f", &score)
	return score, true
}

// SetSimilarity stores a similarity score
func (s *SemanticCache) SetSimilarity(ctx context.Context, namespace, query string, similarity float64) {
	key := fmt.Sprintf("sim:%s:%s", namespace, query)
	data := []byte(fmt.Sprintf("%.6f", similarity))
	s.cache.Set(ctx, key, data)
}

// ContextCache provides caching for context strings (chat history, etc.)
type ContextCache struct {
	cache *L1Cache
}

// NewContextCache creates a new context cache
func NewContextCache(l1 *L1Cache) *ContextCache {
	return &ContextCache{
		cache: l1,
	}
}

// GetContext retrieves cached context for a conversation
func (cc *ContextCache) GetContext(ctx context.Context, conversationID string) (string, bool) {
	key := fmt.Sprintf("ctx:%s", conversationID)
	data, found := cc.cache.Get(ctx, key)
	if !found {
		return "", false
	}
	return string(data), true
}

// SetContext stores context for a conversation
func (cc *ContextCache) SetContext(ctx context.Context, conversationID, context string) {
	key := fmt.Sprintf("ctx:%s", conversationID)
	cc.cache.Set(ctx, key, []byte(context))
}
