// Package kernel provides the ingestion pipeline for the Memory Kernel.
// This implements Phase 1 of the three-phase loop: receiving transcript events
// from the Front-End Agent and writing them to the Knowledge Graph.
package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/graph"
)

// IngestionStats holds metrics about ingestion performance
type IngestionStats struct {
	TotalProcessed       int64     `json:"total_processed"`
	TotalErrors          int64     `json:"total_errors"`
	TotalEntitiesCreated int64     `json:"total_entities_created"`
	LastDurationMs       int64     `json:"last_duration_ms"`
	AvgDurationMs        float64   `json:"avg_duration_ms"`
	LastExtractionMs     int64     `json:"last_extraction_ms"`
	LastDgraphWriteMs    int64     `json:"last_dgraph_write_ms"`
	LastProcessedAt      time.Time `json:"last_processed_at"`
	mu                   sync.RWMutex
}

// IngestionPipeline handles the ingestion of transcript events into the Knowledge Graph
type IngestionPipeline struct {
	graphClient   *graph.Client
	jetStream     nats.JetStreamContext
	redisClient   *redis.Client
	aiServicesURL string

	batchSize     int
	flushInterval time.Duration
	logger        *zap.Logger

	// Batching
	eventBuffer []graph.TranscriptEvent
	bufferMu    sync.Mutex

	// Metrics
	stats         IngestionStats
	totalDuration int64 // for calculating average
}

// GetStats returns current ingestion statistics
func (p *IngestionPipeline) GetStats() IngestionStats {
	p.stats.mu.RLock()
	defer p.stats.mu.RUnlock()
	return IngestionStats{
		TotalProcessed:       p.stats.TotalProcessed,
		TotalErrors:          p.stats.TotalErrors,
		TotalEntitiesCreated: p.stats.TotalEntitiesCreated,
		LastDurationMs:       p.stats.LastDurationMs,
		AvgDurationMs:        p.stats.AvgDurationMs,
		LastExtractionMs:     p.stats.LastExtractionMs,
		LastDgraphWriteMs:    p.stats.LastDgraphWriteMs,
		LastProcessedAt:      p.stats.LastProcessedAt,
	}
}

// NewIngestionPipeline creates a new ingestion pipeline
func NewIngestionPipeline(
	graphClient *graph.Client,
	jetStream nats.JetStreamContext,
	redisClient *redis.Client,
	aiServicesURL string,
	batchSize int,
	flushInterval time.Duration,
	logger *zap.Logger,
) *IngestionPipeline {
	return &IngestionPipeline{
		graphClient:   graphClient,
		jetStream:     jetStream,
		redisClient:   redisClient,
		aiServicesURL: aiServicesURL,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		logger:        logger,
		eventBuffer:   make([]graph.TranscriptEvent, 0, batchSize),
	}
}

// Process processes a raw message from NATS
func (p *IngestionPipeline) Process(ctx context.Context, data []byte) error {
	var event graph.TranscriptEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal transcript event: %w", err)
	}

	return p.Ingest(ctx, &event)
}

// Ingest ingests a transcript event into the Knowledge Graph
func (p *IngestionPipeline) Ingest(ctx context.Context, event *graph.TranscriptEvent) error {
	startTime := time.Now()

	p.logger.Debug("Ingesting transcript event",
		zap.String("conversation_id", event.ConversationID),
		zap.String("user_id", event.UserID))

	// Step 1: Extract entities using the AI service
	extractionStart := time.Now()
	entities, err := p.extractEntities(ctx, event)
	if err != nil {
		p.logger.Warn("Entity extraction failed, using basic extraction",
			zap.Error(err))
		// Fall back to basic extraction
		entities = p.basicEntityExtraction(event)
	}
	extractionDuration := time.Since(extractionStart)
	event.ExtractedEntities = entities

	// DEBUG: Log extracted entities
	for i, e := range entities {
		p.logger.Info("Extracted entity",
			zap.Int("index", i),
			zap.String("name", e.Name),
			zap.String("type", string(e.Type)),
			zap.String("description", e.Description))
	}

	// Step 2: Batch process all entities and relationships
	dgraphStart := time.Now()
	if err := p.processBatchedEntities(ctx, event.UserID, event.ConversationID, entities); err != nil {
		p.logger.Error("Failed to batch process entities", zap.Error(err))
		p.updateStats(0, extractionDuration, time.Duration(0), true)
		return err
	}
	dgraphDuration := time.Since(dgraphStart)

	// Step 3: Cache recent context in Redis for fast access
	if err := p.cacheRecentContext(ctx, event); err != nil {
		p.logger.Warn("Failed to cache context", zap.Error(err))
	}

	totalDuration := time.Since(startTime)
	p.updateStats(len(entities), extractionDuration, dgraphDuration, false)

	p.logger.Info("Transcript event ingested successfully",
		zap.String("conversation_id", event.ConversationID),
		zap.Int("entities_extracted", len(entities)),
		zap.Duration("extraction_time", extractionDuration),
		zap.Duration("dgraph_time", dgraphDuration),
		zap.Duration("total_time", totalDuration))

	return nil
}

