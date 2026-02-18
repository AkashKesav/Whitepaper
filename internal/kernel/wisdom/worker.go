package wisdom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/reflective-memory-kernel/internal/graph"
	"go.uber.org/zap"
)

// Embedder interface for generating embeddings
type Embedder interface {
	Embed(text string) ([]float32, error)
}

// VectorStorer interface for storing vectors in a vector database
// VectorStorer interface for storing vectors in a vector database
type VectorStorer interface {
	Store(ctx context.Context, namespace, uid string, embedding []float32, metadata map[string]interface{}) error
}

// Config holds configuration for the Wisdom Worker
type Config struct {
	BatchSize     int
	FlushInterval time.Duration
	AIServiceURL  string
}

// WisdomManager manages the Cold Path (Wisdom Layer)
// It buffers events and writes high-density summaries to DGraph
type WisdomManager struct {
	config      Config
	logger      *zap.Logger
	graphClient *graph.Client

	// Embedder for Hybrid RAG
	embedder     Embedder
	vectorStorer VectorStorer

	// Buffer
	eventBuffer []graph.TranscriptEvent
	mu          sync.Mutex

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager creates a new WisdomManager
func NewManager(cfg Config, graphClient *graph.Client, embedder Embedder, vectorStorer VectorStorer, logger *zap.Logger) *WisdomManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &WisdomManager{
		config:       cfg,
		logger:       logger,
		graphClient:  graphClient,
		embedder:     embedder,
		vectorStorer: vectorStorer,
		eventBuffer:  make([]graph.TranscriptEvent, 0, cfg.BatchSize),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start starts the background batch processing loop
func (wm *WisdomManager) Start() {
	wm.wg.Add(1)
	go wm.runLoop()
}

// Stop gracefully shuts down the worker
func (wm *WisdomManager) Stop() {
	wm.cancel()
	wm.wg.Wait()
	// Process remaining items? Maybe. For now, we drop or implement graceful drain later.
}

// AddEvent adds an event to the buffer (called from Hot Path)
func (wm *WisdomManager) AddEvent(event graph.TranscriptEvent) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	wm.eventBuffer = append(wm.eventBuffer, event)

	// Trigger flush if batch size reached
	if len(wm.eventBuffer) >= wm.config.BatchSize {
		// We could signal channel, but for simplicity let the ticker handle strict time
		// OR we can force a flush here. Let's rely on ticker for simpler concurrency for now
		// or better: signal a flush.
		// For MVP: let ticker pick it up to avoid lock contention.
	}
}

func (wm *WisdomManager) runLoop() {
	defer wm.wg.Done()

	ticker := time.NewTicker(wm.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-wm.ctx.Done():
			return
		case <-ticker.C:
			wm.flushBatch()
		}
	}
}

func (wm *WisdomManager) flushBatch() {
	wm.mu.Lock()
	if len(wm.eventBuffer) == 0 {
		wm.mu.Unlock()
		return
	}

	// Swap buffer
	batch := wm.eventBuffer
	wm.eventBuffer = make([]graph.TranscriptEvent, 0, wm.config.BatchSize)
	wm.mu.Unlock()

	// Process batch (Async from ingest, but sync within worker)
	if err := wm.processBatch(context.Background(), batch); err != nil {
		wm.logger.Error("Failed to process wisdom batch", zap.Error(err))
	}
}

