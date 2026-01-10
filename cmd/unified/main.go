// Unified Binary - "The Singularity"
// Merges Front-End Agent and Memory Kernel into a single process for Zero-Copy communication.
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/agent"
	"github.com/reflective-memory-kernel/internal/graph"
	"github.com/reflective-memory-kernel/internal/kernel"
)

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("Starting Unified System (Agent + Kernel)...")

	// ==========================================
	// 1. Initialize Memory Kernel
	// ==========================================
	kernelCfg := kernel.Config{
		DGraphAddress:          getEnv("DGRAPH_URL", "localhost:9180"),
		NATSAddress:            getEnv("NATS_URL", "nats://localhost:4322"),
		RedisAddress:           getEnv("REDIS_URL", "localhost:6479"),
		AIServicesURL:          getEnv("AI_SERVICES_URL", "http://localhost:8000"),
		ReflectionInterval:     5 * time.Minute,
		ActivationDecayRate:    0.05,
		MinReflectionBatch:     10,
		MaxReflectionBatch:     100,
		IngestionBatchSize:     50,
		IngestionFlushInterval: 10 * time.Second,
		WisdomBatchSize:        5,
		WisdomFlushInterval:    5 * time.Second,
		QdrantURL:              getEnv("QDRANT_URL", "http://localhost:6333"),
	}

	k, err := kernel.New(kernelCfg, logger)
	if err != nil {
		logger.Fatal("Failed to create kernel", zap.Error(err))
	}

	if err := k.Start(); err != nil {
		logger.Fatal("Failed to start kernel", zap.Error(err))
	}
	logger.Info("Memory Kernel started")

	// ==========================================
	// 2. Initialize Front-End Agent
	// ==========================================
	agentCfg := agent.Config{
		NATSAddress:     getEnv("NATS_URL", "nats://localhost:4322"),
		MemoryKernelURL: "local", // Not used when using local client, but kept for config
		AIServicesURL:   getEnv("AI_SERVICES_URL", "http://localhost:8000"),
		RedisAddress:    getEnv("REDIS_ADDRESS", "127.0.0.1:6479"),
		ResponseTimeout: 60 * time.Second,
	}

	a, err := agent.New(agentCfg, logger)
	if err != nil {
		logger.Fatal("Failed to create agent", zap.Error(err))
	}

	// zero-copy injection
	localClient := agent.NewLocalKernelClient(k)
	a.SetKernel(localClient)

	if err := a.Start(); err != nil {
		logger.Fatal("Failed to start agent", zap.Error(err))
	}
	logger.Info("Front-End Agent started (Zero-Copy enabled)")

	// ==========================================
	// 3. Start HTTP Servers
	// ==========================================

	// Agent Server (Port 3000)
	agentRouter := mux.NewRouter()
	agentServer := agent.NewServer(a, logger)
	agentServer.SetupRoutes(agentRouter)
	agentRouter.PathPrefix("/").Handler(http.FileServer(http.Dir("./static")))

	portAgent := getEnv("PORT", "3000")
	httpServerAgent := &http.Server{
		Addr:         ":" + portAgent,
		Handler:      agentRouter,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logger.Info("Agent HTTP server starting", zap.String("port", portAgent))
		if err := httpServerAgent.ListenAndServe(); err != http.ErrServerClosed {
			logger.Fatal("Agent HTTP server failed", zap.Error(err))
		}
	}()

	// Kernel Server (Port 9000) - Optional, but keeping for debug/tooling compatibility
	kernelRouter := mux.NewRouter()
	setupKernelRoutes(kernelRouter, k, logger)

	portKernel := "9000"
	httpServerKernel := &http.Server{
		Addr:         ":" + portKernel,
		Handler:      kernelRouter,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logger.Info("Kernel HTTP server starting", zap.String("port", portKernel))
		if err := httpServerKernel.ListenAndServe(); err != http.ErrServerClosed {
			logger.Error("Kernel HTTP server failed", zap.Error(err))
		}
	}()

	// ==========================================
	// 4. Shutdown Handling
	// ==========================================
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down Unified System...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	httpServerAgent.Shutdown(ctx)
	httpServerKernel.Shutdown(ctx)
	a.Stop()
	k.Stop()

	logger.Info("Shutdown complete")
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func setupKernelRoutes(r *mux.Router, k *kernel.Kernel, logger *zap.Logger) {
	// Consultation endpoint
	r.HandleFunc("/api/consult", func(w http.ResponseWriter, r *http.Request) {
		var req graph.ConsultationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		resp, err := k.Consult(r.Context(), &req)
		if err != nil {
			logger.Error("Consultation failed", zap.Error(err))
			http.Error(w, "Consultation failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}).Methods("POST")

	// Hot Cache Store endpoint
	r.HandleFunc("/api/hot-cache/store", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UserID   string `json:"user_id"`
			Query    string `json:"query"`
			Response string `json:"response"`
			ID       string `json:"conversation_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if err := k.StoreInHotCache(req.UserID, req.Query, req.Response, req.ID); err != nil {
			logger.Error("Failed to store in hot cache", zap.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}).Methods("POST")
}
