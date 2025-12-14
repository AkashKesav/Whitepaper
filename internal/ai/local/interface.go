package local

// LocalEmbedder is the interface for local embedding generation
type LocalEmbedder interface {
	// Embed generates an embedding vector for the given text
	Embed(text string) ([]float32, error)
	// Close cleans up resources
	Close() error
}