// updateStats updates ingestion statistics
func (p *IngestionPipeline) updateStats(entityCount int, extractionTime, dgraphTime time.Duration, isError bool) {
	p.stats.mu.Lock()
	defer p.stats.mu.Unlock()

	if isError {
		p.stats.TotalErrors++
		return
	}

	p.stats.TotalProcessed++
	p.stats.TotalEntitiesCreated += int64(entityCount)
	p.stats.LastExtractionMs = extractionTime.Milliseconds()
	p.stats.LastDgraphWriteMs = dgraphTime.Milliseconds()
	p.stats.LastDurationMs = (extractionTime + dgraphTime).Milliseconds()
	p.stats.LastProcessedAt = time.Now()

	p.totalDuration += p.stats.LastDurationMs
	if p.stats.TotalProcessed > 0 {
		p.stats.AvgDurationMs = float64(p.totalDuration) / float64(p.stats.TotalProcessed)
	}
}

// extractEntities calls the AI service to extract structured entities from the transcript
func (p *IngestionPipeline) extractEntities(ctx context.Context, event *graph.TranscriptEvent) ([]graph.ExtractedEntity, error) {
	type ExtractionRequest struct {
		UserQuery  string `json:"user_query"`
		AIResponse string `json:"ai_response"`
		Context    string `json:"context,omitempty"`
	}

	reqBody := ExtractionRequest{
		UserQuery:  event.UserQuery,
		AIResponse: event.AIResponse,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		p.aiServicesURL+"/extract",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// Increased timeout to 120s to support large models running on hybrid CPU/GPU or pure CPU
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("extraction service returned status %d", resp.StatusCode)
	}

	var entities []graph.ExtractedEntity
	if err := json.NewDecoder(resp.Body).Decode(&entities); err != nil {
		return nil, err
	}

	return entities, nil
}

// basicEntityExtraction provides fallback entity extraction without AI
func (p *IngestionPipeline) basicEntityExtraction(event *graph.TranscriptEvent) []graph.ExtractedEntity {
	// This is a simple fallback that creates a Fact node for the conversation
	return []graph.ExtractedEntity{
		{
			Name: fmt.Sprintf("Conversation_%s", event.ConversationID),
			Type: graph.NodeTypeFact,
			Attributes: map[string]string{
				"user_query":  event.UserQuery,
				"ai_response": event.AIResponse,
				"timestamp":   event.Timestamp.Format(time.RFC3339),
			},
		},
	}
}

