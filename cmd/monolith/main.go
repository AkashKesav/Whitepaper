package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/agent"
	"github.com/reflective-memory-kernel/internal/ai/local"
	"github.com/reflective-memory-kernel/internal/graph"
	"github.com/reflective-memory-kernel/internal/kernel"
	"github.com/reflective-memory-kernel/internal/kernel/cache"
	"github.com/reflective-memory-kernel/internal/precortex"
)

// spaHandler implements http.Handler for Single Page Application support
type spaHandler struct {
	staticDir http.FileSystem
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Try to serve the file directly
	path := r.URL.Path
	// Prevent directory traversal
	if strings.Contains(path, "..") {
		http.NotFound(w, r)
		return
	}

	// Check if file exists
	file, err := h.staticDir.Open(path)
	if err == nil {
		stat, _ := file.Stat()
		// If it's a file (not a directory), serve it
		if !stat.IsDir() {
			http.FileServer(h.staticDir).ServeHTTP(w, r)
			return
		}
		file.Close()
	}

	// Otherwise serve index.html for SPA routing (using FileSystem, not relative path)
	index, err := h.staticDir.Open("index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer index.Close()

	stat, _ := index.Stat()
	http.ServeContent(w, r, "index.html", stat.ModTime(), index.(interface {
		io.ReadSeeker
	}))
}

// ollamaEmbedderAdapter wraps local.OllamaEmbedder to implement precortex.Embedder
type ollamaEmbedderAdapter struct {
	embedder *local.OllamaEmbedder
}

func (a *ollamaEmbedderAdapter) Embed(text string) ([]float32, error) {
	return a.embedder.Embed(text)
}

func (a *ollamaEmbedderAdapter) Close() {
	a.embedder.Close()
}

func main() {
	// Initialize Logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Global Panic Recovery
	defer func() {
		if r := recover(); r != nil {
			logger.Fatal("CRITICAL PANIC IN MONOLITH MAIN",
				zap.Any("panic", r),
				zap.Stack("stacktrace"),
			)
		}
	}()

	logger.Info("Starting Monolith (Unified Agent + Kernel)...")

	// 1. Initialize Kernel (Reflective Memory)
	kernelCfg := kernel.DefaultConfig()
	// Override defaults with Env Vars if needed (simplified for MVP)
	if dgraph := os.Getenv("DGRAPH_ADDRESS"); dgraph != "" {
		kernelCfg.DGraphAddress = dgraph
	}
	if redis := os.Getenv("REDIS_ADDRESS"); redis != "" {
		kernelCfg.RedisAddress = redis
	}
	if nats := os.Getenv("NATS_URL"); nats != "" {
		kernelCfg.NATSAddress = nats
	}
	if ai := os.Getenv("AI_SERVICES_URL"); ai != "" {
		kernelCfg.AIServicesURL = ai
	}
	if qdrant := os.Getenv("QDRANT_URL"); qdrant != "" {
		kernelCfg.QdrantURL = qdrant
	}

	k, err := kernel.New(kernelCfg, logger.Named("kernel"))
	if err != nil {
		logger.Fatal("Failed to initialize Kernel", zap.Error(err))
	}

	// 2. Initialize Agent (Consciousness)
	agentCfg := agent.DefaultConfig()
	if aiURL := os.Getenv("AI_SERVICES_URL"); aiURL != "" {
		agentCfg.AIServicesURL = aiURL
	}
	if redisAddr := os.Getenv("REDIS_ADDRESS"); redisAddr != "" {
		agentCfg.RedisAddress = redisAddr
	}
	// Since we are monolithic, Agent can talk to Kernel API directly via localhost
	// IF we kept the HTTP client. BUT we want zero-copy for Ingestion.
	// For Consultation (Read), we currently still use HTTP (agent -> mkClient -> HTTP -> Kernel).
	// TODO: Optimize Consultation to be zero-copy too (Phase 3.5)

	a, err := agent.New(agentCfg, logger.Named("agent"))
	if err != nil {
		logger.Fatal("Failed to initialize Agent", zap.Error(err))
	}

	// 3. Unification: Zero-Copy Bridge
	// Create buffered channel for transcripts
	ingestChan := make(chan *graph.TranscriptEvent, 1000)

	// Configure Agent to use this channel
	a.SetIngestChannel(ingestChan)

	// Start Bridge Goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		logger.Info("Zero-Copy Bridge Active: Agent -> Kernel")
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-ingestChan:
				// Direct function call across memory space
				if err := k.IngestEvent(ctx, event); err != nil {
					logger.Error("Bridge: Failed to ingest event", zap.Error(err))
				}
			}
		}
	}()

	// 4. Start Services

	// Start Kernel Background Loops
	if err := k.Start(); err != nil {
		logger.Fatal("Failed to start Kernel", zap.Error(err))
	}
	defer k.Stop()

	// Start Agent Internals (Connects to Redis, NATS, initializes mkClient)
	if err := a.Start(); err != nil {
		logger.Fatal("Failed to start Agent", zap.Error(err))
	}
	defer a.Stop()

	// NOW configure Agent to use Kernel directly (Zero-Copy Consultation)
	// MUST be called AFTER a.Start() since mkClient is initialized there
	a.SetKernel(k)

	// 5. Initialize Pre-Cortex (Cognitive Firewall for 90% cost reduction)
	logger.Info("Initializing Pre-Cortex cognitive firewall...")
	cacheManager, err := cache.NewManager(cache.DefaultConfig(), logger.Named("cache"))
	if err != nil {
		logger.Warn("Failed to initialize cache manager, Pre-Cortex will work without caching", zap.Error(err))
	} else {
		defer cacheManager.Close()
	}

	// Pre-Cortex configuration with semantic cache
	pcConfig := precortex.Config{
		EnableSemanticCache: true,
		EnableIntentRouter:  true,
		EnableDGraphReflex:  true, // Enabled for full functionality
		CacheSimilarity:     0.85, // 85% similarity threshold for cache hits
	}

	// Initialize Cache Vector Index
	// Use same Qdrant URL as Kernel (env var or default)
	qdrantURL := os.Getenv("QDRANT_URL") // Fallback handled by NewVectorIndex
	cacheIndex := kernel.NewVectorIndex(qdrantURL, kernel.CacheCollectionName, logger.Named("cache_index"))
	if err := cacheIndex.Initialize(context.Background()); err != nil {
		logger.Warn("Failed to initialize cache vector index", zap.Error(err))
	}

	pc, err := precortex.NewPreCortex(
		pcConfig,
		cacheManager,
		k.GetGraphClient(),
		cacheIndex,
		logger.Named("precortex"),
	)
	if err != nil {
		logger.Warn("Failed to initialize Pre-Cortex, LLM will be used for all requests", zap.Error(err))
	} else {
		a.SetPreCortex(pc)

		// Wire up Ollama embedder for semantic similarity cache
		ollamaEmbedder := local.NewOllamaEmbedder("", "")
		pc.SetEmbedder(&ollamaEmbedderAdapter{ollamaEmbedder})
		logger.Info("Pre-Cortex semantic cache enabled with Ollama embeddings")
	}

	// Configure allowed origins for WebSocket and CORS (from ALLOWED_ORIGINS env var)
	allowedOriginsStr := os.Getenv("ALLOWED_ORIGINS")
	var allowedOrigins []string
	if allowedOriginsStr == "" {
		// Default to localhost for development
		allowedOrigins = []string{"http://localhost:5173", "http://localhost:3000"}
		logger.Info("Using default CORS origins (development mode)",
			zap.Strings("origins", allowedOrigins))
	} else {
		allowedOrigins = strings.Split(allowedOriginsStr, ",")
		// Trim whitespace from each origin
		for i, origin := range allowedOrigins {
			allowedOrigins[i] = strings.TrimSpace(origin)
		}
		logger.Info("Using configured CORS origins",
			zap.Strings("origins", allowedOrigins))
	}

	// Start API Server
	router := mux.NewRouter()
	server := agent.NewServer(a, logger.Named("server"), allowedOrigins...)
	if err := server.SetupRoutes(router); err != nil {
		logger.Fatal("Failed to setup routes", zap.Error(err))
	}

	// Serve static files for web UI (must be after API routes to avoid conflicts)
	staticDir := "./static"
	// Always serve static files - SPA fallback handles missing files
	spaHandler := &spaHandler{staticDir: http.Dir(staticDir)}
	router.PathPrefix("/").Handler(spaHandler)
	logger.Info("Serving static files from", zap.String("dir", staticDir))

	corsObj := handlers.CORS(
		handlers.AllowedOrigins(allowedOrigins),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
		handlers.AllowCredentials(),
	)

	apiPort := "0.0.0.0:8080"
	if p := os.Getenv("PORT"); p != "" {
		apiPort = ":" + p
	}

	srv := &http.Server{
		Handler:      corsObj(router),
		Addr:         apiPort,
		WriteTimeout: 120 * time.Second,
		ReadTimeout:  120 * time.Second,
	}

	// Graceful Shutdown
	go func() {
		logger.Info("Monolith API listening", zap.String("addr", apiPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server startup failed", zap.Error(err))
		}
	}()

	// Wait for Signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	logger.Info("Shutting down Monolith...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("API shutdown error", zap.Error(err))
	}

	// Kernel & Agent Stop() called by defers
}
