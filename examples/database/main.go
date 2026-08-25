package main

import (
	"context"
	"fmt"
	"log"

	"github.com/LackOfMorals/neo4jPackages/external/database"
)

func main() {
	ctx := context.Background()

	// Bolt backend example
	boltURI := "bolt://neo4j:password@localhost:7687"
	boltSvc, err := database.New(boltURI, database.WithDatabase("neo4j"))
	if err != nil {
		log.Fatalf("bolt New failed: %v", err)
	}
	defer boltSvc.Close(ctx)

	fmt.Println("=== Bolt Backend ===")
	res, err := boltSvc.Execute(ctx, "RETURN 1 AS n", nil)
	if err != nil {
		log.Fatalf("bolt Execute failed: %v", err)
	}
	fmt.Printf("Bolt buffered: Keys=%v Records=%d\n", res.Keys, len(res.Records))

	stream, err := boltSvc.ExecuteStream(ctx, "UNWIND range(1,3) AS i RETURN i AS n", nil)
	if err != nil {
		log.Fatalf("bolt ExecuteStream failed: %v", err)
	}
	fmt.Printf("Bolt stream keys: %v\n", stream.Keys())
	for rec, err := range stream.Records() {
		if err != nil {
			log.Fatalf("stream error: %v", err)
		}
		v, _ := rec.Get("n")
		fmt.Printf("  row: %v\n", v)
	}
	stream.Close()

	// Query API backend example
	httpURI := "http://neo4j:password@localhost:7474"
	httpSvc, err := database.New(httpURI, database.WithDatabase("neo4j"), database.WithMaxResultBytes(10*1024*1024))
	if err != nil {
		log.Fatalf("http New failed: %v", err)
	}
	defer httpSvc.Close(ctx)

	fmt.Println("\n=== Query API Backend ===")
	res2, err := httpSvc.Execute(ctx, "RETURN 2 AS n", nil, database.WithTransactionMode(database.Implicit))
	if err != nil {
		log.Fatalf("http Execute failed: %v", err)
	}
	fmt.Printf("HTTP buffered: Keys=%v Records=%d\n", res2.Keys, len(res2.Records))

	// Explicit single-statement
	res3, err := httpSvc.Execute(ctx, "CREATE (n:Demo {id: 1}) RETURN n", nil, database.WithTransactionMode(database.Explicit))
	if err != nil {
		log.Fatalf("explicit Execute failed: %v", err)
	}
	fmt.Printf("HTTP explicit: Records=%d\n", len(res3.Records))

	// Multi-statement Tx
	tx, err := httpSvc.BeginTx(ctx)
	if err != nil {
		log.Fatalf("BeginTx failed: %v", err)
	}
	_, err = tx.Run(ctx, "CREATE (n:TxNode {id: 2})", nil)
	if err != nil {
		log.Fatalf("Tx Run failed: %v", err)
	}
	commit, err := tx.Commit(ctx)
	if err != nil {
		log.Fatalf("Tx Commit failed: %v", err)
	}
	fmt.Printf("Tx committed, bookmarks: %v\n", commit.Bookmarks)

	// Result type inspection
	res4, err := httpSvc.Execute(ctx, "MATCH (n:Demo) RETURN n AS node", nil)
	if err != nil {
		log.Fatalf("node query failed: %v", err)
	}
	if len(res4.Records) > 0 {
		node, ok := res4.Records[0].GetNode("node")
		if ok {
			fmt.Printf("Node ElementID=%s Labels=%v Props=%v\n", node.ElementID, node.Labels, node.Properties)
		}
	}

	fmt.Println("Example complete")
}
