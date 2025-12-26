// Package kernel implements the Memory Kernel - the "subconscious" of the system.
// It handles ingestion, reflection, and consultation in a continuous loop.
package kernel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/ai/local"
	"github.com/reflective-memory-kernel/internal/graph"
	"github.com/reflective-memory-kernel/internal/kernel/wisdom"
	"github.com/reflective-memory-kernel/internal/memory"
	"github.com/reflective-memory-kernel/internal/reflection"
)

// Config holds the Memory Kernel configuration
type Config struct {
	// DGraph configuration
	DGraphAddress string

	// NATS configuration
	NATSAddress string

	// Redis configuration
	RedisAddress  string
	RedisPassword string
	RedisDB       int

	// AI Services configuration
	AIServicesURL string

	// Qdrant vector database configuration
	QdrantURL string

	// Reflection configuration
	ReflectionInterval  time.Duration
	ActivationDecayRate float64
	MinReflectionBatch  int
	MaxReflectionBatch  int

	// Ingestion configuration
	IngestionBatchSize     int
	IngestionFlushInterval time.Duration

	// Wisdom configuration
	WisdomBatchSize     int
	WisdomFlushInterval time.Duration
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		DGraphAddress:          "localhost:9080",
		NATSAddress:            "nats://localhost:4222",
		RedisAddress:           "localhost:6379",
		RedisPassword:          "",
		RedisDB:                0,
		AIServicesURL:          "http://localhost:8000",
		QdrantURL:              "http://localhost:6333",
		ReflectionInterval:     5 * time.Minute,
		ActivationDecayRate:    0.05,
		MinReflectionBatch:     10,
		MaxReflectionBatch:     100,
		IngestionBatchSize:     50,
		IngestionFlushInterval: 10 * time.Second,
		WisdomBatchSize:        50,
		WisdomFlushInterval:    30 * time.Second,
	}
}

// Kernel is the Memory Kernel - the persistent, asynchronous "subconscious" agent
type Kernel struct {
	config Config
	logger *zap.Logger

	// Data layer
	graphClient  *graph.Client
	queryBuilder *graph.QueryBuilder
	natsConn     *nats.Conn
	jetStream    nats.JetStreamContext
	redisClient  *redis.Client

	// Reflection engine
	reflectionEngine *reflection.Engine

	// Ingestion pipeline
	ingestionPipeline *IngestionPipeline
	localEmbedder     local.LocalEmbedder

	// Wisdom manager (Cold Path)
	wisdomManager *wisdom.WisdomManager

	// Vector index for Hybrid RAG
	vectorIndex *VectorIndex

	// Hot Cache for recent messages (Hot Path)
	hotCache *memory.HotCache

	// Consultation handler
	consultationHandler *ConsultationHandler

	// Control
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.RWMutex
	isRunning bool
}

