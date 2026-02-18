// Memory Kernel main entry point - gnet-based server
// Migrated from net/http to gnet for high-performance event-driven networking
package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/reflective-memory-kernel/internal/graph"
	"github.com/reflective-memory-kernel/internal/kernel"
	"github.com/reflective-memory-kernel/internal/server"
)

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("Starting Reflective Memory Kernel (gnet-based)")

	// Load configuration from environment
	cfg := kernel.Config{
		DGraphAddress:          getEnv("DGRAPH_URL", "localhost:9180"),
		NATSAddress:            getEnv("NATS_URL", "nats://localhost:4322"),
		RedisAddress:           getEnv("REDIS_URL", "localhost:6479"),
		AIServicesURL:          getEnv("AI_SERVICES_URL", "http://localhost:8000"),
		ReflectionInterval:     5 * time.Minute,
		ActivationDecayRate:    0.002,
		MinReflectionBatch:     10,
		MaxReflectionBatch:     100,
		IngestionBatchSize:     50,
		IngestionFlushInterval: 10 * time.Second,
	}

	// Create and start the kernel
	k, err := kernel.New(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to create kernel", zap.Error(err))
	}

	if err := k.Start(); err != nil {
		logger.Fatal("Failed to start kernel", zap.Error(err))
	}

	// Create gnet engine
	addr := ":" + getEnv("PORT", "9000")
	opts := &server.Options{
		Network:   "tcp",
		Multicore: true,
		Logger:    logger,
	}
	engine := server.New(addr, opts)

	// Setup routes
	setupRoutes(engine, k, logger)

	logger.Info("gnet server starting", zap.String("address", addr))

	// Start server in background
	go func() {
		if err := engine.Start(); err != nil {
			logger.Fatal("Server failed", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	engine.Shutdown(ctx)
	k.Stop()

	logger.Info("Shutdown complete")
}

func setupRoutes(engine *server.Engine, k *kernel.Kernel, logger *zap.Logger) {
	// Health check
	engine.GET("/health", func(req *server.Request) *server.Response {
		return server.JSON(map[string]string{"status": "healthy"}, 200)
	})

	// Consultation endpoint
	engine.POST("/api/consult", func(req *server.Request) *server.Response {
		var consultationReq graph.ConsultationRequest
		if err := server.ParseJSON(req, &consultationReq); err != nil {
			return server.JSON(map[string]string{"error": "Invalid request", "details": err.Error()}, 400)
		}

		resp, err := k.Consult(context.Background(), &consultationReq)
		if err != nil {
			logger.Error("Consultation failed", zap.Error(err))
			return server.JSON(map[string]string{"error": "Consultation failed"}, 500)
		}

		return server.JSON(resp, 200)
	})

	// Stats endpoint
	engine.GET("/api/stats", func(req *server.Request) *server.Response {
		stats, err := k.GetStats(context.Background())
		if err != nil {
			return server.JSON(map[string]string{"error": "Failed to get stats"}, 500)
		}
		return server.JSON(stats, 200)
	})

	// Trigger reflection (for testing)
	engine.POST("/api/reflect", func(req *server.Request) *server.Response {
		if err := k.TriggerReflection(context.Background()); err != nil {
			return server.JSON(map[string]string{"error": "Reflection failed"}, 500)
		}
		return server.JSON(map[string]string{"status": "reflection triggered"}, 200)
	})

	// EnsureUserNode endpoint
	engine.POST("/api/ensure-user", func(req *server.Request) *server.Response {
		var userReq struct {
			Username string `json:"username"`
		}
		if err := server.ParseJSON(req, &userReq); err != nil {
			return server.JSON(map[string]string{"error": "Invalid request"}, 400)
		}

		if err := k.EnsureUserNode(context.Background(), userReq.Username, "subuser"); err != nil {
			logger.Error("EnsureUserNode failed", zap.Error(err))
			return server.JSON(map[string]string{"error": "Failed to ensure user node"}, 500)
		}

		return server.JSON(map[string]string{"status": "ok"}, 200)
	})

	// Create group
	engine.POST("/api/groups", func(req *server.Request) *server.Response {
		var groupReq struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			OwnerID     string `json:"owner_id"`
		}
		if err := server.ParseJSON(req, &groupReq); err != nil {
			return server.JSON(map[string]string{"error": "Invalid request"}, 400)
		}

		groupID, err := k.CreateGroup(context.Background(), groupReq.Name, groupReq.Description, groupReq.OwnerID)
		if err != nil {
			logger.Error("Create group failed", zap.Error(err))
			return server.JSON(map[string]string{"error": "Create group failed"}, 500)
		}

		return server.JSON(map[string]string{"group_id": groupID}, 200)
	})

	// List groups
	engine.GET("/api/groups", func(req *server.Request) *server.Response {
		userID := req.Query.Get("user")
		groups, err := k.ListUserGroups(context.Background(), userID)
		if err != nil {
			logger.Error("List groups failed", zap.Error(err))
			return server.JSON(map[string]string{"error": "List groups failed"}, 500)
		}
		return server.JSON(groups, 200)
	})

	// Add group member
	engine.POST("/api/groups/members", func(req *server.Request) *server.Response {
		var memberReq struct {
			GroupID  string `json:"group_id"`
			Username string `json:"username"`
		}
		if err := server.ParseJSON(req, &memberReq); err != nil {
			return server.JSON(map[string]string{"error": "Invalid request"}, 400)
		}

		if err := k.AddGroupMember(context.Background(), memberReq.GroupID, memberReq.Username); err != nil {
			logger.Error("Add member failed", zap.Error(err))
			return server.JSON(map[string]string{"error": "Add member failed"}, 500)
		}

		return server.JSON(map[string]string{"status": "added"}, 200)
	})

	// Check admin status
	engine.POST("/api/groups/is-admin", func(req *server.Request) *server.Response {
		var adminReq struct {
			GroupNamespace string `json:"group_namespace"`
			UserID         string `json:"user_id"`
		}
		if err := server.ParseJSON(req, &adminReq); err != nil {
			return server.JSON(map[string]string{"error": "Invalid request"}, 400)
		}

		isAdmin, err := k.IsGroupAdmin(context.Background(), adminReq.GroupNamespace, adminReq.UserID)
		if err != nil {
			logger.Error("Check admin status failed", zap.Error(err))
			return server.JSON(map[string]string{"error": "Check admin status failed"}, 500)
		}

		return server.JSON(map[string]bool{"is_admin": isAdmin}, 200)
	})
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// JSON helper for encoding responses
func encodeJSON(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}
