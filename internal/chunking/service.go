// Package chunking provides high-performance semantic text chunking
// Optimized for document processing and RAG applications
package chunking

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

// ChunkResult represents a single chunk with metadata
type ChunkResult struct {
	Text        string `json:"text"`
	StartPos    int    `json:"start_pos"`
	EndPos      int    `json:"end_pos"`
	IsComplete  bool   `json:"is_complete"` // True if chunk ends at delimiter
	CharCount   int    `json:"char_count"`
	ByteCount   int    `json:"byte_count"`
}

// Config configures the chunker behavior
type Config struct {
	ChunkSize        int      `json:"chunk_size"`         // Target chunk size in bytes
	Delimiters       []byte   `json:"delimiters"`          // Single-byte delimiters
	Pattern          []byte   `json:"pattern,omitempty"`   // Multi-byte pattern (e.g., SentencePiece ▁)
	PrefixMode       bool     `json:"prefix_mode"`         // Put delimiter at start of next chunk
	Consecutive      bool     `json:"consecutive"`         // Split at START of consecutive runs
	ForwardFallback  bool     `json:"forward_fallback"`    // Search forward if no delimiter in window
	Overlap          int      `json:"overlap,omitempty"`   // Overlap between chunks
	RespectSentence  bool     `json:"respect_sentence"`    // Try to keep sentences together
	MinChunkSize     int      `json:"min_chunk_size"`      // Minimum chunk size
}

// DefaultConfig returns default chunking configuration
func DefaultConfig() *Config {
	return &Config{
		ChunkSize:       4096,
		Delimiters:      []byte{'\n', '.', '?', '!'},
		PrefixMode:      false,
		Consecutive:     false,
		ForwardFallback: true,
		Overlap:         0,
		RespectSentence: true,
		MinChunkSize:    100,
	}
}

// MarkdownConfig returns configuration optimized for Markdown documents
func MarkdownConfig(chunkSize int) *Config {
	return &Config{
		ChunkSize:       chunkSize,
		Delimiters:      []byte{'\n', '#', '`', '-', '*', '>', '\t'},
		PrefixMode:      true,
		Consecutive:     true,
		ForwardFallback: true,
		Overlap:         0,
		MinChunkSize:    100,
	}
}

// SentencePieceConfig returns configuration for SentencePiece tokenized text
func SentencePieceConfig(chunkSize int) *Config {
	// Metaspace character (U+2581) in UTF-8: b'\xe2\x96\x81'
	metaspace := []byte{0xe2, 0x96, 0x81}

	return &Config{
		ChunkSize:       chunkSize,
		Pattern:         metaspace,
		PrefixMode:      true,
		Consecutive:     true,
		ForwardFallback: true,
		MinChunkSize:    50,
	}
}

// Chunker performs semantic text chunking
type Chunker struct {
	config *Config
}

// New creates a new chunker with the given configuration
func New(config *Config) *Chunker {
	if config == nil {
		config = DefaultConfig()
	}
	return &Chunker{config: config}
}

// NewMarkdown creates a chunker optimized for Markdown
func NewMarkdown(chunkSize int) *Chunker {
	return New(MarkdownConfig(chunkSize))
}

// NewSentencePiece creates a chunker for SentencePiece tokenized text
func NewSentencePiece(chunkSize int) *Chunker {
	return New(SentencePieceConfig(chunkSize))
}

