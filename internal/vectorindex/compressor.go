// Package vectorindex provides neural compression for vector representations
// This is the Go port of ai/vector_index/compressor.py
package vectorindex

import (
	"math"
	"sync"

	"go.uber.org/zap"
)

// CompressionMethod determines how vectors are compressed
type CompressionMethod int

const (
	// CompressionMeanPool uses simple mean pooling
	CompressionMeanPool CompressionMethod = iota
	// CompressionWeightedMean uses cosine-similarity weighted mean pooling
	CompressionWeightedMean
	// CompressionMaxPool uses max pooling (captures salient features)
	CompressionMaxPool
	// CompressionHybrid combines mean and max pooling
	CompressionHybrid
)

// CompressorConfig configures the vector compressor
type CompressorConfig struct {
	InputDim    int               `json:"input_dim"`
	HiddenDim   int               `json:"hidden_dim"`
	Method      CompressionMethod `json:"method"`
	UseLayerNorm bool              `json:"use_layer_norm"`
}

// DefaultCompressorConfig returns default compression configuration
func DefaultCompressorConfig() *CompressorConfig {
	return &CompressorConfig{
		InputDim:     1536,
		HiddenDim:    512,
		Method:       CompressionWeightedMean,
		UseLayerNorm: true,
	}
}

// Compressor compresses a cluster of vectors into a single representative vector
// Replaces the PyTorch neural network with pure Go algorithms
type Compressor struct {
	config *CompressorConfig
	logger *zap.Logger
	// Pre-allocated buffers for thread safety
	bufferPool *sync.Pool
}

