// Package main provides the Inngest workflow worker entry point for RMK.
// This runs durable execution workflows for ingestion, wisdom layer, and maintenance.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/reflective-memory-kernel/internal/ai/local"
	"github.com/reflective-memory-kernel/internal/graph"
	"github.com/reflective-memory-kernel/internal/kernel"
)

var (
	logger     *zap.Logger
	logLevel   = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	addr       = flag.String("addr", "", "Address to listen on for Inngest events (default: :8080, or ADDR env var)")
	appID      = flag.String("app-id", "rmk-workflows", "Inngest App ID")
	dgraphAddr = flag.String("dgraph", "localhost:9080", "DGraph address")
	ollamaURL  = flag.String("ollama", "http://localhost:11434", "Ollama URL for embeddings")
)

func main() {
	flag.Parse()

	// Use ADDR environment variable if addr flag is not set
	if *addr == "" {
		*addr = os.Getenv("ADDR")
		if *addr == "" {
			*addr = ":8080"
		}
	}

	// Initialize logger
	initLogger(*logLevel)

	logger.Info("Starting RMK Workflow Worker",
		zap.String("addr", *addr),
		zap.String("app_id", *appID),
		zap.String("dgraph", *dgraphAddr))

	// Use DGRAPH_ADDRESS env var if flag is default (for Docker environment)
	if *dgraphAddr == "localhost:9080" {
		if envAddr := os.Getenv("DGRAPH_ADDRESS"); envAddr != "" {
			*dgraphAddr = envAddr
		}
	}

	// Wait for DGraph to be ready before connecting
	// This prevents race conditions where DGraph health check passes but schema isn't ready
	if err := waitForDGraph(*dgraphAddr, logger); err != nil {
		logger.Fatal("DGraph did not become ready in time", zap.Error(err))
	}

	// Initialize DGraph client - NewClient handles its own retry logic
	// Pass background context so it can retry up to MaxRetries times
	logger.Info("About to initialize DGraph client...")
	graphClient, err := initGraphClient(context.Background(), *dgraphAddr)
	if err != nil {
		logger.Fatal("Failed to initialize graph client", zap.Error(err))
	}
	logger.Info("DGraph client initialized successfully")

	// Initialize local embedder
	logger.Info("About to initialize embedder...")
	embedder := initEmbedder(*ollamaURL)
	logger.Info("Embedder initialized")

	// Configure workflows
	cfg := kernel.WorkflowConfig{
		InngestAPIKey: os.Getenv("INNGEST_API_KEY"),
		EventKey:      os.Getenv("INNGEST_EVENT_KEY"),
		AppID:         *appID,
		Logger:        logger,
	}

	// Create and start workflow service
	logger.Info("About to create workflow service...")
	workflowSvc, err := kernel.NewWorkflowService(cfg, graphClient, embedder)
	if err != nil {
		logger.Fatal("Failed to create workflow service", zap.Error(err))
	}
	logger.Info("Workflow service created successfully")

	// Handle graceful shutdown
	shutdownCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start serving in a goroutine
	logger.Info("About to start serving on", zap.String("addr", *addr))
	errCh := make(chan error, 1)
	go func() {
		logger.Info("Serve goroutine started, calling workflowSvc.Serve()")
		if err := workflowSvc.Serve(*addr); err != nil {
			errCh <- err
		}
		// If Serve() returns nil, server is running - wait for shutdown signal
		<-shutdownCtx.Done()
		logger.Info("Shutdown signal received in serve goroutine")
	}()

	// Wait for shutdown signal or error
	select {
	case <-shutdownCtx.Done():
		logger.Info("Shutdown signal received, stopping workflow service")
		if err := workflowSvc.Shutdown(context.Background()); err != nil {
			logger.Error("Error during shutdown", zap.Error(err))
		}
		logger.Info("Workflow worker stopped gracefully")
	case err := <-errCh:
		logger.Fatal("Workflow service error during Serve", zap.Error(err))
	}
}

// waitForDGraph waits for DGraph to be ready to accept connections.
// It checks both the health endpoint and ensures the server is ready for schema mutations.
func waitForDGraph(addr string, logger *zap.Logger) error {
	// Convert dgraph-alpha:9080 to http://dgraph-alpha:8080 for health check
	var healthURL string
	if strings.HasSuffix(addr, ":9080") {
		// Replace :9080 with :8080
		healthURL = fmt.Sprintf("http://%s:8080", strings.TrimSuffix(addr, ":9080"))
		logger.Info("Converted DGraph address for health check",
			zap.String("original", addr),
			zap.String("health_url", healthURL))
	} else {
		healthURL = fmt.Sprintf("http://%s", addr)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	maxAttempts := 40 // 40 * 5s = 200 seconds max wait

	for i := 0; i < maxAttempts; i++ {
		// Log attempt at info level
		logger.Info("Waiting for DGraph to be ready",
			zap.String("url", healthURL),
			zap.Int("attempt", i+1))

		// Check if DGraph health endpoint responds
		resp, err := client.Get(healthURL + "/health")
		if err != nil {
			logger.Info("DGraph health check failed (will retry)",
				zap.String("error", err.Error()),
				zap.Int("attempt", i+1))
		} else if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			logger.Info("DGraph health check returned non-OK status",
				zap.Int("status", resp.StatusCode),
				zap.Int("attempt", i+1))
		} else {
			resp.Body.Close()
			logger.Info("DGraph health check passed - ready to proceed",
				zap.String("url", healthURL),
				zap.Int("attempt", i+1))
			// DGraph is responding, give it a moment to be fully ready
			time.Sleep(2 * time.Second)
			return nil
		}

		if i < maxAttempts-1 {
			time.Sleep(5 * time.Second)
		}
	}

	return fmt.Errorf("DGraph at %s not ready after %d attempts", addr, maxAttempts)
}

func initLogger(level string) {
	var zapLevel zap.AtomicLevel
	switch level {
	case "debug":
		zapLevel = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapLevel = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapLevel = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		zapLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	config := zap.NewDevelopmentConfig()
	config.Level = zapLevel
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, _ = config.Build()
}

func initGraphClient(ctx context.Context, addr string) (*graph.Client, error) {
	cfg := graph.DefaultClientConfig()
	cfg.Address = addr

	return graph.NewClient(ctx, cfg, logger)
}

func initEmbedder(ollamaURL string) local.LocalEmbedder {
	embedder := local.NewOllamaEmbedder(ollamaURL, "nomic-embed-text")

	logger.Info("Initialized Ollama embedder",
		zap.String("url", ollamaURL),
		zap.String("model", "nomic-embed-text"))

	return embedder
}
