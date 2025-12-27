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

// VectorIndex manages vector embeddings using Qdrant
type VectorIndex struct {
	baseURL        string
	httpClient     *http.Client
	dimension      int
	collectionName string
	logger         *zap.Logger
	initialized    bool
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

// Search finds top-K similar nodes by vector similarity
// Returns UIDs, scores, and payloads of matching nodes
func (vi *VectorIndex) Search(ctx context.Context, namespace string, queryVec []float32, topK int) ([]string, []float32, []map[string]interface{}, error) {
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
