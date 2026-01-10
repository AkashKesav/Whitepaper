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
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/ai/local"
	"github.com/reflective-memory-kernel/internal/graph"
	"github.com/reflective-memory-kernel/internal/kernel/wisdom"
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
	graphClient *graph.Client
	jetStream   nats.JetStreamContext

	redisClient   *redis.Client
	aiServicesURL string
	localEmbedder local.LocalEmbedder
	wisdomManager *wisdom.WisdomManager
	vectorIndex   *VectorIndex

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
	localEmbedder local.LocalEmbedder,
	wisdomManager *wisdom.WisdomManager,
	vectorIndex *VectorIndex,
	batchSize int,
	flushInterval time.Duration,
	logger *zap.Logger,
) *IngestionPipeline {
	return &IngestionPipeline{
		graphClient:   graphClient,
		jetStream:     jetStream,
		redisClient:   redisClient,
		aiServicesURL: aiServicesURL,
		localEmbedder: localEmbedder,
		wisdomManager: wisdomManager,
		vectorIndex:   vectorIndex,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		logger:        logger,
		eventBuffer:   make([]graph.TranscriptEvent, 0, batchSize),
	}
}

// Process processes a raw message from NATS
func (p *IngestionPipeline) Process(ctx context.Context, data []byte) error {
	if p == nil {
		return fmt.Errorf("ingestion pipeline is nil")
	}

	var event graph.TranscriptEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal transcript event: %w", err)
	}

	return p.Ingest(ctx, &event)
}

