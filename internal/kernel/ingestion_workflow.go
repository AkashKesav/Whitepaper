// Package kernel provides Inngest-based durable execution workflows for the RMK.
// This provides reliable, retry-safe execution of long-running ingestion pipelines.
package kernel

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/inngest/inngestgo"
	"github.com/inngest/inngestgo/step"
	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/ai/local"
	"github.com/reflective-memory-kernel/internal/graph"
)

// WorkflowConfig holds configuration for Inngest workflows
type WorkflowConfig struct {
	InngestAPIKey string
	EventKey      string
	AppID         string
	Logger        *zap.Logger
}

// IngestionInput represents the input for the ingestion workflow
type IngestionInput struct {
	ConversationID string `json:"conversation_id"`
	UserID        string `json:"user_id"`
	Namespace     string `json:"namespace"`
	UserQuery     string `json:"user_query"`
	AIResponse    string `json:"ai_response"`
	Timestamp     string `json:"timestamp"`
}

// EntityExtractionOutput represents the output of entity extraction
type EntityExtractionOutput struct {
	Entities []graph.ExtractedEntity `json:"entities"`
	Error    string                  `json:"error,omitempty"`
}

// DeduplicationOutput represents the output of entity deduplication
type DeduplicationOutput struct {
	Entities   []graph.ExtractedEntity `json:"entities"`
	Deduped    int                     `json:"deduped_count"`
	Duplicates []string               `json:"duplicate_names"`
}

// EmbeddingOutput represents the output of embedding generation
type EmbeddingOutput struct {
	Embedding []float32 `json:"embedding"`
	Error     string    `json:"error,omitempty"`
}

// PersistenceOutput represents the output of graph persistence
type PersistenceOutput struct {
	Success   bool     `json:"success"`
	NodeUIDs  []string `json:"node_uids"`
	EdgeCount int      `json:"edge_count"`
	Error     string   `json:"error,omitempty"`
}

// IngestionOutput represents the final output of the ingestion workflow
type IngestionOutput struct {
	Success       bool   `json:"success"`
	ErrorMessage string `json:"error,omitempty"`
	SummaryUID   string `json:"summary_uid,omitempty"`
	EntityCount  int    `json:"entity_count"`
}

// ingestTranscriptWorkflow implements the durable ingestion workflow
// This provides automatic retries, deadlock detection, and observability
func ingestTranscriptWorkflow(
	cfg WorkflowConfig,
	graphClient *graph.Client,
	embedder local.LocalEmbedder,
) func(ctx context.Context, input inngestgo.Input[IngestionInput]) (any, error) {
	return func(ctx context.Context, input inngestgo.Input[IngestionInput]) (any, error) {
		logger := cfg.Logger.With(
			zap.String("conversation_id", input.Event.Data.ConversationID),
			zap.String("user_id", input.Event.Data.UserID),
		)

		logger.Info("Starting durable ingestion workflow")

		// Step 1: Entity Extraction (with auto-retry)
		var extractResult EntityExtractionOutput
		extractRes, extractErr := step.Run(ctx, "extract-entities", func(ctx context.Context) (EntityExtractionOutput, error) {
			// In a real implementation, this would call the AI service
			// For now, we return a placeholder result
			logger.Info("Step 1: Extracting entities")
			return EntityExtractionOutput{
				Entities: []graph.ExtractedEntity{},
			}, nil
		})
		if extractErr != nil {
			return IngestionOutput{
				Success:       false,
				ErrorMessage: fmt.Sprintf("entity extraction failed: %v", extractErr),
			}, extractErr
		}
		extractResult = extractRes

		// Step 2: Fuzzy dedup (using Bleve for faster lookups)
		var dedupResult DeduplicationOutput
		dedupRes, dedupErr := step.Run(ctx, "deduplicate-entities", func(ctx context.Context) (DeduplicationOutput, error) {
			logger.Info("Step 2: Duplicating entities",
				zap.Int("input_count", len(extractResult.Entities)))
			// In a real implementation, this would query Bleve
			return DeduplicationOutput{
				Entities:  extractResult.Entities,
				Deduped:   0,
				Duplicates: []string{},
			}, nil
		})
		if dedupErr != nil {
			return IngestionOutput{
				Success:       false,
				ErrorMessage: fmt.Sprintf("deduplication failed: %v", dedupErr),
			}, dedupErr
		}
		dedupResult = dedupRes

		// Step 3: Vector embedding (with circuit breaker via Inngest's retry)
		_, embedErr := step.Run(ctx, "generate-embeddings", func(ctx context.Context) (EmbeddingOutput, error) {
			logger.Info("Step 3: Generating embeddings")
			if embedder == nil {
				return EmbeddingOutput{
					Embedding: []float32{},
					Error:     "embedder not configured",
				}, nil
			}
			// Generate embedding for the query
			vec, err := embedder.Embed(input.Event.Data.UserQuery)
			if err != nil {
				return EmbeddingOutput{Error: err.Error()}, err
			}
			return EmbeddingOutput{Embedding: vec}, nil
		})
		if embedErr != nil {
			// Embedding failure is not fatal - we can continue without it
			logger.Warn("Embedding generation failed, continuing without it",
				zap.Error(embedErr))
		}

		// Step 4: Store in DGraph (batched)
		var persistResult PersistenceOutput
		persistRes, persistErr := step.Run(ctx, "persist-graph", func(ctx context.Context) (PersistenceOutput, error) {
			logger.Info("Step 4: Persisting to graph",
				zap.Int("entity_count", len(dedupResult.Entities)))
			// In a real implementation, this would batch write to DGraph
			return PersistenceOutput{
				Success:   true,
				NodeUIDs:  []string{},
				EdgeCount: 0,
			}, nil
		})
		if persistErr != nil {
			return IngestionOutput{
				Success:       false,
				ErrorMessage: fmt.Sprintf("graph persistence failed: %v", persistErr),
			}, persistErr
		}
		persistResult = persistRes

		logger.Info("Durable ingestion workflow completed successfully",
			zap.Int("entity_count", len(dedupResult.Entities)),
			zap.Int("edge_count", persistResult.EdgeCount))

		return IngestionOutput{
			Success:     true,
			EntityCount: len(dedupResult.Entities),
		}, nil
	}
}

