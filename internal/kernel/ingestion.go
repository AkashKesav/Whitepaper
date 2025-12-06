// Package kernel provides the ingestion pipeline for the Memory Kernel.
// This implements Phase 1 of the three-phase loop: receiving transcript events
// from the Front-End Agent and writing them to the Knowledge Graph.
package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/graph"
)

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
	p.logger.Debug("Ingesting transcript event",
		zap.String("conversation_id", event.ConversationID),
		zap.String("user_id", event.UserID))

	// Step 1: Extract entities using the AI service
	entities, err := p.extractEntities(ctx, event)
	if err != nil {
		p.logger.Warn("Entity extraction failed, using basic extraction",
			zap.Error(err))
		// Fall back to basic extraction
		entities = p.basicEntityExtraction(event)
	}
	event.ExtractedEntities = entities

	// Step 2: Create or update nodes for each entity
	for _, entity := range entities {
		if err := p.processEntity(ctx, event.UserID, event.ConversationID, entity); err != nil {
			p.logger.Error("Failed to process entity",
				zap.String("entity", entity.Name),
				zap.Error(err))
			continue
		}
	}

	// Step 3: Cache recent context in Redis for fast access
	if err := p.cacheRecentContext(ctx, event); err != nil {
		p.logger.Warn("Failed to cache context", zap.Error(err))
	}

	p.logger.Info("Transcript event ingested successfully",
		zap.String("conversation_id", event.ConversationID),
		zap.Int("entities_extracted", len(entities)))

	return nil
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

	client := &http.Client{Timeout: 30 * time.Second}
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

// processEntity creates or updates a node and its relationships in the graph
func (p *IngestionPipeline) processEntity(ctx context.Context, userID, conversationID string, entity graph.ExtractedEntity) error {
	// Check if entity already exists
	existingNode, err := p.graphClient.FindNodeByName(ctx, entity.Name, entity.Type)
	if err != nil {
		return err
	}

	var nodeUID string
	if existingNode != nil {
		// Update existing node - boost activation
		nodeUID = existingNode.UID
		if err := p.graphClient.IncrementAccessCount(ctx, nodeUID, graph.DefaultActivationConfig()); err != nil {
			p.logger.Warn("Failed to increment access count", zap.Error(err))
		}
	} else {
		// Create new node
		node := &graph.Node{
			Type:                 entity.Type,
			Name:                 entity.Name,
			Attributes:           entity.Attributes,
			SourceConversationID: conversationID,
			Activation:           0.5, // Start at 50% activation
			Confidence:           0.8, // Default confidence
		}

		nodeUID, err = p.graphClient.CreateNode(ctx, node)
		if err != nil {
			return fmt.Errorf("failed to create node: %w", err)
		}
		p.logger.Debug("Created new node",
			zap.String("uid", nodeUID),
			zap.String("name", entity.Name),
			zap.String("type", string(entity.Type)))
	}

	// Process relationships
	for _, relation := range entity.Relations {
		if err := p.processRelation(ctx, nodeUID, relation); err != nil {
			p.logger.Warn("Failed to process relation",
				zap.String("from", entity.Name),
				zap.String("to", relation.TargetName),
				zap.Error(err))
		}
	}

	return nil
}

// processRelation creates or updates a relationship between nodes
func (p *IngestionPipeline) processRelation(ctx context.Context, fromUID string, relation graph.ExtractedRelation) error {
	// Find or create the target node
	targetNode, err := p.graphClient.FindNodeByName(ctx, relation.TargetName, relation.TargetType)
	if err != nil {
		return err
	}

	var targetUID string
	if targetNode != nil {
		targetUID = targetNode.UID
		// Boost activation on related node
		if err := p.graphClient.IncrementAccessCount(ctx, targetUID, graph.DefaultActivationConfig()); err != nil {
			p.logger.Warn("Failed to boost related node", zap.Error(err))
		}
	} else {
		// Create new target node
		node := &graph.Node{
			Type:       relation.TargetType,
			Name:       relation.TargetName,
			Activation: 0.5,
			Confidence: 0.7,
		}
		targetUID, err = p.graphClient.CreateNode(ctx, node)
		if err != nil {
			return fmt.Errorf("failed to create target node: %w", err)
		}
	}

	// Create the edge
	if err := p.graphClient.CreateEdge(ctx, fromUID, targetUID, relation.Type, graph.EdgeStatusCurrent); err != nil {
		return fmt.Errorf("failed to create edge: %w", err)
	}

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
	_, err = js.Publish(subject, data)
	return err
}