func (wm *WisdomManager) processBatch(ctx context.Context, batch []graph.TranscriptEvent) error {
	wm.logger.Info("Wisdom Layer: Processing batch", zap.Int("count", len(batch)))

	// 1. Group by Conversation/Context to avoid cross-contamination?
	// For "Quantum" speed, we might just batch everything.
	// But summarization works best per conversation.
	// We'll group by Namespace (ContextID)

	batchesByNS := make(map[string][]graph.TranscriptEvent)
	for _, e := range batch {
		ns := e.Namespace
		if ns == "" {
			ns = fmt.Sprintf("user_%s", e.UserID)
		}
		batchesByNS[ns] = append(batchesByNS[ns], e)
	}

	for ns, events := range batchesByNS {
		start := time.Now()
		summary, entities, err := wm.summarizeEvents(ctx, events)
		if err != nil {
			wm.logger.Error("Summarization failed", zap.String("namespace", ns), zap.Error(err))
			continue
		}

		// Write to Graph
		// We use the existing graph client but maybe a specialized method for summaries
		// For now, we reuse the entity structure but mark them as High Confidence

		// Create a synthetic "Summary" node or just Fact nodes?
		// "Fact" nodes are good.

		// Logic:
		// 1. Create a "Summary" event/node?
		// 2. Create extracted entities.

		// Let's create a new function in GraphClient for "IngestWisdom" or similar.
		// For now, let's reuse CreateNodes/CreateEdges logic locally or call a helper.
		// We can reuse the IngestionPipeline's `processBatchedEntities` logic if we export it
		// or we can implement a simplified version here.

		wm.logger.Info("Wisdom Layer: Summary generated",
			zap.String("namespace", ns),
			zap.Int("entities", len(entities)),
			zap.String("summary_snippet", summary[:min(50, len(summary))]),
			zap.Duration("duration", time.Since(start)))

		// 3. Write Phase (High Density)
		summaryUID, err := wm.graphClient.IngestWisdomBatch(ctx, ns, summary, entities)
		if err != nil {
			wm.logger.Error("Failed to persist wisdom batch", zap.String("namespace", ns), zap.Error(err))
			continue
		}
		wm.logger.Info("Wisdom Batch crystallized to DGraph", zap.String("namespace", ns), zap.String("uid", summaryUID))

		// 4. Generate and store embedding for Hybrid RAG
		if wm.embedder != nil && wm.vectorStorer != nil && summaryUID != "" {
			embedding, err := wm.embedder.Embed(summary)
			if err != nil {
				wm.logger.Warn("Failed to generate embedding for summary", zap.Error(err))
			} else {
				// Store with metadata
				metadata := map[string]interface{}{
					"type": "summary",
					"text": summary,
				}
				if err := wm.vectorStorer.Store(ctx, ns, summaryUID, embedding, metadata); err != nil {
					wm.logger.Warn("Failed to store embedding in vector index", zap.Error(err))
				} else {
					wm.logger.Info("Stored summary embedding in Qdrant",
						zap.String("namespace", ns),
						zap.String("uid", summaryUID),
						zap.Int("dims", len(embedding)))
				}
			}
		}
	}

	return nil
}

func (wm *WisdomManager) summarizeEvents(ctx context.Context, events []graph.TranscriptEvent) (string, []graph.ExtractedEntity, error) {
	// Construct Prompt
	var conversationText bytes.Buffer
	for _, e := range events {
		conversationText.WriteString(fmt.Sprintf("User: %s\nAI: %s\n", e.UserQuery, e.AIResponse))
	}

	// Request to External AI
	type SummaryRequest struct {
		Text string `json:"text"`
		Type string `json:"type"` // "crystallize"
	}

	reqBody := SummaryRequest{
		Text: conversationText.String(),
		Type: "crystallize",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		wm.config.AIServiceURL+"/summarize_batch",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Fallback: Local entity extraction from conversation text
		wm.logger.Warn("Summarize endpoint failed, using local extraction fallback")
		return wm.localExtractEntities(events)
	}

	// Parse response (Expect standard entity extraction format + summary text)
	type SummaryResponse struct {
		Summary  string                  `json:"summary"`
		Entities []graph.ExtractedEntity `json:"entities"`
	}

	var res SummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", nil, err
	}

	return res.Summary, res.Entities, nil
}

// localExtractEntities extracts entities from conversation events when AI endpoint fails
func (wm *WisdomManager) localExtractEntities(events []graph.TranscriptEvent) (string, []graph.ExtractedEntity, error) {
	var entities []graph.ExtractedEntity
	var summaryParts []string

	for _, event := range events {
		// Extract facts from user message
		userMessage := event.UserQuery
		if userMessage != "" {
			// Create a Fact entity from the user's statement
			entities = append(entities, graph.ExtractedEntity{
				Name:        wm.extractKeyPhrase(userMessage),
				Type:        graph.NodeTypeFact,
				Description: userMessage,
				SourceText:  userMessage, // Store the original user sentence
			})
			summaryParts = append(summaryParts, userMessage)
		}
	}

	// Build summary from all user inputs
	summary := "User shared: "
	if len(summaryParts) > 0 {
		for i, part := range summaryParts {
			if i > 0 {
				summary += "; "
			}
			if len(part) > 100 {
				summary += part[:100] + "..."
			} else {
				summary += part
			}
		}
	} else {
		summary = "No specific facts extracted"
	}

	wm.logger.Info("Local extraction completed",
		zap.Int("entities", len(entities)),
		zap.String("summary", summary[:min(50, len(summary))]))

	return summary, entities, nil
}

// extractKeyPhrase extracts a short key phrase from text (first 5 words or less)
func (wm *WisdomManager) extractKeyPhrase(text string) string {
	import_strings := []byte(text)
	words := make([]string, 0)
	currentWord := ""

	for _, c := range import_strings {
		if c == ' ' || c == '\n' || c == '\t' {
			if currentWord != "" {
				words = append(words, currentWord)
				currentWord = ""
			}
		} else {
			currentWord += string(c)
		}
	}
	if currentWord != "" {
		words = append(words, currentWord)
	}

	// Take first 5 words max
	if len(words) > 5 {
		words = words[:5]
	}

	result := ""
	for i, w := range words {
		if i > 0 {
			result += " "
		}
		result += w
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
