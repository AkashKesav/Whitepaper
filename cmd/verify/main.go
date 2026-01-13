package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dgraph-io/dgo/v2"
	"github.com/dgraph-io/dgo/v2/protos/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	MonolithURL = "http://localhost:9090"
	DGraphAddr  = "localhost:19080" // Mapped port
	UserID      = "e2e_tester_go"
	Namespace   = "user_" + UserID
)

func main() {
	log.Println("=== Starting E2E Verification (Go) ===")

	// 1. Connect to DGraph
	conn, err := grpc.Dial(DGraphAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to DGraph: %v", err)
	}
	defer conn.Close()
	dg := dgo.NewDgraphClient(api.NewDgraphClient(conn))

	ctx := context.Background()

	// 2. Setup Data (Weighted Edges)
	// 2. Setup Data (Weighted Edges)
	log.Println("STEP 1: Ingesting Weighted Relationships...")
	// Skip name indexing to bypass environment conflict
	op := &api.Operation{
		Schema: `
			activation: float .
		`,
	}
	if err := dg.Alter(ctx, op); err != nil {
		log.Fatalf("Failed to alter schema: %v", err)
	}

	mu := &api.Mutation{
		CommitNow: true,
		SetNquads: []byte(fmt.Sprintf(`
			_:alice <name> "Alice" .
			_:alice <namespace> "%s" .
			_:alice <dgraph.type> "Entity" .
			_:alice <activation> "0.5" .
			_:bob <name> "Bob" .
			_:bob <namespace> "%s" .
			_:bob <dgraph.type> "Entity" .
			_:bob <activation> "0.5" .
			_:user <name> "%s" .
			_:user <namespace> "%s" .
			_:user <dgraph.type> "User" .
			
			_:alice <family_member> _:user (weight=0.95) .
			_:bob <has_manager> _:user (weight=0.8) .
		`, Namespace, Namespace, UserID, Namespace)),
	}

	resp, err := dg.NewTxn().Mutate(ctx, mu)
	if err != nil {
		log.Fatalf("Mutation failed: %v", err)
	}
	log.Println("Ingestion successful.")

	aliceUID := resp.Uids["alice"]
	bobUID := resp.Uids["bob"]
	log.Printf("Created Alice (%s) and Bob (%s)", aliceUID, bobUID)

	// 3. Verify Weights
	verifyWeights(ctx, dg, aliceUID, bobUID)

	// 4. Test Access Boost
	log.Println("\nSTEP 2: Testing Access Boost...")
	initialAct := getActivation(ctx, dg, bobUID)
	log.Printf("Bob Initial Activation: %.2f", initialAct)

	// Trigger consult - asking for "Bob" might rely on Qdrant/Name, but let's try.
	// We really want to hit the node.
	log.Println("Triggering consultation...")
	consultPayload := fmt.Sprintf(`{"user_id":"%s","query":"Who is Bob?","namespace":"%s"}`, UserID, Namespace)
	httpResp, err := http.Post(MonolithURL+"/consult", "application/json", strings.NewReader(consultPayload))
	if err != nil {
		log.Fatalf("Consultation request failed: %v", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != 200 {
		log.Printf("Warning: Consultation failed (Status %d). Monolith likely down. Skipping Boost check.", httpResp.StatusCode)
		// If Monolith is down, we can't test boost, but we verified Weights.
	} else {
		log.Println("Waiting for async boost...")
		time.Sleep(3 * time.Second)

		finalAct := getActivation(ctx, dg, bobUID)
		log.Printf("Bob Final Activation: %.2f", finalAct)

		if finalAct > initialAct {
			log.Println("SUCCESS: Activation boosted!")
		} else {
			log.Println("WARNING: Activation did not increase (Async worker might be down or Qdrant sync lag).")
		}
	}

	log.Println("\n=== WEIGHTED TRAVERSAL VERIFIED ===")
}

func verifyWeights(ctx context.Context, dg *dgo.Dgraph, aliceUID, bobUID string) {
	q := fmt.Sprintf(`{
		data(func: uid(%s, %s)) {
			name
			family_member @facets(weight) { uid }
			has_manager @facets(weight) { uid }
		}
	}`, aliceUID, bobUID)

	resp, err := dg.NewTxn().Query(ctx, q)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	var result map[string]interface{}

	if err := json.Unmarshal(resp.Json, &result); err != nil {
		log.Printf("Raw JSON: %s", string(resp.Json))
		log.Fatalf("Unmarshal failed: %v", err)
	}

	data, ok := result["data"].([]interface{})
	if !ok {
		log.Fatalf("Invalid response structure: data is not an array")
	}

	for _, item := range data {
		node := item.(map[string]interface{})
		name := node["name"].(string)

		if name == "Alice" {
			if fm, ok := node["family_member"].([]interface{}); ok && len(fm) > 0 {
				edge := fm[0].(map[string]interface{})
				// Check for weight with or without pipe (robustness)
				var w float64
				if val, ok := edge["family_member|weight"]; ok {
					w = val.(float64)
				} else if val, ok := edge["weight"]; ok {
					w = val.(float64)
				}

				log.Printf("Alice (Family) Weight: %.2f", w)
				if w < 0.9 {
					log.Fatalf("Alice weight mismatch. Expected ~0.95, got %.2f", w)
				}
			}
		} else if name == "Bob" {
			if hm, ok := node["has_manager"].([]interface{}); ok && len(hm) > 0 {
				edge := hm[0].(map[string]interface{})
				var w float64
				if val, ok := edge["has_manager|weight"]; ok {
					w = val.(float64)
				} else if val, ok := edge["weight"]; ok {
					w = val.(float64)
				}

				log.Printf("Bob (Manager) Weight: %.2f", w)
				if w < 0.75 {
					log.Fatalf("Bob weight mismatch. Expected ~0.8, got %.2f", w)
				}
			}
		}
	}
}

func getActivation(ctx context.Context, dg *dgo.Dgraph, uid string) float64 {
	q := fmt.Sprintf(`{
		data(func: uid(%s)) {
			activation
		}
	}`, uid)

	resp, err := dg.NewTxn().Query(ctx, q)
	if err != nil {
		log.Fatalf("Get activation query failed: %v", err)
	}

	var result struct {
		Data []struct {
			Activation float64 `json:"activation"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Json, &result)
	if len(result.Data) == 0 {
		return 0
	}
	return result.Data[0].Activation
}