// Chunk splits text into semantic chunks
func (c *Chunker) Chunk(text string) []ChunkResult {
	if text == "" {
		return nil
	}

	results := make([]ChunkResult, 0)
	position := 0
	textLen := len(text)
	overlap := c.config.Overlap

	for position < textLen {
		remaining := textLen - position

		// Last chunk - return all remaining
		if remaining <= c.config.ChunkSize {
			results = append(results, ChunkResult{
				Text:       text[position:],
				StartPos:   position,
				EndPos:     textLen,
				IsComplete: true,
				CharCount:  textLen - position,
				ByteCount:  len(text[position:]),
			})
			break
		}

		// Calculate target end position
		targetEnd := position + c.config.ChunkSize

		// Search for delimiter in window
		splitPos := c.findLastDelimiter(text[position:targetEnd])

		if splitPos >= 0 {
			// Found delimiter
			actualPos := position + splitPos
			if !c.config.PrefixMode {
				actualPos++ // Include the delimiter
			}

			// Ensure minimum chunk size
			if actualPos-position < c.config.MinChunkSize && targetEnd < textLen {
				actualPos = targetEnd
			}

			results = append(results, ChunkResult{
				Text:       text[position:actualPos],
				StartPos:   position,
				EndPos:     actualPos,
				IsComplete: true,
				CharCount:  actualPos - position,
				ByteCount:  len(text[position:actualPos]),
			})

			// Apply overlap
			position = actualPos - overlap
			if position < 0 {
				position = 0
			}
		} else if c.config.ForwardFallback {
			// Search forward for delimiter
			forwardWindow := text[targetEnd:]
			forwardPos := c.findFirstDelimiter(forwardWindow)

			if forwardPos >= 0 {
				actualPos := targetEnd + forwardPos
				if !c.config.PrefixMode {
					actualPos++
				}

				results = append(results, ChunkResult{
					Text:       text[position:actualPos],
					StartPos:   position,
					EndPos:     actualPos,
					IsComplete: true,
					CharCount:  actualPos - position,
					ByteCount:  len(text[position:actualPos]),
				})

				position = actualPos - overlap
				if position < 0 {
					position = 0
				}
			} else {
				// No delimiter found, take all remaining
				results = append(results, ChunkResult{
					Text:       text[position:],
					StartPos:   position,
					EndPos:     textLen,
					IsComplete: false,
					CharCount:  textLen - position,
					ByteCount:  len(text[position:]),
				})
				break
			}
		} else {
			// Hard split at target position
			results = append(results, ChunkResult{
				Text:       text[position:targetEnd],
				StartPos:   position,
				EndPos:     targetEnd,
				IsComplete: false,
				CharCount:  targetEnd - position,
				ByteCount:  len(text[position:targetEnd]),
			})

			position = targetEnd - overlap
			if position < 0 {
				position = 0
			}
		}
	}

	return results
}

// ChunkBytes chunks raw byte data
func (c *Chunker) ChunkBytes(data []byte) []ChunkResult {
	if len(data) == 0 {
		return nil
	}

	results := make([]ChunkResult, 0)
	position := 0
	dataLen := len(data)
	overlap := c.config.Overlap

	for position < dataLen {
		remaining := dataLen - position

		if remaining <= c.config.ChunkSize {
			results = append(results, ChunkResult{
				Text:       string(data[position:]),
				StartPos:   position,
				EndPos:     dataLen,
				IsComplete: true,
				CharCount:  utf8.RuneCount(data[position:]),
				ByteCount:  remaining,
			})
			break
		}

		targetEnd := position + c.config.ChunkSize
		splitPos := c.findLastDelimiterInBytes(data[position:targetEnd])

		if splitPos >= 0 {
			actualPos := position + splitPos
			if !c.config.PrefixMode {
				actualPos++
			}

			if actualPos-position < c.config.MinChunkSize && targetEnd < dataLen {
				actualPos = targetEnd
			}

			results = append(results, ChunkResult{
				Text:       string(data[position:actualPos]),
				StartPos:   position,
				EndPos:     actualPos,
				IsComplete: true,
				CharCount:  utf8.RuneCount(data[position:actualPos]),
				ByteCount:  actualPos - position,
			})

			position = actualPos - overlap
			if position < 0 {
				position = 0
			}
		} else if c.config.ForwardFallback {
			forwardWindow := data[targetEnd:]
			forwardPos := c.findFirstDelimiterInBytes(forwardWindow)

			if forwardPos >= 0 {
				actualPos := targetEnd + forwardPos
				if !c.config.PrefixMode {
					actualPos++
				}

				results = append(results, ChunkResult{
					Text:       string(data[position:actualPos]),
					StartPos:   position,
					EndPos:     actualPos,
					IsComplete: true,
					CharCount:  utf8.RuneCount(data[position:actualPos]),
					ByteCount:  actualPos - position,
				})

				position = actualPos - overlap
			} else {
				results = append(results, ChunkResult{
					Text:       string(data[position:]),
					StartPos:   position,
					EndPos:     dataLen,
					IsComplete: false,
					CharCount:  utf8.RuneCount(data[position:]),
					ByteCount:  dataLen - position,
				})
				break
			}
		} else {
			results = append(results, ChunkResult{
				Text:       string(data[position:targetEnd]),
				StartPos:   position,
				EndPos:     targetEnd,
				IsComplete: false,
				CharCount:  utf8.RuneCount(data[position:targetEnd]),
				ByteCount:  targetEnd - position,
			})

			position = targetEnd - overlap
			if position < 0 {
				position = 0
			}
		}
	}

	return results
}

