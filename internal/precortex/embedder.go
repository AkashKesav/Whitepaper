package precortex

import (
	"math"
	"strings"

	"go.uber.org/zap"
)

// Embedder interface for generating text embeddings
type Embedder interface {
	Embed(text string) ([]float32, error)
	Close()
}

// ONNXEmbedder provides local text embeddings using ONNX Runtime
// Note: Full ONNX implementation requires model files and proper tensor setup.
// This implementation provides a hash-based fallback that ensures deterministic
// embeddings for cache lookups while maintaining the interface for future ONNX support.
type ONNXEmbedder struct {
	tokenizer *SimpleTokenizer
	logger    *zap.Logger
	maxTokens int
	embedDim  int
	useONNX   bool // Flag for future ONNX integration
}

// EmbedderConfig holds ONNX embedder configuration
type EmbedderConfig struct {
	ModelPath    string // Path to .onnx model file
	TokenizerDir string // Path to tokenizer directory
	MaxTokens    int    // Maximum sequence length
	EmbedDim     int    // Embedding dimension
}

// DefaultEmbedderConfig returns sensible defaults
func DefaultEmbedderConfig() EmbedderConfig {
	return EmbedderConfig{
		ModelPath:    "models/all-MiniLM-L6-v2.onnx",
		TokenizerDir: "models/tokenizer",
		MaxTokens:    128,
		EmbedDim:     384, // MiniLM-L6-v2 has 384 dimensions
	}
}

// NewONNXEmbedder creates a new ONNX-based embedder
func NewONNXEmbedder(cfg EmbedderConfig, logger *zap.Logger) (*ONNXEmbedder, error) {
	// Load tokenizer
	tokenizer, err := NewSimpleTokenizer(cfg.TokenizerDir)
	if err != nil {
		logger.Info("Using fallback tokenizer", zap.Error(err))
		tokenizer = NewFallbackTokenizer()
	}

	embedder := &ONNXEmbedder{
		tokenizer: tokenizer,
		logger:    logger,
		maxTokens: cfg.MaxTokens,
		embedDim:  cfg.EmbedDim,
		useONNX:   false, // Default to hash-based embedding
	}

	logger.Info("Embedder initialized (deterministic hash-based)",
		zap.Int("embed_dim", cfg.EmbedDim),
		zap.Bool("onnx_enabled", embedder.useONNX))

	return embedder, nil
}

// Embed converts text to a vector embedding
// Uses deterministic hashing when ONNX is not available
func (e *ONNXEmbedder) Embed(text string) ([]float32, error) {
	if e.useONNX {
		return e.onnxEmbed(text)
	}
	return e.hashEmbed(text), nil
}

// onnxEmbed performs actual ONNX inference (placeholder for future implementation)
func (e *ONNXEmbedder) onnxEmbed(text string) ([]float32, error) {
	// TODO: Implement full ONNX inference when model is available
	// This requires:
	// 1. Loading the model file
	// 2. Creating input tensors (input_ids, attention_mask, token_type_ids)
	// 3. Running inference
	// 4. Extracting output embeddings
	return e.hashEmbed(text), nil
}

// hashEmbed creates a deterministic hash-based embedding
// This is suitable for exact-match caching where the same text always produces the same vector
func (e *ONNXEmbedder) hashEmbed(text string) []float32 {
	vec := make([]float32, e.embedDim)

	// Normalize text
	text = strings.ToLower(strings.TrimSpace(text))

	// Tokenize for better distribution
	tokens := e.tokenizer.Tokenize(text)

	// Create a deterministic pseudo-embedding based on token hashes
	// This ensures the same text always produces the same vector
	for i, token := range tokens {
		// Distribute token values across the embedding dimensions
		for j := 0; j < 3 && j < e.embedDim; j++ {
			idx := (token + i*31 + j*17) % e.embedDim
			if idx < 0 {
				idx = -idx
			}
			vec[idx] += float32(token%256) / 256.0
		}
	}

	// Add position-based features for word order sensitivity
	for i, char := range text {
		idx := (int(char)*7 + i*11) % e.embedDim
		if idx < 0 {
			idx = -idx
		}
		vec[idx] += float32(char) / 512.0
	}

	// L2 normalize
	vec = normalizeVector(vec)

	return vec
}

// normalizeVector performs L2 normalization
func normalizeVector(vec []float32) []float32 {
	var norm float32
	for _, v := range vec {
		norm += v * v
	}
	if norm > 0 {
		invNorm := float32(1.0 / math.Sqrt(float64(norm)))
		for i := range vec {
			vec[i] *= invNorm
		}
	}
	return vec
}

// Close releases embedder resources
func (e *ONNXEmbedder) Close() {
	// No resources to release in fallback mode
	e.logger.Debug("Embedder closed")
}

// CosineSimilarity computes the cosine similarity between two vectors
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
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
