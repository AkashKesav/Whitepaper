// Package ingester provides efficient document-to-memory pipeline with tiered extraction
// Implements cost-optimized extraction: Rules (FREE) → Embeddings (CHEAP) → LLM (EXPENSIVE)
package ingester

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/reflective-memory-kernel/internal/ai/router"
	"github.com/reflective-memory-kernel/internal/chunking"
	"github.com/reflective-memory-kernel/internal/validation"
	"github.com/reflective-memory-kernel/internal/vectorindex"
	"go.uber.org/zap"
)

// Entity represents an entity extracted from a document
type Entity struct {
	Name        string  `json:"name"`
	EntityType  string  `json:"entity_type"`
	Description string  `json:"description,omitempty"`
	Confidence  float64 `json:"confidence"`
	Source      string  `json:"source"` // "rule", "llm", "vision"
	Count       int     `json:"count,omitempty"`
}

// Relationship represents a relationship between entities
type Relationship struct {
	FromEntity   string  `json:"from_entity"`
	ToEntity     string  `json:"to_entity"`
	RelationType string  `json:"relation_type"`
	Confidence   float64 `json:"confidence"`
}

// Chunk represents a document chunk with metadata
type Chunk struct {
	Text           string    `json:"text"`
	PageNumber     int       `json:"page_number"`
	ChunkIndex     int       `json:"chunk_index"`
	Embedding      []float64 `json:"embedding,omitempty"`
	IsClusterRep   bool      `json:"is_cluster_rep"`
	CharCount      int       `json:"char_count"`
	WordCount      int       `json:"word_count"`
}

// ExtractedImage represents an image extracted from a document
type ExtractedImage struct {
	ImageBase64     string  `json:"image_base64"`
	PageNumber      int     `json:"page_number"`
	ImageIndex      int     `json:"image_index"`
	ComplexityScore float64 `json:"complexity_score"`
	Caption         string  `json:"caption,omitempty"`
}

// IngestionResult represents the result of document ingestion
type IngestionResult struct {
	Entities      []Entity         `json:"entities"`
	Relationships []Relationship  `json:"relationships"`
	Chunks       []Chunk          `json:"chunks"`
	Images       []ExtractedImage  `json:"images"`
	Summary       string           `json:"summary"`
	Stats        IngestStats      `json:"stats"`
	VectorTree    map[string]*vectorindex.VectorNode `json:"vector_tree,omitempty"`
	ProcessedAt   time.Time         `json:"processed_at"`
	Filename     string           `json:"filename,omitempty"`
}

// IngestStats holds statistics about the ingestion process
type IngestStats struct {
	Pages           int     `json:"pages"`
	TotalChunks     int     `json:"chunks"`
	ClusterReps     int     `json:"cluster_reps"`
	Images          int     `json:"images"`
	Tier1Entities   int     `json:"tier1_entities"`
	LLMCalls        int     `json:"llm_calls"`
	VisionCalls     int     `json:"vision_calls"`
	TotalEntities    int     `json:"total_entities"`
	ProcessingTimeMs int64  `json:"processing_time_ms"`
	ExtractedChars   int     `json:"extracted_chars"`
}

// IngestRequest represents a document ingestion request
type IngestRequest struct {
	ContentBase64 string `json:"content_base64"`
	Text          string `json:"text"`
	DocumentType  string `json:"document_type"` // "text", "pdf"
	Filename      string `json:"filename,omitempty"`
}

// Config configures the ingester
type Config struct {
	ChunkSize      int
	ChunkOverlap   int
	MaxLLMCalls    int
	MaxVisionCalls int
	LLMProvider    router.Provider
}

// DefaultConfig returns default ingester configuration
func DefaultConfig() *Config {
	return &Config{
		ChunkSize:      512,
		ChunkOverlap:   50,
		MaxLLMCalls:    10,
		MaxVisionCalls: 5,
		LLMProvider:    router.ProviderNVIDIA,
	}
}

// Service handles document ingestion
type Service struct {
	config       *Config
	router       *router.Router
	chunker      *chunking.Chunker
	vectorIndex  *vectorindex.IndexBuilder
	validator    *validation.FileValidator
	logger       *zap.Logger
}

// New creates a new document ingestion service
func New(cfg *Config, router *router.Router, logger *zap.Logger) *Service {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	// Create chunker
	chunkerConfig := &chunking.Config{
		ChunkSize:       cfg.ChunkSize,
		Overlap:         cfg.ChunkOverlap,
		MinChunkSize:    50,
		ForwardFallback:  true,
		RespectSentence:  true,
	}

	return &Service{
		config:      cfg,
		router:     router,
		chunker:     chunking.New(chunkerConfig),
		vectorIndex: vectorindex.NewIndexBuilder(10, 1536, logger),
		validator:   validation.DefaultConfig(),
		logger:     logger,
	}
}