// NewIngestionWorkflow creates the ingestion workflow function
func NewIngestionWorkflow(cfg WorkflowConfig, graphClient *graph.Client, embedder local.LocalEmbedder) (inngestgo.FunctionOpts, inngestgo.Trigger) {
	return inngestgo.FunctionOpts{
			ID:   "ingest-transcript",
			Name: "Ingest Transcript Event",
		},
		inngestgo.EventTrigger("transcript.received", nil)
}

// WisdomBatchInput represents input for wisdom batch processing
type WisdomBatchInput struct {
	Namespace       string    `json:"namespace"`
	EventsCount     int       `json:"events_count"`
	OldestTimestamp time.Time `json:"oldest_timestamp"`
}

// WisdomBatchOutput represents the output of wisdom batch processing
type WisdomBatchOutput struct {
	Success      bool   `json:"success"`
	Summary      string `json:"summary"`
	EntityCount  int    `json:"entity_count"`
	SummaryUID   string `json:"summary_uid"`
	ErrorMessage string `json:"error,omitempty"`
}

// wisdomBatchWorkflow implements the durable wisdom layer workflow
func wisdomBatchWorkflow(
	cfg WorkflowConfig,
	graphClient *graph.Client,
	embedder local.LocalEmbedder,
) func(ctx context.Context, input inngestgo.Input[WisdomBatchInput]) (any, error) {
	return func(ctx context.Context, input inngestgo.Input[WisdomBatchInput]) (any, error) {
		logger := cfg.Logger.With(
			zap.String("namespace", input.Event.Data.Namespace),
			zap.Int("events_count", input.Event.Data.EventsCount),
		)

		logger.Info("Starting wisdom batch processing workflow")

		// Step 1: Fetch batch events from buffer
		var events []graph.TranscriptEvent
		_, fetchErr := step.Run(ctx, "fetch-events", func(ctx context.Context) (struct{ Events []graph.TranscriptEvent }, error) {
			logger.Info("Step 1: Fetching batch events")
			// In a real implementation, this would fetch from the wisdom buffer
			return struct{ Events []graph.TranscriptEvent }{
				Events: events,
			}, nil
		})
		if fetchErr != nil {
			return WisdomBatchOutput{
				Success:       false,
				ErrorMessage: fmt.Sprintf("fetch events failed: %v", fetchErr),
			}, fetchErr
		}

		// Step 2: Summarize with AI (with auto-retry)
		var summary string
		summarizeRes, summarizeErr := step.Run(ctx, "summarize-batch", func(ctx context.Context) (struct{ Summary string }, error) {
			logger.Info("Step 2: Summarizing batch")
			// In a real implementation, this would call the AI service
			return struct{ Summary string }{
				Summary: "Batch summary placeholder",
			}, nil
		})
		if summarizeErr != nil {
			return WisdomBatchOutput{
				Success:       false,
				ErrorMessage: fmt.Sprintf("summarization failed: %v", summarizeErr),
			}, summarizeErr
		}
		summary = summarizeRes.Summary

		// Step 3: Generate and store embeddings
		embeddingDim := 0
		_, embedErr := step.Run(ctx, "generate-embedding", func(ctx context.Context) (struct{ Embedding []float32 }, error) {
			logger.Info("Step 3: Generating summary embedding")
			if embedder == nil {
				return struct{ Embedding []float32 }{
					Embedding: []float32{},
				}, nil
			}
			vec, err := embedder.Embed(summary)
			if err != nil {
				return struct{ Embedding []float32 }{}, err
			}
			embeddingDim = len(vec)
			return struct{ Embedding []float32 }{Embedding: vec}, nil
		})
		if embedErr != nil {
			logger.Warn("Embedding generation failed", zap.Error(embedErr))
		}

		// Step 4: Persist summary and entities to DGraph
		var summaryUID string
		persistRes, persistErr := step.Run(ctx, "persist-summary", func(ctx context.Context) (struct{ UID string }, error) {
			logger.Info("Step 4: Persisting summary to graph")
			// In a real implementation, this would create nodes in DGraph
			return struct{ UID string }{
				UID: "new-summary-uid",
			}, nil
		})
		if persistErr != nil {
			return WisdomBatchOutput{
				Success:       false,
				ErrorMessage: fmt.Sprintf("persistence failed: %v", persistErr),
			}, persistErr
		}
		summaryUID = persistRes.UID

		logger.Info("Wisdom batch workflow completed",
			zap.String("summary_uid", summaryUID),
			zap.Int("embedding_dim", embeddingDim))

		return WisdomBatchOutput{
			Success:    true,
			Summary:    summary,
			SummaryUID: summaryUID,
		}, nil
	}
}