// NewCompressor creates a new vector compressor
func NewCompressor(cfg *CompressorConfig, logger *zap.Logger) *Compressor {
	if cfg == nil {
		cfg = DefaultCompressorConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Compressor{
		config: cfg,
		logger: logger,
		bufferPool: &sync.Pool{
			New: func() interface{} {
				return make([]float64, cfg.InputDim)
			},
		},
	}
}

// Compress compresses a cluster of vectors into a single parent vector
// Args:
//   vectors: Slice of vectors to compress, each of length InputDim
// Returns:
//   A single compressed vector of length InputDim
func (c *Compressor) Compress(vectors []Vector) []float64 {
	if len(vectors) == 0 {
		return make([]float64, c.config.InputDim)
	}
	if len(vectors) == 1 {
		result := make([]float64, c.config.InputDim)
		copy(result, vectors[0])
		if c.config.UseLayerNorm {
			result = c.layerNorm(result)
		}
		return normalize(result)
	}

	var compressed []float64

	switch c.config.Method {
	case CompressionMeanPool:
		compressed = c.meanPool(vectors)
	case CompressionWeightedMean:
		compressed = c.weightedMeanPool(vectors)
	case CompressionMaxPool:
		compressed = c.maxPool(vectors)
	case CompressionHybrid:
		compressed = c.hybridPool(vectors)
	default:
		compressed = c.weightedMeanPool(vectors)
	}

	// Apply layer normalization if enabled
	if c.config.UseLayerNorm {
		compressed = c.layerNorm(compressed)
	}

	// Normalize to unit length (crucial for cosine similarity)
	return normalize(compressed)
}

// meanPool performs simple mean pooling
func (c *Compressor) meanPool(vectors []Vector) []float64 {
	dim := c.config.InputDim
	result := c.getBuffer()
	defer c.putBuffer(result)

	for _, vec := range vectors {
		for i := 0; i < dim; i++ {
			result[i] += vec[i]
		}
	}

	count := float64(len(vectors))
	for i := 0; i < dim; i++ {
		result[i] /= count
	}

	// Copy to return value
	output := make([]float64, dim)
	copy(output, result)
	return output
}

// weightedMeanPool performs attention-weighted mean pooling
// Vectors more similar to the cluster centroid get higher weight
func (c *Compressor) weightedMeanPool(vectors []Vector) []float64 {
	dim := c.config.InputDim
	n := len(vectors)

	// First compute the centroid (unweighted mean)
	centroid := c.getBuffer()
	defer c.putBuffer(centroid)

	for _, vec := range vectors {
		for i := 0; i < dim; i++ {
			centroid[i] += vec[i]
		}
	}
	for i := 0; i < dim; i++ {
		centroid[i] /= float64(n)
	}

	// Normalize centroid
	centroid = normalize(centroid)

	// Compute attention weights based on cosine similarity to centroid
	weights := make([]float64, n)
	sumWeights := 0.0

	for i, vec := range vectors {
		// Normalize vector for similarity computation
		normVec := make([]float64, dim)
		copy(normVec, vec)
		normVec = normalize(normVec)

		// Cosine similarity = dot product for normalized vectors
		similarity := 0.0
		for j := 0; j < dim; j++ {
			similarity += normVec[j] * centroid[j]
		}

		// Apply softmax-like transformation
		// Shift to positive, apply mild nonlinearity
		weights[i] = math.Exp(similarity * 2) // Temperature of 0.5
		sumWeights += weights[i]
	}

	// Normalize weights
	if sumWeights > 0 {
		for i := range weights {
			weights[i] /= sumWeights
		}
	} else {
		// Fallback to uniform weights
		for i := range weights {
			weights[i] = 1.0 / float64(n)
		}
	}

	// Compute weighted mean
	result := make([]float64, dim)
	for i, vec := range vectors {
		weight := weights[i]
		for j := 0; j < dim; j++ {
			result[j] += vec[j] * weight
		}
	}

	return result
}

// maxPool performs max pooling (captures salient features)
func (c *Compressor) maxPool(vectors []Vector) []float64 {
	dim := c.config.InputDim
	result := make([]float64, dim)

	// Initialize with first vector
	for i := 0; i < dim; i++ {
		result[i] = vectors[0][i]
	}

	// Take element-wise maximum
	for _, vec := range vectors[1:] {
		for i := 0; i < dim; i++ {
			if vec[i] > result[i] {
				result[i] = vec[i]
			}
		}
	}

	return result
}

// hybridPool combines mean and max pooling
// Takes weighted average of both methods
func (c *Compressor) hybridPool(vectors []Vector) []float64 {
	dim := c.config.InputDim

	mean := c.meanPool(vectors)
	max := c.maxPool(vectors)

	result := make([]float64, dim)
	alpha := 0.7 // Weight for mean pooling

	for i := 0; i < dim; i++ {
		result[i] = alpha*mean[i] + (1-alpha)*max[i]
	}

	return result
}

// layerNorm applies layer normalization to a vector
func (c *Compressor) layerNorm(x []float64) []float64 {
	dim := c.config.InputDim
	result := make([]float64, dim)

	// Compute mean
	mean := 0.0
	for i := 0; i < dim; i++ {
		mean += x[i]
	}
	mean /= float64(dim)

	// Compute variance
	variance := 0.0
	for i := 0; i < dim; i++ {
		diff := x[i] - mean
		variance += diff * diff
	}
	variance /= float64(dim)

	// Avoid division by zero
	epsilon := 1e-5
	stdDev := math.Sqrt(variance + epsilon)

	// Normalize
	for i := 0; i < dim; i++ {
		result[i] = (x[i] - mean) / stdDev
	}

	return result
}


// CompressBatch compresses multiple clusters of vectors
// Returns a slice of compressed vectors
func (c *Compressor) CompressBatch(clusters [][]Vector) [][]float64 {
	results := make([][]float64, len(clusters))

	for i, cluster := range clusters {
		results[i] = c.Compress(cluster)
	}

	return results
}

// getBuffer gets a buffer from the pool
func (c *Compressor) getBuffer() []float64 {
	return c.bufferPool.Get().([]float64)
}

// putBuffer returns a buffer to the pool after clearing it
func (c *Compressor) putBuffer(buf []float64) {
	for i := range buf {
		buf[i] = 0
	}
	c.bufferPool.Put(buf)
}

// Vector represents a single embedding vector
type Vector []float64

// CompressClusters is a convenience function for simple compression
func CompressClusters(clusters [][]Vector, dim int) [][]float64 {
	cfg := &CompressorConfig{
		InputDim:     dim,
		Method:       CompressionWeightedMean,
		UseLayerNorm: true,
	}
	compressor := NewCompressor(cfg, nil)
	return compressor.CompressBatch(clusters)
}

// CompressSingleCluster compresses a single cluster of vectors
func CompressSingleCluster(vectors []Vector, dim int) []float64 {
	cfg := &CompressorConfig{
		InputDim:     dim,
		Method:       CompressionWeightedMean,
		UseLayerNorm: true,
	}
	compressor := NewCompressor(cfg, nil)
	return compressor.Compress(vectors)
}

// AttentionWeights computes attention weights for vectors based on their similarity
// This can be used for more sophisticated compression strategies
func (c *Compressor) AttentionWeights(vectors []Vector, query []float64) []float64 {
	n := len(vectors)
	dim := c.config.InputDim

	if n == 0 {
		return nil
	}

	// Normalize query
	queryNorm := make([]float64, dim)
	copy(queryNorm, query)
	queryNorm = normalize(query)

	// Compute similarities
	weights := make([]float64, n)
	sumWeights := 0.0

	for i, vec := range vectors {
		// Normalize vector
		vecNorm := make([]float64, dim)
		copy(vecNorm, vec)
		vecNorm = normalize(vecNorm)

		// Dot product (cosine similarity for normalized vectors)
		similarity := 0.0
		for j := 0; j < dim; j++ {
			similarity += vecNorm[j] * queryNorm[j]
		}

		// Softmax with temperature
		weights[i] = math.Exp(similarity * 2)
		sumWeights += weights[i]
	}

	// Normalize
	if sumWeights > 0 {
		for i := range weights {
			weights[i] /= sumWeights
		}
	}

	return weights
}

// ComputeSimilarityMatrix computes pairwise similarities between vectors
func (c *Compressor) ComputeSimilarityMatrix(vectors []Vector) [][]float64 {
	n := len(vectors)
	matrix := make([][]float64, n)

	for i := 0; i < n; i++ {
		matrix[i] = make([]float64, n)
		matrix[i][i] = 1.0 // Self-similarity

		for j := i + 1; j < n; j++ {
			sim := cosineSimilarity(vectors[i], vectors[j])
			matrix[i][j] = sim
			matrix[j][i] = sim
		}
	}

	return matrix
}

// GetStats returns statistics about the compressor
func (c *Compressor) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"type":           "vector_compressor",
		"input_dim":      c.config.InputDim,
		"hidden_dim":     c.config.HiddenDim,
		"method":         c.config.Method,
		"use_layer_norm": c.config.UseLayerNorm,
	}
}

// CompressBatchParallel compresses multiple clusters in parallel
// Useful for processing many clusters efficiently
func (c *Compressor) CompressBatchParallel(clusters [][]Vector, workers int) [][]float64 {
	if workers <= 0 {
		workers = 4
	}

	results := make([][]float64, len(clusters))
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, workers)

	for i, cluster := range clusters {
		wg.Add(1)
		sem <- struct{}{} // Acquire

		go func(idx int, cl []Vector) {
			defer wg.Done()
			defer func() { <-sem }() // Release

			compressed := c.Compress(cl)

			mu.Lock()
			results[idx] = compressed
			mu.Unlock()
		}(i, cluster)
	}

	wg.Wait()
	return results
}