// IngestText ingests plain text
func (s *Service) IngestText(ctx context.Context, text string, filename string) (*IngestionResult, error) {
	start := time.Now()

	// Create mock pages
	pages := []PageInfo{{Number: 1, Text: text}}

	return s.processDocument(ctx, pages, nil, filename, start)
}

// IngestBase64Content ingests base64-encoded content
func (s *Service) IngestBase64Content(ctx context.Context, contentB64, docType, filename string) (*IngestionResult, error) {
	start := time.Now()

	// Validate file if filename provided
	if filename != "" {
		result := s.validator.ValidateBase64Content(contentB64, filename)
		if !result.Valid {
			return nil, fmt.Errorf("file validation failed: %s", result.ErrorMessage)
		}
	}

	// Decode content
	content, err := base64.StdEncoding.DecodeString(contentB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	if docType == "pdf" || strings.HasSuffix(filename, ".pdf") {
		// For PDF, we'd extract text and images
		// For now, treat as text since we don't have PDF parser
		return s.processDocument(ctx, []PageInfo{{Number: 1, Text: string(content)}}, nil, filename, start)
	}

	return s.IngestText(ctx, string(content), filename)
}

// processDocument processes document pages with tiered extraction
type PageInfo struct {
	Number int
	Text   string
}

func (s *Service) processDocument(ctx context.Context, pages []PageInfo, images []ExtractedImage, filename string, start time.Time) (*IngestionResult, error) {
	var (
		entities     []Entity
		relationships []Relationship
		llmCalls    int
		visionCalls  int
		chunks       []Chunk
	)

	// Combine all text
	fullText := new(strings.Builder)
	for _, page := range pages {
		fullText.WriteString(page.Text)
		fullText.WriteString("\n\n")
	}
	textStr := fullText.String()

	// === TIER 1: Rule-based extraction (FREE) ===
	tier1Entities := s.extractRules(textStr)
	entities = append(entities, tier1Entities...)

	// === TIER 2: Smart chunking ===
	chunks = s.createChunks(pages)

	// Mark cluster representatives (every 5th chunk)
	for i := range chunks {
		chunks[i].IsClusterRep = (i % 5 == 0)
	}

	// Get cluster representatives
	var clusterReps []Chunk
	for _, chunk := range chunks {
		if chunk.IsClusterRep {
			clusterReps = append(clusterReps, chunk)
		}
	}

	// === TIER 3: LLM extraction on representatives ===
	if s.router != nil && len(clusterReps) > 0 {
		maxLLM := s.config.MaxLLMCalls
		if maxLLM > len(clusterReps) {
			maxLLM = len(clusterReps)
		}

		for _, chunk := range clusterReps[:maxLLM] {
			llmEntities, err := s.extractWithLLM(ctx, chunk.Text)
			if err == nil && len(llmEntities) > 0 {
				entities = append(entities, llmEntities...)
				llmCalls++
			}
		}
	}

	// === VISION: Process complex images ===
	if len(images) > 0 && s.router != nil {
		maxVision := s.config.MaxVisionCalls
		processedImages := 0

		for _, img := range images {
			if processedImages >= maxVision {
				break
			}

			// Calculate complexity if not already set
			if img.ComplexityScore == 0 {
				img.ComplexityScore = s.calculateComplexity(img.ImageBase64)
			}

			// Only process images with complexity > 5 (likely charts/diagrams)
			if img.ComplexityScore > 5.0 {
				visionEntities, err := s.extractWithVision(ctx, img)
				if err == nil && len(visionEntities) > 0 {
					entities = append(entities, visionEntities...)
					visionCalls++
					processedImages++
				}
			}
		}
	}

	// Deduplicate entities
	uniqueEntities := s.deduplicateEntities(entities)

	// Calculate statistics
	stats := IngestStats{
		Pages:           len(pages),
		TotalChunks:     len(chunks),
		ClusterReps:     len(clusterReps),
		Images:          len(images),
		Tier1Entities:   len(tier1Entities),
		LLMCalls:        llmCalls,
		VisionCalls:     visionCalls,
		TotalEntities:    len(uniqueEntities),
		ProcessingTimeMs: time.Since(start).Milliseconds(),
		ExtractedChars:   len(textStr),
	}

	// Build vector tree if we have embeddings
	var vectorTree map[string]*vectorindex.VectorNode
	if s.hasEmbeddings(chunks) {
		inputChunks := make([]vectorindex.Chunk, len(chunks))
		for i, chunk := range chunks {
			inputChunks[i] = vectorindex.Chunk{
				Text:      chunk.Text,
				Embedding: chunk.Embedding,
			}
		}
		vectorTree = s.vectorIndex.BuildIndex(inputChunks)
	}

	return &IngestionResult{
		Entities:      uniqueEntities,
		Relationships: relationships,
		Chunks:       chunks,
		Images:       images,
		Stats:        stats,
		VectorTree:   vectorTree,
		ProcessedAt:   time.Now(),
		Filename:     filename,
	}, nil
}

// extractRules performs rule-based entity extraction (FREE)
func (s *Service) extractRules(text string) []Entity {
	entities := []Entity{}
	seen := make(map[string]bool)

	// Email regex
	emailRegex := regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`)
	emails := emailRegex.FindAllString(text, -1)
	for _, email := range emails[:min(10, len(emails))] {
		key := strings.ToLower(email)
		if !seen[key] {
			seen[key] = true
			entities = append(entities, Entity{
				Name:       email,
				EntityType: "Email",
				Source:     "rule",
				Confidence: 1.0,
			})
		}
	}

	// URL regex
	urlRegex := regexp.MustCompile("https?://[^\\s<>\"{}|\\^`\\[\\]]+")
	urls := urlRegex.FindAllString(text, -1)
	for _, url := range urls[:min(10, len(urls))] {
		key := strings.ToLower(url)
		if !seen[key] {
			seen[key] = true
			entities = append(entities, Entity{
				Name:       url,
				EntityType: "URL",
				Source:     "rule",
				Confidence: 1.0,
			})
		}
	}

	// Money amounts
	moneyRegex := regexp.MustCompile(`\$[\d,]+(?:\.\d{2})?`)
	amounts := moneyRegex.FindAllString(text, -1)
	for _, amount := range amounts[:min(20, len(amounts))] {
		key := amount + "-metric"
		if !seen[key] {
			seen[key] = true
			entities = append(entities, Entity{
				Name:       amount,
				EntityType: "Metric",
				Description: "Monetary value",
				Source:     "rule",
				Confidence: 1.0,
			})
		}
	}

	// Percentages
	pctRegex := regexp.MustCompile(`\d+(?:\.\d+)?%`)
	percentages := pctRegex.FindAllString(text, -1)
	for _, pct := range percentages[:min(20, len(percentages))] {
		key := pct + "-metric"
		if !seen[key] {
			seen[key] = true
			entities = append(entities, Entity{
				Name:       pct,
				EntityType: "Metric",
				Description: "Percentage",
				Source:     "rule",
				Confidence: 1.0,
			})
		}
	}

	// Dates
	dateRegex := regexp.MustCompile(`\b\d{1,2}[/-]\d{1,2}[/-]\d{2,4}\b`)
	dates := dateRegex.FindAllString(text, -1)
	for _, date := range dates[:min(20, len(dates))] {
		key := date + "-date"
		if !seen[key] {
			seen[key] = true
			entities = append(entities, Entity{
				Name:       date,
				EntityType: "Date",
				Source:     "rule",
				Confidence: 0.9,
			})
		}
	}

	return entities
}