// NewWisdomWorkflow creates the wisdom layer batch processing workflow
func NewWisdomWorkflow(cfg WorkflowConfig, graphClient *graph.Client, embedder local.LocalEmbedder) (inngestgo.FunctionOpts, inngestgo.Trigger) {
	return inngestgo.FunctionOpts{
			ID:   "wisdom-batch-process",
			Name: "Wisdom Layer Batch Processing",
		},
		inngestgo.EventTrigger("wisdom.batch.ready", nil)
}

// MaintenanceInput represents input for maintenance workflow
type MaintenanceInput struct {
	TriggeredAt time.Time `json:"triggered_at"`
}

// MaintenanceOutput represents the output of maintenance workflow
type MaintenanceOutput struct {
	Success          bool     `json:"success"`
	TasksCompleted   []string `json:"tasks_completed"`
	CacheEntriesCleared int    `json:"cache_entries_cleared"`
	ErrorMessage     string   `json:"error,omitempty"`
}

// maintenanceWorkflow implements the maintenance workflow
func maintenanceWorkflow(
	cfg WorkflowConfig,
	graphClient *graph.Client,
) func(ctx context.Context, input inngestgo.Input[MaintenanceInput]) (any, error) {
	return func(ctx context.Context, input inngestgo.Input[MaintenanceInput]) (any, error) {
		logger := cfg.Logger.With(zap.Time("triggered_at", input.Event.Data.TriggeredAt))

		logger.Info("Starting maintenance workflow")

		tasks := []string{}

		// Task 1: Clean up expired cache entries
		cacheRes := struct{ Count int }{}
		_, cacheErr := step.Run(ctx, "cleanup-cache", func(ctx context.Context) (struct{ Count int }, error) {
			logger.Info("Task 1: Cleaning up cache")
			// In a real implementation, this would clean up the cache
			return struct{ Count int }{Count: 0}, nil
		})
		if cacheErr == nil {
			tasks = append(tasks, "cache_cleanup")
			logger.Info("Cache cleanup completed", zap.Int("entries_cleared", cacheRes.Count))
		}

		// Task 2: Compact vector index
		_, indexErr := step.Run(ctx, "compact-vectors", func(ctx context.Context) (struct{ Success bool }, error) {
			logger.Info("Task 2: Compacting vector index")
			// In a real implementation, this would compact Qdrant
			return struct{ Success bool }{Success: true}, nil
		})
		if indexErr == nil {
			tasks = append(tasks, "vector_compaction")
		}

		// Task 3: Update node activations
		activateRes := struct{ Updated int }{}
		_, activateErr := step.Run(ctx, "update-activations", func(ctx context.Context) (struct{ Updated int }, error) {
			logger.Info("Task 3: Updating node activations")
			// In a real implementation, this would update activations
			return struct{ Updated int }{Updated: 0}, nil
		})
		if activateErr == nil {
			tasks = append(tasks, "activation_update")
			logger.Info("Activation update completed", zap.Int("nodes_updated", activateRes.Updated))
		}

		logger.Info("Maintenance workflow completed",
			zap.Strings("tasks", tasks))

		return MaintenanceOutput{
			Success:      true,
			TasksCompleted: tasks,
		}, nil
	}
}

