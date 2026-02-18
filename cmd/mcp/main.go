// Package main provides the standalone MCP server for RMK
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/reflective-memory-kernel/internal/agent"
	"github.com/reflective-memory-kernel/internal/mcp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	version   = "1.0.0"
	buildTime = "unknown"

	// Command line flags
	mode        = flag.String("mode", "stdio", "Transport mode: stdio or http")
	addr        = flag.String("addr", ":8081", "HTTP address (for http mode)")
	natsAddr    = flag.String("nats", "nats://localhost:4222", "NATS address")
	mkURL       = flag.String("mk-url", "http://127.0.0.1:9000", "Memory Kernel URL")
	aiURL       = flag.String("ai-url", "http://localhost:8000", "AI Services URL")
	redisAddr   = flag.String("redis", "127.0.0.1:6379", "Redis address")
	logLevel    = flag.String("log-level", "info", "Log level: debug, info, warn, error")
	showVersion = flag.Bool("version", false, "Show version and exit")
)

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("RMK MCP Server v%s (built %s)\n", version, buildTime)
		os.Exit(0)
	}

	// Setup logger
	logger := setupLogger(*logLevel)
	defer logger.Sync()

	logger.Info("RMK MCP Server starting",
		zap.String("version", version),
		zap.String("mode", *mode),
		zap.String("addr", *addr))

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize dependencies
	agt, err := initializeAgent(logger)
	if err != nil {
		logger.Fatal("Failed to initialize agent", zap.Error(err))
	}

	// Create MCP server
	server := mcp.NewServer(mcp.ServerConfig{
		Logger:        logger,
		Agent:         agt,
		Name:          "reflective-memory-kernel",
		Version:       version,
	})

	logger.Info("MCP server initialized",
		zap.Int("tools", len(server.GetToolNames())))

	// Start transport
	var transport mcp.Transport
	switch *mode {
	case "stdio":
		transport = mcp.NewStdioTransport(logger)
	case "http":
		transport = mcp.NewHTTPTransport(*addr, logger)
	default:
		logger.Fatal("Unknown transport mode", zap.String("mode", *mode))
	}

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- transport.Serve(ctx, server)
	}()

	// Wait for signal or error
	select {
	case sig := <-sigCh:
		logger.Info("Received signal, shutting down", zap.String("signal", sig.String()))
		cancel()
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			logger.Error("Transport error", zap.Error(err))
		}
	}

	// Cleanup
	if err := agt.Stop(); err != nil {
		logger.Error("Error stopping agent", zap.Error(err))
	}

	logger.Info("RMK MCP Server stopped")
}

// initializeAgent initializes the Front-End Agent with all dependencies
func initializeAgent(logger *zap.Logger) (*agent.Agent, error) {
	// Create agent config
	cfg := agent.Config{
		NATSAddress:     *natsAddr,
		MemoryKernelURL: *mkURL,
		AIServicesURL:   *aiURL,
		RedisAddress:    *redisAddr,
		ResponseTimeout: 30,
	}

	// Create agent
	agt, err := agent.New(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Start agent (connects to NATS, Redis, initializes policy manager)
	if err := agt.Start(); err != nil {
		return nil, fmt.Errorf("failed to start agent: %w", err)
	}

	logger.Info("Agent initialized successfully")
	return agt, nil
}

// setupLogger creates a configured zap logger
func setupLogger(level string) *zap.Logger {
	var zapLevel zapcore.Level
	switch level {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapLevel),
		Development:      false,
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	if *mode == "stdio" {
		// Use console encoding for stdio mode (for Claude Desktop)
		config.Encoding = "console"
		config.EncoderConfig = zap.NewDevelopmentEncoderConfig()
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	logger, err := config.Build()
	if err != nil {
		// Fallback to default logger
		return zap.NewExample()
	}

	return logger
}