// New creates a new Memory Kernel
func New(cfg Config, logger *zap.Logger) (*Kernel, error) {
	ctx, cancel := context.WithCancel(context.Background())

	k := &Kernel{
		config: cfg,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	return k, nil
}

// CreateGroup creates a new group
func (k *Kernel) CreateGroup(ctx context.Context, name, description, ownerID string) (string, error) {
	return k.graphClient.CreateGroup(ctx, name, description, ownerID)
}

// ListUserGroups returns groups the user is a member of
func (k *Kernel) ListUserGroups(ctx context.Context, userID string) ([]graph.Group, error) {
	return k.graphClient.ListUserGroups(ctx, userID)
}

// IsGroupAdmin checks if a user is an admin of a group
func (k *Kernel) IsGroupAdmin(ctx context.Context, groupNamespace, userID string) (bool, error) {
	return k.graphClient.IsGroupAdmin(ctx, groupNamespace, userID)
}

// AddGroupMember adds a user to a group
func (k *Kernel) AddGroupMember(ctx context.Context, groupID, username string) error {
	return k.graphClient.AddGroupMember(ctx, groupID, username)
}

// EnsureUserNode creates a User node in DGraph if it doesn't exist
func (k *Kernel) EnsureUserNode(ctx context.Context, username string) error {
	return k.graphClient.EnsureUserNode(ctx, username)
}

// RemoveGroupMember removes a user from a group
func (k *Kernel) RemoveGroupMember(ctx context.Context, groupID, username string) error {
	return k.graphClient.RemoveGroupMember(ctx, groupID, username)
}

// DeleteGroup deletes a group
func (k *Kernel) DeleteGroup(ctx context.Context, groupID string) error {
	return k.graphClient.DeleteGroup(ctx, groupID)
}

// ShareToGroup shares a conversation with a group
func (k *Kernel) ShareToGroup(ctx context.Context, conversationID, groupID string) error {
	return k.graphClient.ShareToGroup(ctx, conversationID, groupID)
}

// ============================================================================
// WORKSPACE COLLABORATION METHODS
// ============================================================================

// InviteToWorkspace invites a user to join a workspace
func (k *Kernel) InviteToWorkspace(ctx context.Context, workspaceNS, inviterID, inviteeUsername, role string) (*graph.WorkspaceInvitation, error) {
	return k.graphClient.InviteToWorkspace(ctx, workspaceNS, inviterID, inviteeUsername, role)
}

// AcceptInvitation accepts a pending invitation
func (k *Kernel) AcceptInvitation(ctx context.Context, invitationUID, userID string) error {
	return k.graphClient.AcceptInvitation(ctx, invitationUID, userID)
}

// DeclineInvitation declines a pending invitation
func (k *Kernel) DeclineInvitation(ctx context.Context, invitationUID, userID string) error {
	return k.graphClient.DeclineInvitation(ctx, invitationUID, userID)
}

// GetPendingInvitations gets all pending invitations for a user
func (k *Kernel) GetPendingInvitations(ctx context.Context, userID string) ([]graph.WorkspaceInvitation, error) {
	return k.graphClient.GetPendingInvitations(ctx, userID)
}

// CreateShareLink creates a shareable link for a workspace
func (k *Kernel) CreateShareLink(ctx context.Context, workspaceNS, creatorID string, maxUses int, expiresAt *time.Time) (*graph.ShareLink, error) {
	return k.graphClient.CreateShareLink(ctx, workspaceNS, creatorID, maxUses, expiresAt)
}

// JoinViaShareLink joins a workspace using a share link
func (k *Kernel) JoinViaShareLink(ctx context.Context, token, userID string) (*graph.ShareLink, error) {
	return k.graphClient.JoinViaShareLink(ctx, token, userID)
}

// RevokeShareLink revokes a share link
func (k *Kernel) RevokeShareLink(ctx context.Context, token, userID string) error {
	return k.graphClient.RevokeShareLink(ctx, token, userID)
}

// GetWorkspaceMembers gets all members of a workspace
func (k *Kernel) GetWorkspaceMembers(ctx context.Context, workspaceNS string) ([]graph.WorkspaceMember, error) {
	return k.graphClient.GetWorkspaceMembers(ctx, workspaceNS)
}

// IsWorkspaceMember checks if a user is a member of a workspace
func (k *Kernel) IsWorkspaceMember(ctx context.Context, workspaceNS, userID string) (bool, error) {
	return k.graphClient.IsWorkspaceMember(ctx, workspaceNS, userID)
}

// GetGraphClient returns the graph client for external use (e.g., Pre-Cortex)
func (k *Kernel) GetGraphClient() *graph.Client {
	return k.graphClient
}

// StoreInHotCache stores a conversation turn in the hot cache for immediate retrieval
// This is the Hot Path - enables instant context for follow-up questions
func (k *Kernel) StoreInHotCache(userID, query, response, convID string) error {
	if k.hotCache == nil {
		k.logger.Debug("Hot cache not initialized, skipping store")
		return nil // Not an error - hot cache is optional
	}
	return k.hotCache.Store(userID, query, response, convID)
}

// Start initializes and starts all kernel components
func (k *Kernel) Start() error {
	k.mu.Lock()
	if k.isRunning {
		k.mu.Unlock()
		return nil
	}
	k.mu.Unlock()

	k.logger.Info("Starting Memory Kernel...")

	// Initialize DGraph client
	graphCfg := graph.ClientConfig{
		Address:        k.config.DGraphAddress,
		MaxRetries:     10,
		RetryInterval:  3 * time.Second,
		RequestTimeout: 30 * time.Second,
	}
	graphClient, err := graph.NewClient(k.ctx, graphCfg, k.logger)
	if err != nil {
		return err
	}
	k.graphClient = graphClient
	k.queryBuilder = graph.NewQueryBuilder(graphClient)

	// Initialize NATS connection with JetStream
	natsConn, err := nats.Connect(k.config.NATSAddress,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return err
	}
	k.natsConn = natsConn

	js, err := natsConn.JetStream()
	if err != nil {
		return err
	}
	k.jetStream = js

	// Create stream for transcript events
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "TRANSCRIPTS",
		Subjects: []string{"transcripts.*"},
		Storage:  nats.FileStorage,
		MaxAge:   24 * time.Hour * 30, // 30 days retention
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		k.logger.Warn("Failed to create NATS stream", zap.Error(err))
	}

	// Initialize Redis client
	k.redisClient = redis.NewClient(&redis.Options{
		Addr:     k.config.RedisAddress,
		Password: k.config.RedisPassword,
		DB:       k.config.RedisDB,
	})
	if err := k.redisClient.Ping(k.ctx).Err(); err != nil {
		return err
	}

	// Initialize reflection engine
	reflectionCfg := reflection.Config{
		GraphClient:        k.graphClient,
		QueryBuilder:       k.queryBuilder,
		RedisClient:        k.redisClient,
		AIServicesURL:      k.config.AIServicesURL,
		ActivationConfig:   graph.DefaultActivationConfig(),
		ReflectionInterval: k.config.ReflectionInterval,
		MinBatchSize:       k.config.MinReflectionBatch,
		MaxBatchSize:       k.config.MaxReflectionBatch,
	}
	k.reflectionEngine = reflection.NewEngine(reflectionCfg, k.logger)

	// Initialize Local AI (Hot Path) - Using Ollama for embeddings
	// Must be initialized before WisdomManager for Hybrid RAG
	ollamaEmbedder := local.NewOllamaEmbedder("", "") // Uses OLLAMA_URL env var

	// Try to ensure the embedding model is available
	if err := ollamaEmbedder.EnsureModel(); err != nil {
		k.logger.Warn("Failed to ensure Ollama embedding model (will retry on first use)", zap.Error(err))
	}
	k.localEmbedder = ollamaEmbedder
	k.logger.Info("Ollama embedder initialized (Hot Path enabled)")

	// Initialize Vector Index (Qdrant) for Hybrid RAG
	// Must be initialized before WisdomManager for embedding storage
	k.vectorIndex = NewVectorIndex(k.config.QdrantURL, k.logger)
	if err := k.vectorIndex.Initialize(k.ctx); err != nil {
		k.logger.Warn("Failed to initialize Qdrant vector index (will retry on first use)", zap.Error(err))
	} else {
		k.logger.Info("Qdrant vector index initialized (Hybrid RAG enabled)")
	}

	// Initialize Wisdom Manager (Cold Path) with Hybrid RAG support
	wisdomCfg := wisdom.Config{
		BatchSize:     k.config.WisdomBatchSize,
		FlushInterval: k.config.WisdomFlushInterval,
		AIServiceURL:  k.config.AIServicesURL,
	}
	k.wisdomManager = wisdom.NewManager(wisdomCfg, k.graphClient, k.localEmbedder, k.vectorIndex, k.logger)

	// Initialize ingestion pipeline
	k.ingestionPipeline = NewIngestionPipeline(
		k.graphClient,
		k.jetStream,
		k.redisClient,
		k.config.AIServicesURL,
		k.localEmbedder,
		k.wisdomManager,
		k.config.IngestionBatchSize,
		k.config.IngestionFlushInterval,
		k.logger,
	)

	// Initialize consultation handler with Hybrid RAG support
	k.consultationHandler = NewConsultationHandler(
		k.graphClient,
		k.queryBuilder,
		k.redisClient,
		k.vectorIndex,
		k.localEmbedder,
		k.config.AIServicesURL,
		k.logger,
	)

	// Start background processes
	k.wg.Add(3)
	go k.runIngestionLoop()
	go k.runReflectionLoop()
	go k.runDecayLoop()

	k.wisdomManager.Start()

	k.mu.Lock()
	k.isRunning = true
	k.mu.Unlock()

	k.logger.Info("Memory Kernel started successfully",
		zap.String("dgraph", k.config.DGraphAddress),
		zap.String("nats", k.config.NATSAddress),
		zap.Duration("reflection_interval", k.config.ReflectionInterval))

	return nil
}