// NewCronWorkflow creates a cron job for periodic maintenance tasks
func NewCronWorkflow(cfg WorkflowConfig, graphClient *graph.Client) (inngestgo.FunctionOpts, inngestgo.Trigger) {
	return inngestgo.FunctionOpts{
			ID:   "maintenance-cron",
			Name: "Periodic Maintenance Tasks",
		},
		inngestgo.CronTrigger("0 * * * *") // Every hour
}

// WorkflowService wraps the Inngest service for RMK workflows
type WorkflowService struct {
	client      inngestgo.Client
	config      WorkflowConfig
	logger      *zap.Logger
	graphClient *graph.Client
	embedder    local.LocalEmbedder
	server      *http.Server
}

// NewWorkflowService creates a new workflow service
func NewWorkflowService(cfg WorkflowConfig, graphClient *graph.Client, embedder local.LocalEmbedder) (*WorkflowService, error) {
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}

	client, err := inngestgo.NewClient(inngestgo.ClientOpts{
		AppID: cfg.AppID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Inngest client: %w", err)
	}

	ws := &WorkflowService{
		client:      client,
		config:      cfg,
		logger:      cfg.Logger,
		graphClient: graphClient,
		embedder:    embedder,
	}

	// Register workflows
	ws.registerWorkflows()

	return ws, nil
}

// registerWorkflows registers all workflows with the Inngest client
func (ws *WorkflowService) registerWorkflows() {
	// Ingestion workflow
	ingestOpts, ingestTrigger := NewIngestionWorkflow(ws.config, ws.graphClient, ws.embedder)
	_, err := inngestgo.CreateFunction(ws.client, ingestOpts, ingestTrigger, ingestTranscriptWorkflow(ws.config, ws.graphClient, ws.embedder))
	if err != nil {
		ws.logger.Error("Failed to register ingestion workflow", zap.Error(err))
	} else {
		ws.logger.Info("Registered ingestion workflow")
	}

	// Wisdom batch workflow
	wisdomOpts, wisdomTrigger := NewWisdomWorkflow(ws.config, ws.graphClient, ws.embedder)
	_, err = inngestgo.CreateFunction(ws.client, wisdomOpts, wisdomTrigger, wisdomBatchWorkflow(ws.config, ws.graphClient, ws.embedder))
	if err != nil {
		ws.logger.Error("Failed to register wisdom workflow", zap.Error(err))
	} else {
		ws.logger.Info("Registered wisdom batch workflow")
	}

	// Maintenance cron workflow
	maintenanceOpts, maintenanceTrigger := NewCronWorkflow(ws.config, ws.graphClient)
	_, err = inngestgo.CreateFunction(ws.client, maintenanceOpts, maintenanceTrigger, maintenanceWorkflow(ws.config, ws.graphClient))
	if err != nil {
		ws.logger.Error("Failed to register maintenance workflow", zap.Error(err))
	} else {
		ws.logger.Info("Registered maintenance cron workflow")
	}
}

// Serve starts the workflow service
func (ws *WorkflowService) Serve(addr string) error {
	ws.logger.Info("Starting Inngest workflow service",
		zap.String("addr", addr))

	// The Inngest client.Serve() returns an http.Handler
	// We need to wrap it to listen on the specified address
	handler := ws.client.Serve()
	ws.logger.Info("Got Inngest handler")

	// Create a simple HTTP server with the Inngest handler
	// Also add a health check endpoint
	mux := http.NewServeMux()
	mux.Handle("/", handler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"workflow-worker"}`))
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	ws.server = server
	ws.logger.Info("Starting HTTP server", zap.String("addr", addr))
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			ws.logger.Error("Workflow server error", zap.Error(err))
		}
		ws.logger.Info("HTTP server stopped")
	}()

	ws.logger.Info("Serve() returning, server should be running in background")
	return nil
}

// ServeHandler returns the HTTP handler for the workflow service
// This can be used with your own HTTP server
func (ws *WorkflowService) ServeHandler() interface{} {
	return ws.client.Serve()
}

// Shutdown gracefully shuts down the workflow service
func (ws *WorkflowService) Shutdown(ctx context.Context) error {
	ws.logger.Info("Shutting down workflow service")
	if ws.server != nil {
		return ws.server.Shutdown(ctx)
	}
	return nil
}
