package local

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"time"
)

// OllamaEmbedder generates embeddings using Ollama's embedding API
type OllamaEmbedder struct {
	baseURL    string
	model      string
	httpClient *http.Client
	dimension  int
}

// OllamaEmbeddingRequest is the request payload for Ollama embeddings
type OllamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// OllamaEmbeddingResponse is the response from Ollama embeddings API
type OllamaEmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

// NewOllamaEmbedder creates a new Ollama-based embedder
func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder {
	if baseURL == "" {
		baseURL = os.Getenv("OLLAMA_URL")
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
	}
	if model == "" {
		model = "nomic-embed-text" // Default embedding model
	}

	return &OllamaEmbedder{
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		dimension: 768, // nomic-embed-text dimension
	}
}

// Embed generates an embedding vector for the given text
func (e *OllamaEmbedder) Embed(text string) ([]float32, error) {
	reqBody := OllamaEmbeddingRequest{
		Model:  e.model,
		Prompt: text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", e.baseURL+"/api/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result OllamaEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}

	// Convert float64 to float32 and normalize
	embedding := make([]float32, len(result.Embedding))
	var sumSq float64
	for i, v := range result.Embedding {
		embedding[i] = float32(v)
		sumSq += v * v
	}

	// L2 normalize
	norm := float32(math.Sqrt(sumSq))
	if norm > 1e-9 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding, nil
}

// EnsureModel pulls the embedding model if not already present
func (e *OllamaEmbedder) EnsureModel() error {
	// Check if model exists
	resp, err := e.httpClient.Get(e.baseURL + "/api/tags")
	if err != nil {
		return fmt.Errorf("failed to check models: %w", err)
	}
	defer resp.Body.Close()

	var tagsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return fmt.Errorf("failed to decode tags: %w", err)
	}

	// Check if our model is already pulled
	for _, m := range tagsResp.Models {
		if m.Name == e.model || m.Name == e.model+":latest" {
			return nil // Model already exists
		}
	}

	// Pull the model
	pullReq := struct {
		Name string `json:"name"`
	}{Name: e.model}

	jsonData, _ := json.Marshal(pullReq)
	pullResp, err := e.httpClient.Post(e.baseURL+"/api/pull", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to pull model: %w", err)
	}
	defer pullResp.Body.Close()

	if pullResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(pullResp.Body)
		return fmt.Errorf("failed to pull model (status %d): %s", pullResp.StatusCode, string(body))
	}

	// Read the streaming response to completion
	io.Copy(io.Discard, pullResp.Body)

	return nil
}

// Close cleans up resources (no-op for HTTP client)
func (e *OllamaEmbedder) Close() error {
	return nil
}

// Dimension returns the embedding dimension
func (e *OllamaEmbedder) Dimension() int {
	return e.dimension
}