// Stop gracefully shuts down the kernel
func (k *Kernel) Stop() error {
	k.mu.Lock()
	if !k.isRunning {
		k.mu.Unlock()
		return nil
	}
	k.mu.Unlock()

	k.logger.Info("Stopping Memory Kernel...")

	// Signal all goroutines to stop
	k.cancel()

	// Wait for all goroutines to finish
	k.wg.Wait()

	// Close connections
	if k.natsConn != nil {
		k.natsConn.Close()
	}
	if k.redisClient != nil {
		k.redisClient.Close()
	}
	if k.graphClient != nil {
		k.graphClient.Close()
	}
	if k.localEmbedder != nil {
		k.localEmbedder.Close()
	}

	if k.wisdomManager != nil {
		k.wisdomManager.Stop()
	}

	k.mu.Lock()
	k.isRunning = false
	k.mu.Unlock()

	k.logger.Info("Memory Kernel stopped")
	return nil
}

// runIngestionLoop continuously processes incoming transcript events
func (k *Kernel) runIngestionLoop() {
	defer k.wg.Done()

	// Add panic recovery
	defer func() {
		if r := recover(); r != nil {
			k.logger.Error("Panic in ingestion loop", zap.Any("panic", r))
		}
	}()

	k.logger.Info("Starting ingestion loop")

	if k.jetStream == nil {
		k.logger.Error("JetStream context is nil, cannot subscribe")
		return
	}

	// Subscribe to transcript events
	sub, err := k.jetStream.Subscribe("transcripts.*", func(msg *nats.Msg) {
		// Add panic recovery for the callback goroutine
		defer func() {
			if r := recover(); r != nil {
				k.logger.Error("Panic in NATS callback", zap.Any("panic", r),
					zap.Stack("stacktrace"))
			}
		}()

		k.logger.Info("=== RECEIVED NATS MESSAGE ===",
			zap.String("subject", msg.Subject),
			zap.Int("data_len", len(msg.Data)))

		// Safety check for nil ingestion pipeline
		if k.ingestionPipeline == nil {
			k.logger.Error("Ingestion pipeline is nil, cannot process message")
			msg.Nak()
			return
		}

		if err := k.ingestionPipeline.Process(k.ctx, msg.Data); err != nil {
			k.logger.Error("Failed to process transcript",
				zap.Error(err),
				zap.String("subject", msg.Subject))
			// Nak to retry later
			msg.Nak()
		} else {
			k.logger.Info("Successfully processed transcript",
				zap.String("subject", msg.Subject))
			msg.Ack()
		}
	}, nats.Durable("kernel-ingestion-v2"), nats.ManualAck())

	if err != nil {
		k.logger.Error("Failed to subscribe to transcripts", zap.Error(err))
		return
	}
	k.logger.Info("NATS subscription active", zap.String("subject", "transcripts.*"))
	defer sub.Unsubscribe()

	// Wait for shutdown signal
	<-k.ctx.Done()
	k.logger.Info("Ingestion loop stopped")
}