// processBatchedEntities handles the 3-step batched ingestion: Read -> Write Nodes -> Write Edges
func (p *IngestionPipeline) processBatchedEntities(ctx context.Context, userID, conversationID string, entities []graph.ExtractedEntity) error {
	// 1. COLLECT ALL UNIQUE NAMES
	// We need to check existence for the User, all extracted Entities, and any Relation Targets
	uniqueNames := make(map[string]bool)
	uniqueNames[userID] = true // Always check user

	for _, e := range entities {
		uniqueNames[e.Name] = true
		for _, r := range e.Relations {
			uniqueNames[r.TargetName] = true
		}
	}

	namesList := make([]string, 0, len(uniqueNames))
	for name := range uniqueNames {
		namesList = append(namesList, name)
	}

	// 2. BULK READ - Check what exists
	existingNodes, err := p.graphClient.GetNodesByNames(ctx, namesList)
	if err != nil {
		return fmt.Errorf("failed to batch get nodes: %w", err)
	}

	// 3. BULK CREATE NODES - Prepare missing nodes
	nodesToCreate := make([]*graph.Node, 0)

	// Check User
	if _, exists := existingNodes[userID]; !exists {
		nodesToCreate = append(nodesToCreate, &graph.Node{
			DType:      []string{string(graph.NodeTypeUser)},
			Name:       userID,
			Activation: 1.0,
			Confidence: 1.0,
		})
	}

	// Check Entities and Relations
	for _, e := range entities {
		if _, exists := existingNodes[e.Name]; !exists {
			// Normalize type
			dtype := e.Type
			if dtype == "" {
				dtype = graph.NodeTypeEntity
			}

			nodesToCreate = append(nodesToCreate, &graph.Node{
				DType:                []string{string(dtype)},
				Name:                 e.Name,
				Description:          e.Description,
				Tags:                 e.Tags,
				Attributes:           e.Attributes,
				SourceConversationID: conversationID,
				Activation:           0.8,
				Confidence:           0.9,
			})
		}

		for _, r := range e.Relations {
			if _, exists := existingNodes[r.TargetName]; !exists {
				// Normalize target type
				dtype := r.TargetType
				if dtype == "" {
					dtype = graph.NodeTypeEntity
				}

				nodesToCreate = append(nodesToCreate, &graph.Node{
					DType:      []string{string(dtype)},
					Name:       r.TargetName,
					Activation: 0.5,
					Confidence: 0.7,
				})
			}
		}
	}

	// Execute Batch Create
	if len(nodesToCreate) > 0 {
		p.logger.Info("Batch creating nodes", zap.Int("count", len(nodesToCreate)))
		newUIDs, err := p.graphClient.CreateNodes(ctx, nodesToCreate)
		if err != nil {
			return err
		}

		// Merge new UIDs into existingNodes map so we can build edges
		for name, uid := range newUIDs {
			// We only need the UID for edge creation
			existingNodes[name] = &graph.Node{UID: uid, Name: name}
		}
	}

	// 4. BULK CREATE EDGES
	edgesToCreate := make([]graph.EdgeInput, 0)
	userUID := existingNodes[userID].UID

	for _, e := range entities {
		entityUID, ok := existingNodes[e.Name]
		if !ok {
			continue
		} // Should not happen

		// User -> Entity (KNOWS)
		edgesToCreate = append(edgesToCreate, graph.EdgeInput{
			FromUID: userUID,
			ToUID:   entityUID.UID,
			Type:    graph.EdgeTypeKnows,
			Status:  graph.EdgeStatusCurrent,
		})

		// Entity -> Target (Relations)
		for _, r := range e.Relations {
			targetUID, ok := existingNodes[r.TargetName]
			if !ok {
				continue
			}

			edgesToCreate = append(edgesToCreate, graph.EdgeInput{
				FromUID: entityUID.UID,
				ToUID:   targetUID.UID,
				Type:    r.Type,
				Status:  graph.EdgeStatusCurrent,
			})
		}
	}

	// Execute Batch edges
	if len(edgesToCreate) > 0 {
		p.logger.Info("Batch creating edges", zap.Int("count", len(edgesToCreate)))
		if err := p.graphClient.CreateEdges(ctx, edgesToCreate); err != nil {
			return err
		}
	}

	// 5. ASYNC UPDATES (Fire and forget)
	// Update activation/tags for existing nodes that we found in step 2
	go func() {
		// Create a separate context for async ops
		asyncCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		for _, e := range entities {
			if node, exists := existingNodes[e.Name]; exists {
				// Boost activation
				p.graphClient.IncrementAccessCount(asyncCtx, node.UID, graph.DefaultActivationConfig())
				// Add tags
				if len(e.Tags) > 0 {
					p.graphClient.AddTags(asyncCtx, node.UID, e.Tags)
				}
			}
		}
	}()

	return nil
}

// cacheRecentContext caches the recent conversation context in Redis
func (p *IngestionPipeline) cacheRecentContext(ctx context.Context, event *graph.TranscriptEvent) error {
	key := fmt.Sprintf("context:%s:recent", event.UserID)

	// Store the last 10 conversation turns
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Push to the list and trim to 10 items
	if err := p.redisClient.LPush(ctx, key, data).Err(); err != nil {
		return err
	}
	if err := p.redisClient.LTrim(ctx, key, 0, 9).Err(); err != nil {
		return err
	}
	// Set expiration to 24 hours
	if err := p.redisClient.Expire(ctx, key, 24*time.Hour).Err(); err != nil {
		return err
	}

	return nil
}

// PublishTranscript publishes a transcript event to NATS for ingestion
// This is called by the Front-End Agent
func PublishTranscript(js nats.JetStreamContext, event *graph.TranscriptEvent) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	subject := fmt.Sprintf("transcripts.%s", event.UserID)

	// Log the publish attempt
	log.Printf("[NATS] Publishing to '%s' (user: %s, query: %s)", subject, event.UserID, event.UserQuery[:min(50, len(event.UserQuery))])

	ack, err := js.Publish(subject, data)
	if err != nil {
		log.Printf("[NATS] Publish FAILED: %v", err)
		return err
	}
	log.Printf("[NATS] Publish SUCCESS: stream=%s, seq=%d", ack.Stream, ack.Sequence)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
