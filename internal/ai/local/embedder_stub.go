//go:build !cgo

package local

import (
	"fmt"
)

// Embedder stub for non-CGO environments
type Embedder struct {
	// No fields needed for stub
}

// InitRuntime stub
func InitRuntime() error {
	return nil
}

// NewEmbedder stub
func NewEmbedder(modelPath, tokenPath string) (*Embedder, error) {
	fmt.Println("WARNING: CGO/GCC is not available. Using Embedder STUB (Mock Vectors).")
	return &Embedder{}, nil
}

// Embed generates a mock vector
func (e *Embedder) Embed(text string) ([]float32, error) {
	// Return a zero vector of size 384
	vec := make([]float32, 384)
	return vec, nil
}

// Close stub
func (e *Embedder) Close() error {
	return nil
}