// runReflectionLoop periodically runs the reflection/rumination process
func (k *Kernel) runReflectionLoop() {
	defer k.wg.Done()

	k.logger.Info("Starting reflection loop",
		zap.Duration("interval", k.config.ReflectionInterval))

	ticker := time.NewTicker(k.config.ReflectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-k.ctx.Done():
			k.logger.Info("Reflection loop stopped")
			return
		case <-ticker.C:
			k.logger.Debug("Running reflection cycle")
			if err := k.reflectionEngine.RunCycle(k.ctx); err != nil {
				k.logger.Error("Reflection cycle failed", zap.Error(err))
			}
		}
	}
}

// runDecayLoop periodically applies activation decay to all nodes
func (k *Kernel) runDecayLoop() {
	defer k.wg.Done()

	k.logger.Info("Starting decay loop")

	// Run decay every 1 minute (for testing - originally 1 hour)
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-k.ctx.Done():
			k.logger.Info("Decay loop stopped")
			return
		case <-ticker.C:
			k.logger.Debug("Running activation decay")
			if err := k.reflectionEngine.ApplyDecay(k.ctx); err != nil {
				k.logger.Error("Decay cycle failed", zap.Error(err))
			}
		}
	}
}

// Consult handles a consultation request from the Front-End Agent
func (k *Kernel) Consult(ctx context.Context, req *graph.ConsultationRequest) (*graph.ConsultationResponse, error) {
	return k.consultationHandler.Handle(ctx, req)
}

