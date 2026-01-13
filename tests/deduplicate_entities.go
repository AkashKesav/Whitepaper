// Build: go run deduplicate_entities.go
// +build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/dgraph-io/dgo/v240/protos/api"
	"github.com/reflective-memory-kernel/internal/graph"
	"go.uber.org/zap"
)

type NodeData struct {
	UID         string `json:"uid"`
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Description string `json:"description,omitempty"`
	EntityType  string `json:"entity_type,omitempty"`
	RelatedTo   []struct {
		UID string `json:"uid"`
	} `json:"related_to,omitempty"`
	InRelatedTo []struct {
		UID string `json:"uid"`
	} `json:"~related_to,omitempty"`
}

func main() {
	logger, _ := zap.NewDevelopment()
	cfg := graph.DefaultClientConfig()
	cfg.Address = "localhost:19080" // gRPC port

	ctx := context.Background()
	client, err := graph.NewClient(ctx, cfg, logger)
	if err != nil {
		log.Fatalf("Failed to connect to DGraph: %v", err)
	}

	targetName := "Sarah"
	fmt.Printf("🔍 Scanning for duplicates of '%s'...\n", targetName)

	query := `query {
        nodes(func: eq(name, "Sarah")) {
            uid
            name
            namespace
            description
            entity_type
            related_to { uid }
            ~related_to { uid }
        }
    }`

	// client.Query returns []byte in this codebase's wrapper
	jsonBytes, err := client.Query(ctx, query, nil)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	var result struct {
		Nodes []NodeData `json:"nodes"`
	}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		log.Fatalf("Unmarshal failed: %v", err)
	}

	if len(result.Nodes) <= 1 {
		fmt.Printf("✅ No duplicates found! (Count: %d)\n", len(result.Nodes))
		return
	}

	fmt.Printf("⚠️  Found %d copies of '%s'. deduplicating...\n", len(result.Nodes), targetName)

	// Master is the first one
	master := result.Nodes[0]
	duplicates := result.Nodes[1:]

	fmt.Printf("👑 Master Node: %s (Namespace: %s)\n", master.UID, master.Namespace) // 3. Prepare Merge Mutation
	var nquads []*api.NQuad
	var delNquads strings.Builder

	for _, dup := range duplicates {
		fmt.Printf("   🗑️  Merging duplicate: %s\n", dup.UID)

		// 1. Move OUTGOING edges: dup -> X  ==> master -> X
		for _, rel := range dup.RelatedTo {
			// master -> X
			nquads = append(nquads, &api.NQuad{
				Subject:   master.UID,
				Predicate: "related_to",
				ObjectId:  rel.UID,
			})
			// DEL: dup -> X
			delNquads.WriteString(fmt.Sprintf("<%s> <related_to> <%s> .\n", dup.UID, rel.UID))
		}

		// 2. Move INCOMING edges: X -> dup  ==> X -> master
		for _, rel := range dup.InRelatedTo {
			// X -> master
			nquads = append(nquads, &api.NQuad{
				Subject:   rel.UID,
				Predicate: "related_to",
				ObjectId:  master.UID,
			})
			// DEL: X -> dup
			delNquads.WriteString(fmt.Sprintf("<%s> <related_to> <%s> .\n", rel.UID, dup.UID))
		}

		// 3. Delete Scalar Data to hide the node (Wildcard deletion)
		delNquads.WriteString(fmt.Sprintf("<%s> <name> * .\n", dup.UID))
		delNquads.WriteString(fmt.Sprintf("<%s> <namespace> * .\n", dup.UID))

		if dup.Description != "" {
			delNquads.WriteString(fmt.Sprintf("<%s> <description> * .\n", dup.UID))
		}
		if dup.EntityType != "" {
			delNquads.WriteString(fmt.Sprintf("<%s> <entity_type> * .\n", dup.UID))
		}
		delNquads.WriteString(fmt.Sprintf("<%s> <activation> * .\n", dup.UID))
		delNquads.WriteString(fmt.Sprintf("<%s> <confidence> * .\n", dup.UID))
	}

	mu := &api.Mutation{
		Set:       nquads,
		DelNquads: []byte(delNquads.String()),
		CommitNow: true,
	}

	if _, err := client.GetDgraphClient().NewTxn().Mutate(ctx, mu); err != nil {
		log.Fatalf("❌ Mutation failed: %v", err)
	}

	fmt.Println("✅ Deduplication complete! (Check DB to verify)")
}