// Ingest ingests a transcript event into the Knowledge Graph
func (p *IngestionPipeline) Ingest(ctx context.Context, event *graph.TranscriptEvent) error {
	// Safety checks
	if p == nil {
		return fmt.Errorf("ingestion pipeline is nil")
	}
	if event == nil {
		return fmt.Errorf("event is nil")
	}
	if p.logger == nil {
		return fmt.Errorf("logger is nil")
	}
	if p.graphClient == nil {
		return fmt.Errorf("graph client is nil")
	}

	startTime := time.Now()

	p.logger.Debug("Ingesting transcript event",
		zap.String("conversation_id", event.ConversationID),
		zap.String("user_id", event.UserID))

	// PERMISSION CHECK: For group namespaces, verify user is a member (write access)
	namespace := event.Namespace
	if namespace == "" {
		namespace = fmt.Sprintf("user_%s", event.UserID)
	}
	if strings.HasPrefix(namespace, "group_") {
		isMember, err := p.graphClient.IsWorkspaceMember(ctx, namespace, event.UserID)
		if err != nil {
			p.logger.Error("Failed to check workspace membership for write", zap.Error(err))
			return fmt.Errorf("permission check failed: %w", err)
		}
		if !isMember {
			p.logger.Warn("Write access denied: user is not a workspace member",
				zap.String("user", event.UserID),
				zap.String("workspace", namespace))
			return fmt.Errorf("write access denied: not a member of workspace %s", namespace)
		}
		p.logger.Debug("Workspace write access verified", zap.String("namespace", namespace))
	}

	// Step 1: Hot Path - Local Embedding (Replacing External AI)
	embedStart := time.Now()

	// Use local embedder if available
	var chatNodeUID string // Will hold DGraph UID for unified ID approach
	if p.localEmbedder != nil {
		vec, err := p.localEmbedder.Embed(event.UserQuery)
		if err != nil {
			p.logger.Error("Local embedding failed", zap.Error(err))
		} else {
			p.logger.Info("Generated local embedding (Hot Path)",
				zap.Int("dims", len(vec)),
				zap.Duration("latency", time.Since(embedStart)))

			// UNIFIED ID APPROACH: Create DGraph node first to get UID
			// This ensures policy matching works (policies target DGraph UIDs)
			if p.graphClient != nil {
				chatNode := &graph.Node{
					DType:                []string{string(graph.NodeTypeFact)},
					Name:                 fmt.Sprintf("Chat: %s", truncateString(event.UserQuery, 50)),
					Description:          event.UserQuery,
					Namespace:            namespace,
					SourceConversationID: event.ConversationID,
					Activation:           0.8, // High initial activation for recent chat
					Confidence:           0.9,
					Tags:                 []string{"chat", "memory"},
					CreatedAt:            time.Now(),
					LastAccessed:         time.Now(),
				}

				// Create in DGraph and get UID
				uids, err := p.graphClient.CreateNodes(ctx, []*graph.Node{chatNode})
				if err != nil {
					p.logger.Warn("Failed to create chat node in DGraph", zap.Error(err))
				} else if len(uids) > 0 {
					// Get the UID that was assigned
					for name, uid := range uids {
						p.logger.Debug("Created DGraph node", zap.String("name", name), zap.String("uid", uid))
						chatNodeUID = uid
						break
					}
				}
			}

			// STORE EMBEDDING IN QDRANT with DGraph UID (Unified ID)
			// This enables semantic search AND policy matching
			if p.vectorIndex != nil && chatNodeUID != "" {
				// Metadata for retrieval
				metadata := map[string]interface{}{
					"text":            event.UserQuery,
					"ai_response":     event.AIResponse,
					"conversation_id": event.ConversationID,
					"type":            "chat",
					"timestamp":       event.Timestamp.Format(time.RFC3339),
				}

				if err := p.vectorIndex.Store(ctx, namespace, chatNodeUID, vec, metadata); err != nil {
					p.logger.Warn("Failed to store embedding in vector index", zap.Error(err))
				} else {
					p.logger.Debug("Stored chat embedding in Qdrant with unified UID", zap.String("uid", chatNodeUID))
				}
			} else if p.vectorIndex != nil && chatNodeUID == "" {
				// Fallback: store with synthetic UID if DGraph failed
				uid := fmt.Sprintf("chat_%s_%d", event.ConversationID, time.Now().UnixNano())
				metadata := map[string]interface{}{
					"text":            event.UserQuery,
					"ai_response":     event.AIResponse,
					"conversation_id": event.ConversationID,
					"type":            "chat",
					"timestamp":       event.Timestamp.Format(time.RFC3339),
				}
				if err := p.vectorIndex.Store(ctx, namespace, uid, vec, metadata); err != nil {
					p.logger.Warn("Failed to store embedding in vector index (fallback)", zap.Error(err))
				}
			}
		}
	} else {
		p.logger.Warn("Local embedder is nil, skipping Hot Path embedding")
	}

	extractionDuration := time.Since(embedStart)

	// Phase 2 Optimization: Hand off to Cold Path (Wisdom Layer)
	if p.wisdomManager != nil {
		p.wisdomManager.AddEvent(*event)
		p.logger.Info("Event queued for Wisdom Layer (Cold Path)")
	} else {
		p.logger.Warn("Wisdom Manager is nil, event will NOT be persisted to graph")
	}

	// Phase 1 Optimization: Skip External Entity Extraction and DGraph Writes for raw chat
	// We rely on Phase 2 "Cold Path" Batcher to summarize and write to DGraph later.
	// For now, checks are removed to ensure <10ms latency.

	entities := []graph.ExtractedEntity{} // Empty entities
	event.ExtractedEntities = entities

	// Step 3: Cache recent context in Redis for fast access (Hot Context)
	// We still need the raw message in Redis for the Agent to see "Recent Chat"
	if err := p.cacheRecentContext(ctx, event); err != nil {
		p.logger.Warn("Failed to cache context", zap.Error(err))
	}

	totalDuration := time.Since(startTime)
	// We pass 0 for dgraph time as we skipped it
	p.updateStats(0, extractionDuration, 0, false)

	p.logger.Info("Transcript processed (Hot Path)",
		zap.String("conversation_id", event.ConversationID),
		zap.Duration("total_time", totalDuration))

	return nil
}

// IngestDirect handles direct ingestion from the Monolith (Zero-Copy)
func (p *IngestionPipeline) IngestDirect(ctx context.Context, event *graph.TranscriptEvent) error {
	p.logger.Debug("Direct ingestion received (Zero-Copy)",
		zap.String("conversation_id", event.ConversationID))
	return p.Ingest(ctx, event)
}