// findLastDelimiter finds the last occurrence of any delimiter in the string
func (c *Chunker) findLastDelimiter(s string) int {
	if len(c.config.Pattern) > 0 {
		// Search for multi-byte pattern
		idx := bytes.LastIndex([]byte(s), c.config.Pattern)
		if idx >= 0 {
			return idx
		}
	}

	// Search for single-byte delimiters
	for i := len(s) - 1; i >= 0; i-- {
		if c.isDelimiter(s[i]) {
			// Check for consecutive delimiters
			if c.config.Consecutive {
				// Find start of consecutive run
				for i > 0 && c.isDelimiter(s[i-1]) {
					i--
				}
			}
			return i
		}
	}

	return -1
}

// findLastDelimiterInBytes finds the last delimiter in byte slice
func (c *Chunker) findLastDelimiterInBytes(data []byte) int {
	if len(c.config.Pattern) > 0 {
		idx := bytes.LastIndex(data, c.config.Pattern)
		if idx >= 0 {
			return idx
		}
	}

	for i := len(data) - 1; i >= 0; i-- {
		if c.isDelimiter(data[i]) {
			if c.config.Consecutive {
				for i > 0 && c.isDelimiter(data[i-1]) {
					i--
				}
			}
			return i
		}
	}

	return -1
}

// findFirstDelimiter finds the first occurrence of any delimiter
func (c *Chunker) findFirstDelimiter(s string) int {
	if len(c.config.Pattern) > 0 {
		idx := bytes.Index([]byte(s), c.config.Pattern)
		if idx >= 0 {
			return idx
		}
	}

	for i, r := range s {
		if c.isDelimiter(byte(r)) {
			if c.config.Consecutive {
				// Skip all consecutive delimiters
				for j := i + 1; j < len(s); j++ {
					if !c.isDelimiter(s[j]) {
						return j
					}
				}
				return len(s)
			}
			return i
		}
	}

	return -1
}

// findFirstDelimiterInBytes finds the first delimiter in byte slice
func (c *Chunker) findFirstDelimiterInBytes(data []byte) int {
	if len(c.config.Pattern) > 0 {
		idx := bytes.Index(data, c.config.Pattern)
		if idx >= 0 {
			return idx
		}
	}

	for i, b := range data {
		if c.isDelimiter(b) {
			if c.config.Consecutive {
				for j := i + 1; j < len(data); j++ {
					if !c.isDelimiter(data[j]) {
						return j
					}
				}
				return len(data)
			}
			return i
		}
	}

	return -1
}

// isDelimiter checks if a byte is a delimiter
func (c *Chunker) isDelimiter(b byte) bool {
	for _, d := range c.config.Delimiters {
		if b == d {
			return true
		}
	}
	return false
}

// ChunkText is a convenience function for simple chunking
func ChunkText(text string, chunkSize int, delimiters string) []string {
	config := &Config{
		ChunkSize:  chunkSize,
		Delimiters: []byte(delimiters),
		PrefixMode: false,
	}

	chunker := New(config)
	results := chunker.Chunk(text)

	chunks := make([]string, len(results))
	for i, r := range results {
		chunks[i] = r.Text
	}

	return chunks
}

// ChunkBySize chunks text into fixed-size chunks without semantic splitting
func ChunkBySize(text string, chunkSize int) []string {
	if text == "" {
		return nil
	}

	runes := []rune(text)
	if len(runes) <= chunkSize {
		return []string{text}
	}

	chunks := make([]string, 0, (len(runes)+chunkSize-1)/chunkSize)
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}

	return chunks
}