// Speculate performs a pre-fetch for a partial query
func (k *Kernel) Speculate(ctx context.Context, req *graph.ConsultationRequest) error {
	return k.consultationHandler.Speculate(ctx, req)
}

// IngestTranscript manually ingests a transcript (for testing)
func (k *Kernel) IngestTranscript(ctx context.Context, event *graph.TranscriptEvent) error {
	return k.ingestionPipeline.Ingest(ctx, event)
}

// TriggerReflection manually triggers a reflection cycle (for testing)
func (k *Kernel) TriggerReflection(ctx context.Context) error {
	return k.reflectionEngine.RunCycle(ctx)
}

// GetStats returns kernel statistics
func (k *Kernel) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count nodes by type
	for _, nodeType := range []graph.NodeType{
		graph.NodeTypeEntity,
		graph.NodeTypeFact,
		graph.NodeTypeInsight,
		graph.NodeTypePattern,
	} {
		count, err := k.queryBuilder.CountNodes(ctx, nodeType)
		if err != nil {
			k.logger.Warn("Failed to count nodes", zap.String("type", string(nodeType)), zap.Error(err))
			continue
		}
		stats[string(nodeType)+"_count"] = count
	}

	// Get high activation nodes
	highActivation, err := k.queryBuilder.GetHighActivationNodes(ctx, "", 0.7, 10)
	if err == nil {
		stats["high_activation_nodes"] = len(highActivation)
	}

	// Get recent insights
	insights, err := k.queryBuilder.GetInsights(ctx, "", 10)
	if err == nil {
		stats["recent_insights"] = len(insights)
	}

	// Get patterns
	patterns, err := k.queryBuilder.GetPatterns(ctx, "", 0.5, 10)
	if err == nil {
		stats["active_patterns"] = len(patterns)
	}

	// Get ingestion pipeline stats
	if k.ingestionPipeline != nil {
		ingestionStats := k.ingestionPipeline.GetStats()
		stats["ingestion"] = map[string]interface{}{
			"total_processed":        ingestionStats.TotalProcessed,
			"total_errors":           ingestionStats.TotalErrors,
			"total_entities_created": ingestionStats.TotalEntitiesCreated,
			"last_duration_ms":       ingestionStats.LastDurationMs,
			"avg_duration_ms":        ingestionStats.AvgDurationMs,
			"last_extraction_ms":     ingestionStats.LastExtractionMs,
			"last_dgraph_write_ms":   ingestionStats.LastDgraphWriteMs,
			"last_processed_at":      ingestionStats.LastProcessedAt,
		}
	}

	return stats, nil
}

// IngestEvent allows direct ingestion of events (Zero-Copy path)
func (k *Kernel) IngestEvent(ctx context.Context, event *graph.TranscriptEvent) error {
	if !k.isRunning {
		return fmt.Errorf("kernel is not running")
	}
	// Delegate to pipeline's direct ingest
	return k.ingestionPipeline.IngestDirect(ctx, event)
}
