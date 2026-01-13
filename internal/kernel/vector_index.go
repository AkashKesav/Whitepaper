// Package kernel provides the vector index for Hybrid RAG.
// Uses Qdrant as the dedicated vector database for semantic search.
package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
)

const (
	// DefaultCollectionName is the default Qdrant collection for node embeddings
	DefaultCollectionName = "rmk_nodes"
	// CacheCollectionName is the Qdrant collection for semantic cache
	CacheCollectionName = "rmk_cache"
	// EmbeddingDimension is the dimension of Ollama nomic-embed-text embeddings
	EmbeddingDimension = 768
)

// Input validation limits for vector operations
const (
	MaxEmbeddingTextLength = 8000 // Maximum text length for embedding generation
	MaxSearchQueryLength   = 500  // Maximum search query length
)

// isValidNamespace validates that a namespace follows the expected format
// SECURITY: Prevents namespace injection and bypass attacks
// Valid formats: user_<alphanumeric> or group_<UUID>
func isValidNamespace(ns string) bool {
	if ns == "" {
		return false
	}
	// Allow: user_<alphanumeric with optional hyphens/underscores>
	// Allow: group_<UUID format or alphanumeric with hyphens/underscores>
	matched, _ := regexp.MatchString(`^(user|group)_[a-zA-Z0-9_-]+$`, ns)
	return matched
}

// VectorIndex manages vector embeddings using Qdrant
type VectorIndex struct {
	baseURL        string
	httpClient     *http.Client
	dimension      int
	collectionName string
	logger         *zap.Logger
	initialized    bool
	rateLimiter    RateLimiter // Optional rate limiter for vector search
}

// RateLimiter defines the interface for rate limiting vector search operations
type RateLimiter interface {
	// Allow checks if a request is allowed under the rate limit
	Allow(ctx context.Context, userID string) (bool, *RateLimitResult)
}

// RateLimitResult contains the result of a rate limit check
type RateLimitResult struct {
	Allowed    bool
	Remaining  int
	ResetAt    time.Time
	RetryAfter  time.Duration
}

// NewVectorIndex creates a new Qdrant-backed vector index
func NewVectorIndex(qdrantURL, collectionName string, logger *zap.Logger) *VectorIndex {
	if qdrantURL == "" {
		qdrantURL = os.Getenv("QDRANT_URL")
		if qdrantURL == "" {
			qdrantURL = "http://localhost:6333"
		}
	}

	if collectionName == "" {
		collectionName = DefaultCollectionName
	}

	return &VectorIndex{
		baseURL:        qdrantURL,
		collectionName: collectionName,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		dimension: EmbeddingDimension,
		logger:    logger,
	}
}

// Initialize creates the collection if it doesn't exist
func (vi *VectorIndex) Initialize(ctx context.Context) error {
	if vi.initialized {
		return nil
	}

	// Check if collection exists
	resp, err := vi.httpClient.Get(vi.baseURL + "/collections/" + vi.collectionName)
	if err != nil {
		return fmt.Errorf("failed to check collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		vi.initialized = true
		vi.logger.Info("Qdrant collection already exists", zap.String("collection", vi.collectionName))
		return nil
	}

	// Create collection
	createReq := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     vi.dimension,
			"distance": "Cosine",
		},
	}

	jsonData, err := json.Marshal(createReq)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT",
		vi.baseURL+"/collections/"+vi.collectionName,
		bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err = vi.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create collection (status %d): %s", resp.StatusCode, string(body))
	}

	vi.initialized = true
	vi.logger.Info("Created Qdrant collection", zap.String("collection", vi.collectionName))
	return nil
}

// Store saves a node's embedding to Qdrant with metadata
// The point ID is a hash of namespace+uid for uniqueness
func (vi *VectorIndex) Store(ctx context.Context, namespace, uid string, embedding []float32, metadata map[string]interface{}) error {
	if err := vi.Initialize(ctx); err != nil {
		return err
	}

	pointID := hashToInt(namespace + ":" + uid)

	upsertReq := map[string]interface{}{
		"points": []map[string]interface{}{
			{
				"id":     pointID,
				"vector": embedding,
				"payload": mergeMaps(map[string]interface{}{
					"namespace": namespace,
					"uid":       uid,
				}, metadata),
			},
		},
	}

	jsonData, err := json.Marshal(upsertReq)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT",
		vi.baseURL+"/collections/"+vi.collectionName+"/points",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := vi.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to store vector: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to store vector (status %d): %s", resp.StatusCode, string(body))
	}

	vi.logger.Debug("Stored vector in Qdrant",
		zap.String("namespace", namespace),
		zap.String("uid", uid))

	return nil
}

// Update updates an existing vector in the index
// Qdrant doesn't support direct updates, so we delete and re-insert
func (vi *VectorIndex) Update(ctx context.Context, namespace, uid string, embedding []float32, metadata map[string]interface{}) error {
	if err := vi.Initialize(ctx); err != nil {
		return err
	}

	pointID := hashToInt(namespace + ":" + uid)

	// First delete the old vector
	deleteReq := map[string]interface{}{
		"points": []int64{pointID},
	}

	deleteJSON, err := json.Marshal(deleteReq)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		vi.baseURL+"/collections/"+vi.collectionName+"/points/delete",
		bytes.NewBuffer(deleteJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := vi.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete vector for update: %w", err)
	}
	resp.Body.Close()

	// Then store the new vector
	return vi.Store(ctx, namespace, uid, embedding, metadata)
}

