// Front-End Agent main entry point
package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/agent"
)

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("Starting Front-End Agent")

	// Load configuration
	cfg := agent.Config{
		NATSAddress:     getEnv("NATS_URL", "nats://localhost:4322"),
		MemoryKernelURL: getEnv("MEMORY_KERNEL_URL", "http://localhost:9000"),
		AIServicesURL:   getEnv("AI_SERVICES_URL", "http://localhost:8000"),
		ResponseTimeout: 60 * time.Second,
	}

	// Create and start the agent
	a, err := agent.New(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to create agent", zap.Error(err))
	}

	if err := a.Start(); err != nil {
		logger.Fatal("Failed to start agent", zap.Error(err))
	}

	// Setup HTTP server
	router := mux.NewRouter()
	server := agent.NewServer(a, logger)
	server.SetupRoutes(router)

	// Serve static files for web UI
	// router.PathPrefix("/").Handler(http.FileServer(http.Dir("./static")))

	port := getEnv("PORT", "3000")
	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logger.Info("HTTP server starting", zap.String("port", port))
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	// Wait for shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down...")
	a.Stop()
	logger.Info("Shutdown complete")
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
