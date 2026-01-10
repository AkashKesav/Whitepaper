// Memory Kernel main entry point
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

	"github.com/reflective-memory-kernel/internal/graph"
	"github.com/reflective-memory-kernel/internal/kernel"
)

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("Starting Reflective Memory Kernel")

	// Load configuration from environment
	cfg := kernel.Config{
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
	}

	// Create and start the kernel
	k, err := kernel.New(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to create kernel", zap.Error(err))
	}

	if err := k.Start(); err != nil {
		logger.Fatal("Failed to start kernel", zap.Error(err))
	}

	// Setup HTTP API
	router := mux.NewRouter()
	setupRoutes(router, k, logger)

	// Start HTTP server
	port := getEnv("PORT", "9000")
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logger.Info("HTTP server starting", zap.String("port", port))
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			logger.Fatal("HTTP server failed", zap.Error(err))
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

	server.Shutdown(ctx)
	k.Stop()

	logger.Info("Shutdown complete")
}

func setupRoutes(r *mux.Router, k *kernel.Kernel, logger *zap.Logger) {
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

	// Stats endpoint
	r.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		stats, err := k.GetStats(r.Context())
		if err != nil {
			http.Error(w, "Failed to get stats", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}).Methods("GET")

	// Trigger reflection (for testing)
	r.HandleFunc("/api/reflect", func(w http.ResponseWriter, r *http.Request) {
		if err := k.TriggerReflection(r.Context()); err != nil {
			http.Error(w, "Reflection failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "reflection triggered"}`))
	}).Methods("POST")

	// EnsureUserNode endpoint (creates User node in DGraph if not exists)
	r.HandleFunc("/api/ensure-user", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if err := k.EnsureUserNode(r.Context(), req.Username, "subuser"); err != nil {
			logger.Error("EnsureUserNode failed", zap.Error(err))
			http.Error(w, "Failed to ensure user node", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}).Methods("POST")

	// Group API endpoints
	r.HandleFunc("/api/groups", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			OwnerID     string `json:"owner_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		groupID, err := k.CreateGroup(r.Context(), req.Name, req.Description, req.OwnerID)
		if err != nil {
			logger.Error("Create group failed", zap.Error(err))
			http.Error(w, "Create group failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"group_id": groupID})
	}).Methods("POST")

	r.HandleFunc("/api/groups", func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user")
		groups, err := k.ListUserGroups(r.Context(), userID)
		if err != nil {
			logger.Error("List groups failed", zap.Error(err))
			http.Error(w, "List groups failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(groups)
	}).Methods("GET")

	r.HandleFunc("/api/groups/members", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			GroupID  string `json:"group_id"`
			Username string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if err := k.AddGroupMember(r.Context(), req.GroupID, req.Username); err != nil {
			logger.Error("Add member failed", zap.Error(err))
			http.Error(w, "Add member failed", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "added"})
	}).Methods("POST")

	r.HandleFunc("/api/groups/is-admin", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			GroupNamespace string `json:"group_namespace"`
			UserID         string `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		isAdmin, err := k.IsGroupAdmin(r.Context(), req.GroupNamespace, req.UserID)
		if err != nil {
			logger.Error("Check admin status failed", zap.Error(err))
			http.Error(w, "Check admin status failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"is_admin": isAdmin})
	}).Methods("POST")

	// Health check
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	}).Methods("GET")
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