// Search finds top-K similar nodes by vector similarity
// Returns UIDs, scores, and payloads of matching nodes
// SECURITY: Supports rate limiting to prevent abuse
func (vi *VectorIndex) Search(ctx context.Context, namespace, userID string, queryVec []float32, topK int) ([]string, []float32, []map[string]interface{}, error) {
	// SECURITY: Reject empty namespace before any processing
	// This prevents namespace bypass attacks that could return results from all namespaces
	if namespace == "" {
		vi.logger.Warn("Vector search rejected: empty namespace",
			zap.String("user", userID))
		return nil, nil, nil, fmt.Errorf("namespace cannot be empty")
	}

	// SECURITY: Validate namespace format to prevent injection attacks
	if !isValidNamespace(namespace) {
		vi.logger.Warn("Vector search rejected: invalid namespace format",
			zap.String("namespace", namespace),
			zap.String("user", userID))
		return nil, nil, nil, fmt.Errorf("invalid namespace format")
	}

	// SECURITY: Check rate limit if limiter is configured
	if vi.rateLimiter != nil && userID != "" {
		allowed, result := vi.rateLimiter.Allow(ctx, userID)
		if !allowed {
			vi.logger.Warn("Vector search rate limit exceeded",
				zap.String("user", userID),
				zap.Duration("retry_after", result.RetryAfter))
			return nil, nil, nil, fmt.Errorf("rate limit exceeded: retry after %v", result.RetryAfter)
		}
	}

	if err := vi.Initialize(ctx); err != nil {
		return nil, nil, nil, err
	}

	searchReq := map[string]interface{}{
		"vector":       queryVec,
		"limit":        topK,
		"with_payload": true,
		"filter": map[string]interface{}{
			"must": []map[string]interface{}{
				{
					"key":   "namespace",
					"match": map[string]interface{}{"value": namespace},
				},
			},
		},
	}

	jsonData, err := json.Marshal(searchReq)
	if err != nil {
		return nil, nil, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		vi.baseURL+"/collections/"+vi.collectionName+"/points/search",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := vi.httpClient.Do(req)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("vector search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, nil, fmt.Errorf("vector search failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Result []struct {
			ID      interface{}            `json:"id"`
			Score   float32                `json:"score"`
			Payload map[string]interface{} `json:"payload"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	uids := make([]string, 0, len(result.Result))
	scores := make([]float32, 0, len(result.Result))
	payloads := make([]map[string]interface{}, 0, len(result.Result))

	for _, hit := range result.Result {
		if uid, ok := hit.Payload["uid"].(string); ok {
			uids = append(uids, uid)
			scores = append(scores, hit.Score)
			payloads = append(payloads, hit.Payload)
		}
	}

	vi.logger.Debug("Vector search completed",
		zap.String("namespace", namespace),
		zap.Int("results", len(uids)))

	return uids, scores, payloads, nil
}

// mergeMaps merges two maps
func mergeMaps(base, extra map[string]interface{}) map[string]interface{} {
	if extra == nil {
		return base
	}
	for k, v := range extra {
		base[k] = v
	}
	return base
}

// Delete removes a node's embedding from Qdrant
func (vi *VectorIndex) Delete(ctx context.Context, namespace, uid string) error {
	if err := vi.Initialize(ctx); err != nil {
		return err
	}

	pointID := hashToInt(namespace + ":" + uid)

	deleteReq := map[string]interface{}{
		"points": []int64{pointID},
	}

	jsonData, err := json.Marshal(deleteReq)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		vi.baseURL+"/collections/"+vi.collectionName+"/points/delete",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := vi.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete vector: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// Stats returns vector index statistics
func (vi *VectorIndex) Stats(ctx context.Context) (map[string]interface{}, error) {
	resp, err := vi.httpClient.Get(vi.baseURL + "/collections/" + vi.collectionName)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// hashToInt creates a deterministic int64 hash from a string
// Used for Qdrant point IDs
func hashToInt(s string) int64 {
	var h int64 = 0
	for _, c := range s {
		h = 31*h + int64(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}

// SanitizeText sanitizes text before embedding generation
// SECURITY: Removes null bytes, collapses whitespace, truncates to max length
func (vi *VectorIndex) SanitizeText(text string) string {
	// Remove null bytes (injection prevention)
	text = strings.ReplaceAll(text, "\x00", "")

	// Collapse multiple whitespace into single space
	text = strings.Join(strings.Fields(text), " ")

	// Truncate to max length if needed
	if len(text) > MaxEmbeddingTextLength {
		vi.logger.Warn("Text truncated for embedding",
			zap.Int("original_length", len(text)),
			zap.Int("truncated_to", MaxEmbeddingTextLength))
		text = text[:MaxEmbeddingTextLength]
	}

	return strings.TrimSpace(text)
}

// ValidateSearchQuery validates a search query before vector search
// SECURITY: Prevents injection and DoS via malformed queries
func (vi *VectorIndex) ValidateSearchQuery(query string) error {
	// Check length
	if len(query) > MaxSearchQueryLength {
		return fmt.Errorf("search query exceeds maximum length of %d characters", MaxSearchQueryLength)
	}

	// Check empty
	if len(strings.TrimSpace(query)) == 0 {
		return fmt.Errorf("search query cannot be empty")
	}

	// Check for null bytes
	if strings.Contains(query, "\x00") {
		return fmt.Errorf("search query contains invalid characters")
	}

	// Check for common injection patterns
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
			vi.logger.Warn("Suspicious search query pattern detected",
				zap.String("pattern", pattern))
			return fmt.Errorf("search query contains suspicious content pattern")
		}
	}

	return nil
}

// SetRateLimiter sets the rate limiter for vector search operations
// SECURITY: Helps prevent abuse of expensive vector search operations
func (vi *VectorIndex) SetRateLimiter(limiter RateLimiter) {
	vi.rateLimiter = limiter
	vi.logger.Info("Vector index rate limiter configured")
}
