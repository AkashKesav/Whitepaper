package main

import (
	"context"
	"fmt"
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
	// Prevent directory traversal
	if strings.Contains(r.URL.Path, "..") {
		http.NotFound(w, r)
		return
	}

	// Try to open the requested file
	file, err := h.staticDir.Open(strings.TrimPrefix(r.URL.Path, "/"))
	if err == nil {
		stat, err := file.Stat()
		if err == nil && !stat.IsDir() {
			// File exists and is not a directory - serve it
			http.FileServer(h.staticDir).ServeHTTP(w, r)
			return
		}
		file.Close()
	}

	// File doesn't exist or is a directory - serve index.html for SPA routing
	r.URL.Path = "/"
	http.FileServer(h.staticDir).ServeHTTP(w, r)
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

	// Check if we should run in frontend-only mode (for deployment when backend services aren't ready)
	frontendOnly := os.Getenv("FRONTEND_ONLY") == "true"

	var k *kernel.Kernel
	var a *agent.Agent

	if !frontendOnly {
		// 1. Initialize Kernel (Reflective Memory)
		kernelCfg := kernel.DefaultConfig()
		// Override defaults with Env Vars if needed (simplified for MVP)
		if dgraph := os.Getenv("DGRAPH_ADDRESS"); dgraph != "" {
			kernelCfg.DGraphAddress = dgraph
		}
		// Railway uses REDIS_URL or REDIS_PRIVATE_URL
		if redis := os.Getenv("REDIS_ADDRESS"); redis != "" {
			kernelCfg.RedisAddress = redis
		} else if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
			kernelCfg.RedisAddress = redisURL
		} else if redisPrivate := os.Getenv("REDIS_PRIVATE_URL"); redisPrivate != "" {
			kernelCfg.RedisAddress = redisPrivate
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

		var err error
		k, err = kernel.New(kernelCfg, logger.Named("kernel"))
		if err != nil {
			logger.Warn("Failed to initialize Kernel, running in frontend-only mode", zap.Error(err))
			k = nil
		}

		// 2. Initialize Agent (Consciousness)
		agentCfg := agent.DefaultConfig()
		if aiURL := os.Getenv("AI_SERVICES_URL"); aiURL != "" {
			agentCfg.AIServicesURL = aiURL
		}
		if redisAddr := os.Getenv("REDIS_ADDRESS"); redisAddr != "" {
			agentCfg.RedisAddress = redisAddr
		}

		a, err = agent.New(agentCfg, logger.Named("agent"))
		if err != nil {
			logger.Warn("Failed to initialize Agent, running in frontend-only mode", zap.Error(err))
			a = nil
		}

		// Only start backend services if both kernel and agent initialized successfully
		logger.Info("About to check kernel and agent", zap.Bool("k_is_nil", k == nil), zap.Bool("a_is_nil", a == nil))
		if k != nil && a != nil {
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
			logger.Info("About to start kernel")
			if err := k.Start(); err != nil {
				logger.Warn("Failed to start Kernel, running in frontend-only mode", zap.Error(err))
				k = nil
			} else {
				logger.Info("Kernel start succeeded")
				defer k.Stop()
			}

			// Start Agent Internals (Connects to Redis, NATS, initializes mkClient)
			logger.Info("About to start agent")
			if err := a.Start(); err != nil {
				logger.Warn("Failed to start Agent, running in frontend-only mode", zap.Error(err))
				a = nil
			} else {
				logger.Info("Agent start succeeded")
				defer a.Stop()
			}

			// NOW configure Agent to use Kernel directly (Zero-Copy Consultation)
			// MUST be called AFTER a.Start() since mkClient is initialized there
			logger.Info("About to configure agent-kernel bridge", zap.Bool("k_is_nil", k == nil), zap.Bool("a_is_nil", a == nil))
			if a != nil && k != nil {
				a.SetKernel(k)
			}
		}
		logger.Info("After backend initialization", zap.Bool("k_is_nil", k == nil), zap.Bool("a_is_nil", a == nil))
	} else {
		logger.Info("Running in FRONTEND_ONLY mode - backend services disabled")
	}

	// 5. Initialize Pre-Cortex (Cognitive Firewall for 90% cost reduction)
	// Skip if kernel not available
	logger.Info("About to check pre-cortex", zap.Bool("k_is_nil", k == nil))
	if k != nil {
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
		} else if a != nil {
			a.SetPreCortex(pc)

			// Wire up Ollama embedder for semantic similarity cache
			ollamaEmbedder := local.NewOllamaEmbedder("", "")
			pc.SetEmbedder(&ollamaEmbedderAdapter{ollamaEmbedder})
			logger.Info("Pre-Cortex semantic cache enabled with Ollama embeddings")
		}
	} else {
		logger.Info("Skipping Pre-Cortex initialization - no kernel available")
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

	// Only create server if agent is available
	if a != nil {
		server := agent.NewServer(a, logger.Named("server"), allowedOrigins...)
		if err := server.SetupRoutes(router); err != nil {
			logger.Fatal("Failed to setup routes", zap.Error(err))
		}
	} else {
		logger.Warn("Agent not available, setting up minimal routes for frontend-only mode")
		// Setup minimal health endpoint
		router.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"frontend-only","kernel":false,"agent":false}`))
		}).Methods("GET")
		router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"frontend-only","kernel":false,"agent":false}`))
		}).Methods("GET")
	}

	// Serve static files for web UI (must be after API routes to avoid conflicts)
	// Docker uses /app/static, local dev uses ./frontend/dist
	staticDir := "/app/static"
	if sd := os.Getenv("STATIC_DIR"); sd != "" {
		staticDir = sd
	} else if _, err := os.Stat("/app/static"); err != nil {
		// Not in Docker, try local paths
		if _, err := os.Stat("./static"); err == nil {
			staticDir = "./static"
		} else if _, err := os.Stat("./frontend/dist"); err == nil {
			staticDir = "./frontend/dist"
		}
	}
	// Always serve static files - SPA fallback handles missing files
	spaHandler := &spaHandler{staticDir: http.Dir(staticDir)}
	router.PathPrefix("/").Handler(spaHandler)
	logger.Info("Serving static files from", zap.String("dir", staticDir))

	// Debug endpoint to check if static files exist
	router.HandleFunc("/debug-static", func(w http.ResponseWriter, r *http.Request) {
		files, err := os.ReadDir(staticDir)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf("Error reading static dir: %v", err)))
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(fmt.Sprintf("Static dir: %s\nFiles:\n", staticDir)))
		for _, f := range files {
			w.Write([]byte(fmt.Sprintf("  - %s\n", f.Name())))
		}
		// Try to read index.html
		indexContent, err := os.ReadFile(staticDir + "/index.html")
		if err != nil {
			w.Write([]byte(fmt.Sprintf("\nError reading index.html: %v", err)))
		} else {
			w.Write([]byte(fmt.Sprintf("\nindex.html size: %d bytes, first 100 chars: %s", len(indexContent), string(indexContent[:min(100, len(indexContent))]))))
		}
	}).Methods("GET")

	corsObj := handlers.CORS(
		handlers.AllowedOrigins(allowedOrigins),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
		handlers.AllowCredentials(),
	)

	// Default port 9090 for local dev (vite proxies to this)
	// Docker sets PORT=8080 via environment
	apiPort := "0.0.0.0:9090"
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
