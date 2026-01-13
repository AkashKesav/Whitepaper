// Build: go run debug_lookup.go
// +build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/reflective-memory-kernel/internal/graph"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewDevelopment()
	cfg := graph.DefaultClientConfig()
	cfg.Address = "localhost:19080"

	ctx := context.Background()
	client, err := graph.NewClient(ctx, cfg, logger)
	if err != nil {
		log.Fatalf("Failed to connect to DGraph: %v", err)
	}
	defer client.Close()

	// 1. Check for User Node (Try global and namespaced)
	fmt.Println("=== DEBUGGING USER NODE ===")
	// Note: Client doesn't have a "GetAllUsers" but we can try FindNodeByName if we know the ID
	// Frontend user ID might be a UUID. Let's try to search by type.

	query := `query {
		users(func: type(User)) {
			uid
			name
			namespace
		}
	}`
	resp, err := client.Query(ctx, query, nil)
	if err != nil {
		fmt.Printf("Query error: %v\n", err)
	} else {
		fmt.Printf("Users Found: %s\n", string(resp))
	}

	// 2. Check for "browser_test_user" specifically
	fmt.Println("\n=== DEBUGGING BROWSER USER ===")
	userQuery := `query {
		users(func: eq(name, "browser_test_user")) {
			uid
			name
			namespace
			dgraph.type
		}
	}`
	resp, err = client.Query(ctx, userQuery, nil)
	if err != nil {
		fmt.Printf("Query error: %v\n", err)
	} else {
		fmt.Printf("User Found: %s\n", string(resp))
	}

	// 1. Get Node by Name (Sarah)
	fmt.Println("--- 1. Querying 'Sarah' ---")
	// Use raw query to count nodes
	debugQuery := `query {
        nodes(func: eq(name, "Sarah")) {
            uid
            name
        }
    }`
	debugResp, err := client.Query(ctx, debugQuery, nil)
	if err != nil {
		fmt.Printf("❌ Query failed: %v\n", err)
	} else {
		var res struct {
			Nodes []struct {
				UID  string `json:"uid"`
				Name string `json:"name"`
			} `json:"nodes"`
		}
		if err := json.Unmarshal(debugResp, &res); err != nil {
			fmt.Printf("❌ Unmarshal failed: %v\n", err)
		} else {
			fmt.Printf("✅ Query 'Sarah': Found %d nodes\n", len(res.Nodes))
			for _, n := range res.Nodes {
				fmt.Printf("   - %s (%s)\n", n.Name, n.UID)
			}
		}
	}
}
