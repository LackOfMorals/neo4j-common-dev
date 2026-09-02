package main

import (
	"context"
	"fmt"
	"github.com/LackOfMorals/neo4jPackages/external/database"
)

func main() {
	ctx := context.Background()
	svc, err := database.New("http://neo4j:password@localhost:7474", database.WithDatabase("neo4j"))
	if err != nil {
		panic(err)
	}
	// 1. Buffered Execute
	res, err := svc.Execute(ctx, "RETURN 1 AS n", nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Buffered Keys: %v, Records: %d, Bookmarks: %v\n", res.Keys, len(res.Records), res.Summary.Bookmarks)

	// 2. ExecuteStream
	stream, err := svc.ExecuteStream(ctx, "UNWIND range(1,3) AS i RETURN i AS n", nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Stream Keys: %v\n", stream.Keys())
	count := 0
	for rec, err := range stream.Records() {
		if err != nil {
			panic(err)
		}
		count++
		v, _ := rec.Get("n")
		fmt.Printf("  row %d: %v\n", count, v)
	}
	fmt.Printf("Stream summary query type: %v, bookmarks: %v\n", stream.Summary().QueryType, stream.Summary().Bookmarks)
	stream.Close()

	// 3. Explicit mode single statement
	res2, err := svc.Execute(ctx, "CREATE (n:TestNode {id:1}) RETURN n", nil, database.WithTransactionMode(database.Explicit))
	if err != nil {
		panic(err)
	}
	fmt.Printf("Explicit CREATE Keys: %v, Records: %d\n", res2.Keys, len(res2.Records))

	// 4. Transaction lifecycle
	tx, err := svc.BeginTx(ctx)
	if err != nil {
		panic(err)
	}
	_, err = tx.Run(ctx, "CREATE (n:TestNode2 {id:2})", nil)
	if err != nil {
		panic(err)
	}
	commitRes, err := tx.Commit(ctx)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Tx committed, bookmarks: %v\n", commitRes.Bookmarks)

	// 5. Vector and Path decoding check
	// Create a node with a vector property
	_, err = svc.Execute(ctx, "CREATE (n:Vec {v: [1.0,2.0,3.0]}) RETURN n.v AS v", nil)
	if err != nil {
		panic(err)
	}
	resVec, err := svc.Execute(ctx, "MATCH (n:Vec) RETURN n.v AS v", nil)
	if err != nil {
		panic(err)
	}
	if len(resVec.Records) > 0 {
		v, _ := resVec.Records[0].Get("v")
		fmt.Printf("Vector decoded: %#v\n", v)
	}

	fmt.Println("Integration tests passed")
}
