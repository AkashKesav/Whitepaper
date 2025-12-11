// Package kernel implements the Memory Kernel - the "subconscious" of the system.
// It handles ingestion, reflection, and consultation in a continuous loop.
package kernel

import (
	"context"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/graph"
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

	// Reflection configuration
	ReflectionInterval  time.Duration
	ActivationDecayRate float64
	MinReflectionBatch  int
	MaxReflectionBatch  int

	// Ingestion configuration
	IngestionBatchSize     int
	IngestionFlushInterval time.Duration
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
		ReflectionInterval:     5 * time.Minute,
		ActivationDecayRate:    0.05,
		MinReflectionBatch:     10,
		MaxReflectionBatch:     100,
		IngestionBatchSize:     50,
		IngestionFlushInterval: 10 * time.Second,
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

	// Initialize ingestion pipeline
	k.ingestionPipeline = NewIngestionPipeline(
		k.graphClient,
		k.jetStream,
		k.redisClient,
		k.config.AIServicesURL,
		k.config.IngestionBatchSize,
		k.config.IngestionFlushInterval,
		k.logger,
	)

	// Initialize consultation handler
	k.consultationHandler = NewConsultationHandler(
		k.graphClient,
		k.queryBuilder,
		k.redisClient,
		k.config.AIServicesURL,
		k.logger,
	)

	// Start background processes
	k.wg.Add(3)
	go k.runIngestionLoop()
	go k.runReflectionLoop()
	go k.runDecayLoop()

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
