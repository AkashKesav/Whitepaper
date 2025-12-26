// Package embedding provides text embedding for semantic similarity search.
// Currently uses HTTP calls to AI services; can be upgraded to local ONNX later.
package embedding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Service provides text-to-vector embedding.
// Uses existing AI services for embeddings (can be upgraded to local ONNX later).
type Service struct {
	aiServicesURL string
	client        *http.Client
	logger        *zap.Logger
	cache         map[string][]float32 // Simple cache for repeated queries
	cacheMu       sync.RWMutex
}

// New creates a new embedding service.
func New(aiServicesURL string, logger *zap.Logger) *Service {
	return &Service{
		aiServicesURL: aiServicesURL,
		client:        &http.Client{Timeout: 10 * time.Second},
		logger:        logger,
		cache:         make(map[string][]float32),
	}
}

// embedRequest is the request format for the AI service.
type embedRequest struct {
	Text string `json:"text"`
}

// embedResponse is the response format from the AI service.
type embedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// Embed generates an embedding vector for the given text.
func (s *Service) Embed(text string) ([]float32, error) {
	// Check cache first
	s.cacheMu.RLock()
	if emb, ok := s.cache[text]; ok {
		s.cacheMu.RUnlock()
		return emb, nil
	}
	s.cacheMu.RUnlock()

	// Call AI service
	reqBody, err := json.Marshal(embedRequest{Text: text})
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Post(
		s.aiServicesURL+"/embed",
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		s.logger.Warn("Embedding service unavailable, falling back to nil", zap.Error(err))
		return nil, nil // Don't fail - just skip embedding
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding service returned %d", resp.StatusCode)
	}

	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Cache the result
	s.cacheMu.Lock()
	s.cache[text] = result.Embedding
	// Keep cache size bounded
	if len(s.cache) > 1000 {
		// Simple eviction: clear half the cache
		count := 0
		for k := range s.cache {
			if count > 500 {
				break
			}
			delete(s.cache, k)
			count++
		}
	}
	s.cacheMu.Unlock()

	return result.Embedding, nil
}

// EmbedBatch generates embeddings for multiple texts.
func (s *Service) EmbedBatch(texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := s.Embed(text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

// CosineSimilarity calculates the cosine similarity between two vectors.
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt(normA) * sqrt(normB))
}

// sqrt is a simple square root for float32.
func sqrt(x float32) float32 {
	if x <= 0 {
		return 0
	}
	z := x / 2
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// Close releases resources.
func (s *Service) Close() {
	s.cacheMu.Lock()
	s.cache = make(map[string][]float32)
	s.cacheMu.Unlock()
}