// createChunks creates document chunks using the configured chunker
func (s *Service) createChunks(pages []PageInfo) []Chunk {
	chunks := []Chunk{}
	chunkIdx := 0

	for _, page := range pages {
		if strings.TrimSpace(page.Text) == "" {
			continue
		}

		// Use the chunker to split text
		chunkResults := s.chunker.Chunk(page.Text)

		for _, cr := range chunkResults {
			chunkText := strings.TrimSpace(cr.Text)
			if chunkText == "" {
				continue
			}

			// Count words
			wordCount := len(strings.Fields(chunkText))

			chunks = append(chunks, Chunk{
				Text:         chunkText,
				PageNumber:   page.Number,
				ChunkIndex:   chunkIdx,
				CharCount:    cr.CharCount,
				WordCount:    wordCount,
			})
			chunkIdx++
		}
	}

	return chunks
}

// extractWithLLM performs LLM-based entity extraction
func (s *Service) extractWithLLM(ctx context.Context, text string) ([]Entity, error) {
	if s.router == nil {
		return nil, fmt.Errorf("no LLM router configured")
	}

	// Limit text length
	if len(text) > 2000 {
		text = text[:2000]
	}

	prompt := fmt.Sprintf(`Extract key entities from this text. Return JSON array:
[{"name": "...", "type": "Person|Organization|Concept|Metric|Location", "description": "..."}]

Text:
%s

JSON:`, text)

	result, err := s.router.ExtractJSON(ctx, prompt, s.config.LLMProvider, "")
	if err != nil {
		return nil, err
	}

	// Parse response into entities
	entities := []Entity{}
	if resultArray, ok := result["entities"].([]interface{}); ok {
		for _, item := range resultArray {
			if entityMap, ok := item.(map[string]interface{}); ok {
				entities = append(entities, Entity{
					Name:       getString(entityMap, "name"),
					EntityType: getString(entityMap, "type"),
					Description: getString(entityMap, "description"),
					Source:     "llm",
					Confidence: getFloat(entityMap, "confidence"),
				})
			}
		}
	}

	return entities, nil
}