// ChunkByLines chunks text by line count
func ChunkByLines(text string, linesPerChunk int) []string {
	if text == "" {
		return nil
	}

	lines := strings.Split(text, "\n")
	if len(lines) <= linesPerChunk {
		return []string{text}
	}

	chunks := make([]string, 0, (len(lines)+linesPerChunk-1)/linesPerChunk)
	for i := 0; i < len(lines); i += linesPerChunk {
		end := i + linesPerChunk
		if end > len(lines) {
			end = len(lines)
		}
		chunks = append(chunks, strings.Join(lines[i:end], "\n"))
	}

	return chunks
}

// ChunkByParagraphs chunks text by paragraphs (double newlines)
func ChunkByParagraphs(text string) []string {
	if text == "" {
		return nil
	}

	// Split by double newlines, trim whitespace from each paragraph
	paragraphs := strings.Split(text, "\n\n")
	chunks := make([]string, 0, len(paragraphs))

	for _, p := range paragraphs {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			chunks = append(chunks, trimmed)
		}
	}

	return chunks
}

// ChunkBySentences chunks text by sentences
func ChunkBySentences(text string) []string {
	if text == "" {
		return nil
	}

	chunks := make([]string, 0)
	current := strings.Builder{}

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		current.WriteRune(runes[i])

		// Check for sentence endings
		if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
			// Check if followed by space or end of string
			if i+1 >= len(runes) || runes[i+1] == ' ' || runes[i+1] == '\n' {
				s := current.String()
				if strings.TrimSpace(s) != "" {
					chunks = append(chunks, strings.TrimSpace(s))
				}
				current.Reset()
			}
		}
	}

	// Add remaining text
	if current.Len() > 0 {
		s := current.String()
		if strings.TrimSpace(s) != "" {
			chunks = append(chunks, strings.TrimSpace(s))
		}
	}

	return chunks
}

// MergeChunks merges small chunks that are below a threshold
func MergeChunks(chunks []ChunkResult, minSize int) []ChunkResult {
	if len(chunks) <= 1 {
		return chunks
	}

	merged := make([]ChunkResult, 0)
	current := strings.Builder{}
	startPos := 0
	currentSize := 0

	for i, chunk := range chunks {
		if currentSize+len(chunk.Text) < minSize && i < len(chunks)-1 {
			// Merge with current
			current.WriteString(chunk.Text)
			currentSize = len(current.String())
			if startPos == 0 {
				startPos = chunk.StartPos
			}
		} else {
			// Flush current if exists
			if current.Len() > 0 {
				merged = append(merged, ChunkResult{
					Text:       current.String(),
					StartPos:   startPos,
					EndPos:     startPos + current.Len(),
					IsComplete: true,
					CharCount:  current.Len(),
					ByteCount:  len(current.String()),
				})
				current.Reset()
				startPos = 0
				currentSize = 0
			}

			// Add current chunk
			merged = append(merged, chunk)
		}
	}

	// Flush remaining
	if current.Len() > 0 {
		merged = append(merged, ChunkResult{
			Text:       current.String(),
			StartPos:   startPos,
			EndPos:     startPos + current.Len(),
			IsComplete: true,
			CharCount:  current.Len(),
			ByteCount:  len(current.String()),
		})
	}

	return merged
}

// GetStats returns statistics about chunking results
func GetStats(chunks []ChunkResult) ChunkStats {
	totalChars := 0
	totalBytes := 0
	completeCount := 0

	for _, c := range chunks {
		totalChars += c.CharCount
		totalBytes += c.ByteCount
		if c.IsComplete {
			completeCount++
		}
	}

	avgSize := 0.0
	if len(chunks) > 0 {
		avgSize = float64(totalChars) / float64(len(chunks))
	}

	return ChunkStats{
		TotalChunks:   len(chunks),
		TotalChars:    totalChars,
		TotalBytes:    totalBytes,
		AvgCharCount:  avgSize,
		CompleteCount: completeCount,
	}
}

// ChunkStats holds statistics about chunking results
type ChunkStats struct {
	TotalChunks   int     `json:"total_chunks"`
	TotalChars    int     `json:"total_chars"`
	TotalBytes    int     `json:"total_bytes"`
	AvgCharCount  float64 `json:"avg_char_count"`
	CompleteCount int     `json:"complete_count"`
}
