// Front-End Agent main entry point - gnet-based server
// Migrated from net/http to gnet for high-performance event-driven networking
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/agent"
	"github.com/reflective-memory-kernel/internal/server"
)

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("Starting Front-End Agent (gnet-based)")

	// Check if we should use gnet or legacy net/http
	useGnet := os.Getenv("USE_GNET") != "false" // Default to gnet

	// Load configuration
	cfg := agent.Config{
		NATSAddress:     getEnv("NATS_URL", "nats://localhost:4322"),
		MemoryKernelURL: getEnv("MEMORY_KERNEL_URL", "http://127.0.0.1:9000"),
		AIServicesURL:   getEnv("AI_SERVICES_URL", "http://localhost:8000"),
		RedisAddress:    getEnv("REDIS_ADDRESS", "127.0.0.1:6479"),
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

	if useGnet {
		startGnetServer(a, logger)
	} else {
		startLegacyServer(a, logger)
	}
}

func startGnetServer(a *agent.Agent, logger *zap.Logger) {
	logger.Info("Starting gnet server for agent")

	// Create gnet engine
	addr := ":" + getEnv("PORT", "3000")
	opts := &server.Options{
		Network:   "tcp",
		Multicore: true,
		Logger:    logger,
	}
	engine := server.New(addr, opts)

	// Create gnet server
	gnetServer := agent.NewGnetServer(a, logger.Named("server"))

	// Setup routes
	if err := gnetServer.SetupGnetRoutes(engine); err != nil {
		logger.Fatal("Failed to setup gnet routes", zap.Error(err))
	}

	// Start server in background
	go func() {
		if err := engine.Start(); err != nil {
			logger.Fatal("gnet server failed", zap.Error(err))
		}
	}()

	logger.Info("gnet server listening", zap.String("address", addr))

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down gnet server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	engine.Shutdown(ctx)
	a.Stop()

	logger.Info("Shutdown complete")
}

func startLegacyServer(a *agent.Agent, logger *zap.Logger) {
	logger.Info("Starting legacy net/http server for agent")

	// Import legacy server packages
	// This would use the original server.go with gorilla/mux
	// For now, just log a message
	logger.Warn("Legacy server not implemented - please use gnet (USE_GNET=true)")
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
