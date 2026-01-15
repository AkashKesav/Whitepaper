// Package main demonstrates using the RMK Go SDK
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/reflective-memory-kernel/sdk/go/rmk"
)

func main() {
	// Create client
	client := rmk.NewClient(rmk.ClientConfig{
		BaseURL: "http://localhost:9090",
		Timeout: 30000,
	})

	ctx := context.Background()

	// Login
	auth, err := client.Login(ctx, "user", "password")
	if err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	fmt.Printf("Logged in as: %s (role: %s)\n", auth.Username, auth.Role)

	// Store a memory
	memory, err := client.MemoryStore(ctx, &rmk.MemoryStoreRequest{
		Namespace: "user_123",
		Content:   "Claude Desktop is an AI assistant that can use MCP tools",
		NodeType:  rmk.NodeTypeFact,
		Tags:      []string{"ai", "mcp", "claude"},
	})
	if err != nil {
		log.Fatalf("Memory store failed: %v", err)
	}
	fmt.Printf("Stored memory: %s\n", memory.UID)

	// Search memories
	search, err := client.MemorySearch(ctx, &rmk.MemorySearchRequest{
		Namespace: "user_123",
		Query:     "Claude Desktop",
		Limit:     5,
	})
	if err != nil {
		log.Fatalf("Memory search failed: %v", err)
	}
	fmt.Printf("Found %d memories\n", search.Count)
	for _, node := range search.Results {
		fmt.Printf("  - %s: %s\n", node.Name, node.Description)
	}

	// Chat consultation
	chat, err := client.ChatConsult(ctx, &rmk.ChatConsultRequest{
		Namespace: "user_123",
		Message:   "What do you know about Claude Desktop?",
	})
	if err != nil {
		log.Fatalf("Chat consult failed: %v", err)
	}
	fmt.Printf("Response: %s\n", chat.Response)

	// Create an entity
	entity, err := client.EntityCreate(ctx, &rmk.EntityCreateRequest{
		Namespace:   "user_123",
		Name:        "Claude",
		EntityType:  "Person",
		Description: "AI assistant by Anthropic",
		Relationships: []rmk.Relationship{
			{Type: rmk.RelationshipWorksOn, Target: "anthropic"},
		},
	})
	if err != nil {
		log.Fatalf("Entity create failed: %v", err)
	}
	fmt.Printf("Created entity: %s\n", entity.UID)

	// List tools
	tools, err := client.ToolsList(ctx)
	if err != nil {
		log.Fatalf("Tools list failed: %v", err)
	}
	fmt.Printf("Available tools: %d\n", len(tools.Tools))
	for _, tool := range tools.Tools {
		fmt.Printf("  - %s: %s\n", tool.Name, tool.Description)
	}
}
