// Build: go run debug_schema.go
// +build ignore

package main

import (
	"context"
	"log"

	"github.com/dgraph-io/dgo/v2"
	"github.com/dgraph-io/dgo/v2/protos/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main2() {
	conn, err := grpc.Dial("localhost:19080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()
	dg := dgo.NewDgraphClient(api.NewDgraphClient(conn))
	ctx := context.Background()

	schemas := []string{
		"entity_type: string @index(exact) .",
		"created_by: string @index(exact) .",
		"workspace_id: string @index(exact) .",
		"invitee_user_id: string @index(exact) .",
		"token: string @index(exact) .",
		"role: string @index(exact) .",
		"is_active: bool @index(bool) .",
	}

	for _, s := range schemas {
		log.Printf("Applying: %s", s)
		err := dg.Alter(ctx, &api.Operation{Schema: s})
		if err != nil {
			log.Fatalf("FAILED on '%s': %v", s, err)
		}
		log.Println("SUCCESS")
	}
}
