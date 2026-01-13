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
	"github.com/reflective-memory-kernel/internal/policy"
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
		ActivationDecayRate:    0.05, // 5% decay per day
		MinReflectionBatch:     10,
		MaxReflectionBatch:     100,
		IngestionBatchSize:     50,
		IngestionFlushInterval: 5 * time.Second,
		WisdomBatchSize:        5,
		WisdomFlushInterval:    5 * time.Second,
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

	// Policy Manager
	policyManager *policy.PolicyManager

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
func (k *Kernel) EnsureUserNode(ctx context.Context, username, role string) error {
	return k.graphClient.EnsureUserNode(ctx, username, role)
}

// RemoveGroupMember removes a user from a group
func (k *Kernel) RemoveGroupMember(ctx context.Context, groupID, username string) error {
	return k.graphClient.RemoveGroupMember(ctx, groupID, username)
}

// DeleteGroup deletes a group
func (k *Kernel) DeleteGroup(ctx context.Context, groupID, userID string) error {
	return k.graphClient.DeleteGroup(ctx, groupID, userID)
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

// GetWorkspaceSentInvitations gets all pending invitations sent by a workspace
func (k *Kernel) GetWorkspaceSentInvitations(ctx context.Context, workspaceNS string) ([]graph.WorkspaceInvitation, error) {
	return k.graphClient.GetWorkspaceSentInvitations(ctx, workspaceNS)
}

// CreateShareLink creates a shareable link for a workspace
func (k *Kernel) CreateShareLink(ctx context.Context, workspaceNS, creatorID string, maxUses int, expiresAt *time.Time) (*graph.ShareLink, error) {
	return k.graphClient.CreateShareLink(ctx, workspaceNS, creatorID, maxUses, expiresAt)
}

// JoinViaShareLink joins a workspace using a share link
// SECURITY: Uses distributed Redis lock to prevent race condition on share link usage
func (k *Kernel) JoinViaShareLink(ctx context.Context, token, userID string) (*graph.ShareLink, error) {
	if k.redisClient == nil {
		// No Redis available - fall back to non-locked version (may have race conditions)
		k.logger.Warn("Redis not available for share link locking - race conditions possible")
		return k.graphClient.JoinViaShareLink(ctx, token, userID)
	}

	// CRITICAL: Use distributed lock to prevent race conditions on concurrent share link joins
	// This prevents multiple users from simultaneously passing the usage limit check
	// SECURITY: Adaptive lock with 30s timeout instead of 10s for resilience
	lockKey := fmt.Sprintf("lock:sharelink:%s", token)
	lockAcquired, err := k.redisClient.SetNX(ctx, lockKey, "1", 30*time.Second).Result()
	if err != nil {
		k.logger.Error("Failed to acquire share link lock", zap.Error(err))
		return nil, fmt.Errorf("share link lock unavailable: %w", err)
	}
	if !lockAcquired {
		return nil, fmt.Errorf("share link is being processed by another request - please try again")
	}

	// Ensure lock is released when done
	defer func() {
		if delCmd := k.redisClient.Del(ctx, lockKey); delCmd.Err() != nil {
			k.logger.Warn("Failed to release share link lock", zap.Error(delCmd.Err()))
		}
	}()

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
// SECURITY: Namespace is required for isolation between tenants/workspaces
func (k *Kernel) StoreInHotCache(userID, namespace, query, response, convID string) error {
	if k.hotCache == nil {
		k.logger.Debug("Hot cache not initialized, skipping store")
		return nil // Not an error - hot cache is optional
	}
	return k.hotCache.Store(userID, namespace, query, response, convID)
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
	// Custom activation config
	activationCfg := graph.DefaultActivationConfig()
	activationCfg.DecayRate = k.config.ActivationDecayRate

	reflectionCfg := reflection.Config{
		GraphClient:        k.graphClient,
		QueryBuilder:       k.queryBuilder,
		RedisClient:        k.redisClient,
		AIServicesURL:      k.config.AIServicesURL,
		ActivationConfig:   activationCfg,
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
	k.vectorIndex = NewVectorIndex(k.config.QdrantURL, DefaultCollectionName, k.logger)
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
		k.vectorIndex,
		k.config.IngestionBatchSize,
		k.config.IngestionFlushInterval,
		k.logger,
	)

	// Initialize Policy Manager
	// Policy enforcement re-enabled after verifying same-namespace access works
	policyConfig := policy.PolicyManagerConfig{
		Enabled:          true, // RE-ENABLED: Namespace isolation verified working
		AuditEnabled:     true,
		RateLimitEnabled: true,
	}
	k.policyManager = policy.NewPolicyManager(policyConfig, k.graphClient, k.natsConn, k.redisClient, k.logger)

	// Initialize consultation handler with Hybrid RAG and Hot Cache support
	k.consultationHandler = NewConsultationHandler(
		k.graphClient,
		k.queryBuilder,
		k.redisClient,
		k.vectorIndex,
		k.localEmbedder,
		k.hotCache,
		k.policyManager,
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
			k.logger.Error("Panic in ingestion loop", zap.Any("panic", r), zap.Stack("stacktrace"))
		}
	}()

	k.logger.Info("Starting ingestion loop")

	if k.jetStream == nil {
		k.logger.Error("JetStream context is nil, cannot subscribe")
		return
	}

	// Create or get dead-letter stream for failed messages
	deadLetterStream := "transcripts_dead"
	if _, err := k.jetStream.StreamInfo(deadLetterStream); err != nil {
		// Stream doesn't exist, create it
		_, err = k.jetStream.AddStream(&nats.StreamConfig{
			Name:     deadLetterStream,
			Subjects: []string{"transcripts_dead.>"},
			Retention: nats.LimitsPolicy,
			MaxAge:   7 * 24 * time.Hour, // Keep dead letters for 7 days
		})
		if err != nil {
			k.logger.Error("Failed to create dead-letter stream", zap.Error(err))
		} else {
			k.logger.Info("Created dead-letter stream", zap.String("stream", deadLetterStream))
		}
	}

	// Track retry counts for messages
	retryCount := make(map[string]int)
	retryCountMu := sync.Mutex{}

	// Configure retry policy
	const (
		maxRetries    = 3                // Maximum retry attempts
		baseDelay     = 1 * time.Second  // Base delay for exponential backoff
		maxDelay      = 30 * time.Second // Maximum delay between retries
	)

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
			msg.NakWithDelay(30 * time.Second) // Delay before retry
			return
		}

		// Process message with retry logic
		err := k.ingestionPipeline.Process(k.ctx, msg.Data)
		if err != nil {
			// Check retry count for this message
			msgID := string(msg.Header.Get("Nats-Msg-Id"))
			if msgID == "" {
				msgID = fmt.Sprintf("%s_%d", msg.Subject, time.Now().UnixNano())
			}

			retryCountMu.Lock()
			count := retryCount[msgID]
			count++
			retryCount[msgID] = count
			retryCountMu.Unlock()

			k.logger.Error("Failed to process transcript",
				zap.Error(err),
				zap.String("subject", msg.Subject),
				zap.Int("retry_attempt", count))

			if count < maxRetries {
				// Calculate exponential backoff delay
				delay := baseDelay * time.Duration(1<<uint(count-1))
				if delay > maxDelay {
					delay = maxDelay
				}

				k.logger.Info("Retrying message with backoff",
					zap.String("msg_id", msgID),
					zap.Duration("delay", delay),
					zap.Int("retry", count))

				// Nak with delay for retry
				msg.NakWithDelay(delay)
			} else {
				// Max retries reached, send to dead-letter queue
				k.logger.Error("Max retries exceeded, sending to dead-letter queue",
					zap.String("msg_id", msgID),
					zap.Int("max_retries", maxRetries),
					zap.Error(err))

				// Publish to dead-letter stream with metadata
				deadLetterMsg := nats.NewMsg("transcripts_dead."+msg.Subject)
				deadLetterMsg.Header.Set("Original-Subject", msg.Subject)
				deadLetterMsg.Header.Set("Error", err.Error())
				deadLetterMsg.Header.Set("Retry-Count", fmt.Sprintf("%d", count))
				deadLetterMsg.Header.Set("Failed-At", time.Now().Format(time.RFC3339))
				deadLetterMsg.Data = msg.Data

				if _, pubErr := k.jetStream.PublishMsg(deadLetterMsg); pubErr != nil {
					k.logger.Error("Failed to publish to dead-letter queue", zap.Error(pubErr))
				}

				// Clean up retry count and ack original message
				retryCountMu.Lock()
				delete(retryCount, msgID)
				retryCountMu.Unlock()
				msg.Ack()
			}
		} else {
			// Success - clean up retry count
			msgID := string(msg.Header.Get("Nats-Msg-Id"))
			if msgID != "" {
				retryCountMu.Lock()
				delete(retryCount, msgID)
				retryCountMu.Unlock()
			}

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

	defer func() {
		if r := recover(); r != nil {
			k.logger.Error("Panic in reflection loop", zap.Any("panic", r), zap.Stack("stacktrace"))
		}
	}()

	k.logger.Info("Starting reflection loop",
		zap.Duration("interval", k.config.ReflectionInterval))

	ticker := time.NewTicker(k.config.ReflectionInterval)
	defer ticker.Stop()

	// SECURITY: Add timeout to prevent reflection cycles from hanging indefinitely
	const reflectionTimeout = 5 * time.Minute

	for {
		select {
		case <-k.ctx.Done():
			k.logger.Info("Reflection loop stopped")
			return
		case <-ticker.C:
			k.logger.Debug("Running reflection cycle")
			// Create a context with timeout for each reflection cycle
			ctx, cancel := context.WithTimeout(k.ctx, reflectionTimeout)
			func() {
				defer cancel()
				if err := k.reflectionEngine.RunCycle(ctx); err != nil {
					k.logger.Error("Reflection cycle failed", zap.Error(err))
				}
			}()
		}
	}
}

// runDecayLoop periodically applies activation decay to all nodes
func (k *Kernel) runDecayLoop() {
	defer k.wg.Done()

	defer func() {
		if r := recover(); r != nil {
			k.logger.Error("Panic in decay loop", zap.Any("panic", r), zap.Stack("stacktrace"))
		}
	}()

	k.logger.Info("Starting decay loop")

	// Run decay every 1 hour (production setting)
	// Decay rate is applied once per hour, targeting ~5% loss per day
	ticker := time.NewTicker(1 * time.Hour)
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

// PersistEntities persists extracted entities to the graph
func (k *Kernel) PersistEntities(ctx context.Context, namespace, userID, conversationID string, entities []graph.ExtractedEntity) error {
	return k.ingestionPipeline.PersistEntities(ctx, namespace, userID, conversationID, entities)
}

// PersistChunks persists document chunks to Qdrant
func (k *Kernel) PersistChunks(ctx context.Context, namespace, docID string, chunks []graph.DocumentChunk) error {
	return k.ingestionPipeline.PersistChunks(ctx, namespace, docID, chunks)
}

// SearchNodes delegates to the graph client to perform a node search
func (k *Kernel) SearchNodes(ctx context.Context, namespace, query string) ([]graph.Node, error) {
	return k.graphClient.SearchNodes(ctx, query, namespace)
}