// updateStats updates ingestion statistics
func (p *IngestionPipeline) updateStats(entityCount int, extractionTime, dgraphTime time.Duration, isError bool) {
	if p == nil {
		return
	}
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

// isValidEntityName filters out UUIDs and metadata nodes
func isValidEntityName(name string) bool {
	if len(name) == 0 || len(name) < 2 {
		return false
	}
	// Filter UUIDs (8-4-4-4-12 format)
	if len(name) == 36 && strings.Count(name, "-") == 4 {
		return false
	}
	// Filter user IDs
	if strings.HasPrefix(name, "user_") {
		return false
	}
	// Filter conversation metadata
	if strings.HasPrefix(name, "Conversation_") {
		return false
	}
	return true
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
func (p *IngestionPipeline) processBatchedEntities(ctx context.Context, namespace, userID, conversationID string, entities []graph.ExtractedEntity) error {
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

	// Namespace passed in
	namesp := namespace

	// 2. BULK READ - Check what exists
	existingNodes, err := p.graphClient.GetNodesByNames(ctx, namesp, namesList)
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
			Namespace:  namesp,
		})
	}

	// Check Entities and Relations
	for _, e := range entities {
		// Filter out junk/metadata nodes
		if !isValidEntityName(e.Name) {
			p.logger.Debug("Skipping invalid entity name", zap.String("name", e.Name))
			continue
		}

		if _, exists := existingNodes[e.Name]; !exists {
			// Normalize type
			dtype := e.Type
			if dtype == "" {
				dtype = graph.NodeTypeEntity
			}

			// Create node for each unique entity
			nodesToCreate = append(nodesToCreate, &graph.Node{
				DType:                []string{string(dtype)},
				Name:                 e.Name,
				Description:          e.Description,
				Tags:                 e.Tags,
				Attributes:           e.Attributes,
				SourceConversationID: conversationID,
				Activation:           0.5, // Start at neutral activation
				Confidence:           0.8,
				Namespace:            namesp,
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
					Namespace:  namesp,
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

	// Safe access to userUID - check if it exists in map first
	userNode, userOk := existingNodes[userID]
	if !userOk || userNode == nil {
		// User node should have been created, but if not, skip edge creation
		p.logger.Warn("User node not found in existingNodes, skipping edge creation",
			zap.String("userID", userID))
		return nil
	}
	userUID := userNode.UID

	// Safe access to convUID - conversation node may not exist
	var convUID string
	if convNode, ok := existingNodes[conversationID]; ok && convNode != nil {
		convUID = convNode.UID
	}

	for _, e := range entities {
		entityUID, ok := existingNodes[e.Name]
		if !ok || entityUID == nil {
			continue
		} // Should not happen

		// User -> Entity (KNOWS) - Relation
		edgesToCreate = append(edgesToCreate, graph.EdgeInput{
			FromUID: userUID,
			ToUID:   entityUID.UID,
			Type:    graph.EdgeTypeKnows,
			Status:  graph.EdgeStatusCurrent,
		})

		// Entity -> User (CREATED_BY) - Ownership
		edgesToCreate = append(edgesToCreate, graph.EdgeInput{
			FromUID: entityUID.UID,
			ToUID:   userUID,
			Type:    "created_by",
			Status:  graph.EdgeStatusCurrent,
		})

		// Entity -> Conversation (DERIVED_FROM) - Origin
		if convUID != "" {
			edgesToCreate = append(edgesToCreate, graph.EdgeInput{
				FromUID: entityUID.UID,
				ToUID:   convUID,
				Type:    "derived_from",
				Status:  graph.EdgeStatusCurrent,
			})
		}

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

// PersistEntities persists a batch of entities to DGraph
func (p *IngestionPipeline) PersistEntities(ctx context.Context, namespace, userID, conversationID string, entities []graph.ExtractedEntity) error {
	return p.processBatchedEntities(ctx, namespace, userID, conversationID, entities)
}

// PersistChunks persists document chunks to Qdrant
func (p *IngestionPipeline) PersistChunks(ctx context.Context, namespace, docID string, chunks []graph.DocumentChunk) error {
	if p.vectorIndex == nil {
		return fmt.Errorf("vector index is not initialized")
	}

	for _, chunk := range chunks {
		// UID: chunk_{docID}_{index}
		uid := fmt.Sprintf("chunk_%s_%d", docID, chunk.ChunkIndex)

		// Metadata for hybrid retrieval
		metadata := map[string]interface{}{
			"text":        chunk.Text,
			"page_number": chunk.PageNumber,
			"chunk_index": chunk.ChunkIndex,
			"source_id":   docID,
			"type":        "chunk",
		}

		// Store in Qdrant (using vectorIndex.Store)
		if err := p.vectorIndex.Store(ctx, namespace, uid, chunk.Embedding, metadata); err != nil {
			// Log error but continue with other chunks
			p.logger.Error("Failed to persist chunk",
				zap.String("uid", uid),
				zap.Error(err))
		}
	}
	return nil
}

// cacheRecentContext caches the recent conversation context in Redis
func (p *IngestionPipeline) cacheRecentContext(ctx context.Context, event *graph.TranscriptEvent) error {
	// Safety check for nil Redis client
	if p.redisClient == nil {
		p.logger.Debug("Redis client is nil, skipping context caching")
		return nil
	}

	// Use Namespace for context key if available, else user ID
	ns := event.Namespace
	if ns == "" {
		ns = fmt.Sprintf("user_%s", event.UserID)
	}
	key := fmt.Sprintf("context:%s:recent", ns)

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

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