// extractWithVision extracts entities from images using Vision LLM
func (s *Service) extractWithVision(ctx context.Context, img ExtractedImage) ([]Entity, error) {
	if s.router == nil {
		return nil, fmt.Errorf("no LLM router configured")
	}

	prompt := `Analyze this image. Extract entities as JSON:
[{"name": "...", "type": "Person|Concept|Metric", "description": "..."}]

Only include clearly visible named entities, metrics, or concepts.`

	req := &router.VisionRequest{
		ImageBase64: img.ImageBase64,
		Prompt:      prompt,
	}
	response, err := s.router.GenerateVision(ctx, req)
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	entities := []Entity{}
	jsonStart := strings.Index(response, "[")
	jsonEnd := strings.LastIndex(response, "]")

	if jsonStart >= 0 && jsonEnd > jsonStart {
		jsonStr := response[jsonStart : jsonEnd+1]
		var result []map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
			for _, item := range result {
				if name, ok := item["name"].(string); ok && name != "" {
					entities = append(entities, Entity{
						Name:       name,
						EntityType: getString(item, "type"),
						Description: getString(item, "description"),
						Source:     "vision",
						Confidence: 0.85,
					})
				}
			}
		}
	}

	return entities, nil
}

// calculateComplexity calculates image complexity score
func (s *Service) calculateComplexity(imageBase64 string) float64 {
	// Simple heuristic: size-based complexity
	size := len(imageBase64)
	if size < 10000 {
		return 1.0
	}
	if size < 50000 {
		return minFloat64(float64(size)/50000*3, 5.0)
	}
	return minFloat64(float64(size)/100000*10, 10.0)
}

// deduplicateEntities removes duplicate entities
func (s *Service) deduplicateEntities(entities []Entity) []Entity {
	seen := make(map[string]*Entity)
	unique := make([]Entity, 0)

	for _, entity := range entities {
		key := strings.ToLower(entity.Name) + "|" + entity.EntityType
		if existing, ok := seen[key]; ok {
			existing.Count++
		} else {
			seen[key] = &entity
			entity.Count = 1
			unique = append(unique, entity)
		}
	}

	return unique
}

// hasEmbeddings checks if chunks have embeddings
func (s *Service) hasEmbeddings(chunks []Chunk) bool {
	for _, chunk := range chunks {
		if len(chunk.Embedding) > 0 {
			return true
		}
	}
	return false
}

// GenerateEmbeddings generates embeddings for all chunks
func (s *Service) GenerateEmbeddings(ctx context.Context, chunks []Chunk) error {
	// This would call an embedding service
	// For now, this is a placeholder
	return nil
}

// GetStats returns statistics about the ingester
func (s *Service) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"type":        "document_ingester",
		"chunk_size":   s.config.ChunkSize,
		"chunk_overlap": s.config.ChunkOverlap,
		"max_llm_calls": s.config.MaxLLMCalls,
		"max_vision_calls": s.config.MaxVisionCalls,
		"llm_provider": string(s.config.LLMProvider),
	}
}

// Helper functions

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case float32:
			return float64(val)
		case int:
			return float64(val)
		}
	}
	return 0.0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// ExtractEntities extracts entities from text
func ExtractEntities(text string) []Entity {
	ingester := New(nil, nil, nil)
	return ingester.extractRules(text)
}

// ChunkText splits text into chunks
func ChunkText(text string, chunkSize, overlap int) []Chunk {
	cfg := &chunking.Config{
		ChunkSize:       chunkSize,
		Overlap:         overlap,
		MinChunkSize:    50,
		ForwardFallback: true,
		RespectSentence: true,
	}

	chunker := chunking.New(cfg)
	results := chunker.Chunk(text)

	chunks := make([]Chunk, len(results))
	for i, r := range results {
		chunks[i] = Chunk{
			Text:        r.Text,
			PageNumber:  1,
			ChunkIndex:  i,
			CharCount:   r.CharCount,
			WordCount:   len(strings.Fields(r.Text)),
		}
	}

	return chunks
}

// ProcessDocument processes a document and returns ingestion result
func ProcessDocument(ctx context.Context, text string) (*IngestionResult, error) {
	ingester := New(nil, nil, nil)
	return ingester.IngestText(ctx, text, "")
}

// ValidateUpload validates a document upload before processing
func ValidateUpload(contentB64, filename, docType string) (*validation.ValidationResult, error) {
	validator := validation.DefaultConfig()
	result := validator.ValidateBase64Content(contentB64, filename)
	return &result, nil
}
