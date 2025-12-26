// Package memory provides in-memory storage for the Hot Path.
// This enables instant retrieval of recent conversation context.
package memory

import (
	"sync"
	"time"

	"github.com/reflective-memory-kernel/internal/embedding"
	"go.uber.org/zap"
)

const (
	// MaxMessagesPerUser is the ring buffer size for each user
	MaxMessagesPerUser = 50
	// DefaultTTL is how long messages stay in the cache
	DefaultTTL = 24 * time.Hour
)

// CachedMessage represents a message with its embedding vector.
type CachedMessage struct {
	UserID      string    `json:"user_id"`
	Query       string    `json:"query"`
	Response    string    `json:"response"`
	Embedding   []float32 `json:"embedding"`
	Timestamp   time.Time `json:"timestamp"`
	ConvID      string    `json:"conversation_id"`
}

// SearchResult represents a similarity search result.
type SearchResult struct {
	Message    CachedMessage `json:"message"`
	Similarity float32       `json:"similarity"`
}

// HotCache provides in-memory storage for recent messages with embeddings.
// It uses a ring buffer per user for O(1) insertion and O(n) search.
type HotCache struct {
	// userMessages maps userID -> ring buffer of messages
	userMessages map[string]*ringBuffer
	embedService *embedding.Service
	logger       *zap.Logger
	mu           sync.RWMutex
}

// ringBuffer is a fixed-size circular buffer for messages.
type ringBuffer struct {
	messages []CachedMessage
	head     int
	size     int
	capacity int
}

// newRingBuffer creates a new ring buffer with the given capacity.
func newRingBuffer(capacity int) *ringBuffer {
	return &ringBuffer{
		messages: make([]CachedMessage, capacity),
		capacity: capacity,
	}
}

// push adds a message to the buffer, overwriting oldest if full.
func (rb *ringBuffer) push(msg CachedMessage) {
	rb.messages[rb.head] = msg
	rb.head = (rb.head + 1) % rb.capacity
	if rb.size < rb.capacity {
		rb.size++
	}
}

// all returns all messages in the buffer (newest first).
func (rb *ringBuffer) all() []CachedMessage {
	result := make([]CachedMessage, rb.size)
	for i := 0; i < rb.size; i++ {
		// Work backwards from head to get newest first
		idx := (rb.head - 1 - i + rb.capacity) % rb.capacity
		result[i] = rb.messages[idx]
	}
	return result
}

// NewHotCache creates a new hot cache with the given embedding service.
func NewHotCache(embedService *embedding.Service, logger *zap.Logger) *HotCache {
	return &HotCache{
		userMessages: make(map[string]*ringBuffer),
		embedService: embedService,
		logger:       logger,
	}
}

// Store adds a message to the cache with its embedding.
// This is called after each conversation turn.
func (hc *HotCache) Store(userID, query, response, convID string) error {
	startTime := time.Now()

	// Generate embedding for the query
	emb, err := hc.embedService.Embed(query)
	if err != nil {
		hc.logger.Warn("Failed to generate embedding for hot cache", zap.Error(err))
		// Continue without embedding - message still stored
		emb = nil
	}

	msg := CachedMessage{
		UserID:    userID,
		Query:     query,
		Response:  response,
		Embedding: emb,
		Timestamp: time.Now(),
		ConvID:    convID,
	}

	hc.mu.Lock()
	defer hc.mu.Unlock()

	// Get or create ring buffer for user
	rb, ok := hc.userMessages[userID]
	if !ok {
		rb = newRingBuffer(MaxMessagesPerUser)
		hc.userMessages[userID] = rb
	}

	rb.push(msg)

	hc.logger.Debug("Stored message in hot cache",
		zap.String("user_id", userID),
		zap.Int("buffer_size", rb.size),
		zap.Duration("embed_time", time.Since(startTime)))

	return nil
}

// Search finds the most similar messages to the query.
// Returns up to topK results with similarity >= threshold.
func (hc *HotCache) Search(userID, query string, topK int, threshold float32) ([]SearchResult, error) {
	startTime := time.Now()

	// Generate embedding for query
	queryEmb, err := hc.embedService.Embed(query)
	if err != nil {
		return nil, err
	}

	hc.mu.RLock()
	rb, ok := hc.userMessages[userID]
	if !ok {
		hc.mu.RUnlock()
		return nil, nil // No messages for this user
	}
	messages := rb.all()
	hc.mu.RUnlock()

	// Calculate similarities
	var results []SearchResult
	for _, msg := range messages {
		if msg.Embedding == nil {
			continue
		}

		sim := embedding.CosineSimilarity(queryEmb, msg.Embedding)
		if sim >= threshold {
			results = append(results, SearchResult{
				Message:    msg,
				Similarity: sim,
			})
		}
	}

	// Sort by similarity (descending)
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Similarity > results[i].Similarity {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Limit to topK
	if len(results) > topK {
		results = results[:topK]
	}

	hc.logger.Debug("Hot cache search completed",
		zap.String("user_id", userID),
		zap.Int("candidates", len(messages)),
		zap.Int("results", len(results)),
		zap.Duration("search_time", time.Since(startTime)))

	return results, nil
}

// GetRecent returns the N most recent messages for a user (no similarity filtering).
func (hc *HotCache) GetRecent(userID string, n int) []CachedMessage {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	rb, ok := hc.userMessages[userID]
	if !ok {
		return nil
	}

	messages := rb.all()
	if len(messages) > n {
		messages = messages[:n]
	}

	return messages
}

// Stats returns cache statistics.
func (hc *HotCache) Stats() map[string]interface{} {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	totalMessages := 0
	for _, rb := range hc.userMessages {
		totalMessages += rb.size
	}

	return map[string]interface{}{
		"total_users":    len(hc.userMessages),
		"total_messages": totalMessages,
	}
}
