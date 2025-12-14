// Package cache provides near-memory caching for the Memory Kernel.
// It uses Ristretto for high-performance in-process caching.
package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto"
	"go.uber.org/zap"
)

// Config holds cache configuration
type Config struct {
	MaxCost     int64         // Maximum cache size in bytes (default: 64MB)
	NumCounters int64         // Number of counters for cost estimation
	BufferItems int64         // Number of keys per Get buffer
	DefaultTTL  time.Duration // Default time-to-live for cache entries
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		MaxCost:     64 * 1024 * 1024, // 64MB
		NumCounters: 1e7,              // 10 million counters
		BufferItems: 64,               // Standard buffer
		DefaultTTL:  1 * time.Minute,  // 1 minute default TTL
	}
}

// Manager provides near-memory caching with various namespaced caches
type Manager struct {
	cache  *ristretto.Cache
	config Config
	logger *zap.Logger
	mu     sync.RWMutex

	// Metrics
	hits   int64
	misses int64
}

// NewManager creates a new cache manager
func NewManager(cfg Config, logger *zap.Logger) (*Manager, error) {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: cfg.NumCounters,
		MaxCost:     cfg.MaxCost,
		BufferItems: cfg.BufferItems,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ristretto cache: %w", err)
	}

	logger.Info("Near-memory cache initialized",
		zap.Int64("max_cost_bytes", cfg.MaxCost),
		zap.Duration("default_ttl", cfg.DefaultTTL))

	return &Manager{
		cache:  cache,
		config: cfg,
		logger: logger,
	}, nil
}

// Close cleans up the cache
func (m *Manager) Close() {
	m.cache.Close()
}

// Get retrieves a value from cache
func (m *Manager) Get(key string) (interface{}, bool) {
	val, found := m.cache.Get(key)
	m.mu.Lock()
	if found {
		m.hits++
	} else {
		m.misses++
	}
	m.mu.Unlock()
	return val, found
}

// Set stores a value in cache with the default TTL
func (m *Manager) Set(key string, value interface{}, cost int64) bool {
	return m.SetWithTTL(key, value, cost, m.config.DefaultTTL)
}

// SetWithTTL stores a value in cache with a custom TTL
func (m *Manager) SetWithTTL(key string, value interface{}, cost int64, ttl time.Duration) bool {
	return m.cache.SetWithTTL(key, value, cost, ttl)
}

// Delete removes a value from cache
func (m *Manager) Delete(key string) {
	m.cache.Del(key)
}

// Clear removes all values from cache
func (m *Manager) Clear() {
	m.cache.Clear()
}

// Stats returns cache statistics
func (m *Manager) Stats() (hits, misses int64, hitRate float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	hits = m.hits
	misses = m.misses
	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}
	return
}

// UserFactsKey generates a cache key for user facts
func UserFactsKey(namespace, userID string) string {
	return fmt.Sprintf("facts:%s:%s", namespace, userID)
}

// RecentContextKey generates a cache key for recent context
func RecentContextKey(conversationID string) string {
	return fmt.Sprintf("context:%s", conversationID)
}

// QueryResultKey generates a cache key for DGraph query results
func QueryResultKey(query string, vars map[string]string) string {
	// Simple key generation - in production, use proper hashing
	return fmt.Sprintf("query:%s:%v", query, vars)
}

// CachedQuery wraps a DGraph query with caching
type CachedQuery struct {
	manager *Manager
	logger  *zap.Logger
}

// NewCachedQuery creates a new cached query wrapper
func NewCachedQuery(manager *Manager, logger *zap.Logger) *CachedQuery {
	return &CachedQuery{
		manager: manager,
		logger:  logger,
	}
}

// Execute runs a query with cache lookup
// queryFunc is called on cache miss to fetch the actual data
func (cq *CachedQuery) Execute(ctx context.Context, key string, ttl time.Duration, queryFunc func() (interface{}, int64, error)) (interface{}, error) {
	// 1. Check cache
	if val, found := cq.manager.Get(key); found {
		cq.logger.Debug("Cache HIT", zap.String("key", key))
		return val, nil
	}

	cq.logger.Debug("Cache MISS", zap.String("key", key))

	// 2. Cache miss - execute query
	result, cost, err := queryFunc()
	if err != nil {
		return nil, err
	}

	// 3. Store in cache
	cq.manager.SetWithTTL(key, result, cost, ttl)

	return result, nil
}
