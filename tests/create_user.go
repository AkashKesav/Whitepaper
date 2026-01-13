// Build: go run create_user.go
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/reflective-memory-kernel/internal/graph"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewDevelopment()
	cfg := graph.DefaultClientConfig()
	cfg.Address = "localhost:19080" // gRPC port

	ctx := context.Background()
	client, err := graph.NewClient(ctx, cfg, logger)
	if err != nil {
		log.Fatalf("Failed to connect to DGraph: %v", err)
	}

	// Execute Mutation
	logger.Info("Creating user node...")

	// We need to access the underlying dgo client to run a raw mutation easily
	// Or we can add a method to client. But wait, client.go usually exposes some mutation method?
	// Let's just use the raw dgo client connection exposed if possible, or use Client.CreateNodes if available?
	// Client.CreateNodes takes *graph.Node struct. Let's use that! It's much cleaner.

	userNode := &graph.Node{
		DType:      []string{"User"},
		Name:       "browser_test_user",
		Namespace:  "user_browser_test_user",
		Activation: 1.0,
		Confidence: 1.0,
		// Email is not on Node struct? Let's check.
		// If not, we might lose it, but name/namespace is what matters for identity.
	}

	uids, err := client.CreateNodes(ctx, []*graph.Node{userNode})
	if err != nil {
		log.Fatalf("Failed to create user node: %v", err)
	}

	fmt.Printf("✅ User Node Created! UIDs: %v\n", uids)
}
