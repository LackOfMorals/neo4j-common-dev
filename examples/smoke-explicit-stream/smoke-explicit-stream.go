package main

import (
	"context"
	"fmt"
	"log"

	"github.com/LackOfMorals/neo4jPackages/external/database"
)

func main() {
	ctx := context.Background()
	svc, err := database.New("http://neo4j:password@localhost:7474", database.WithDatabase("neo4j"))
	if err != nil {
		log.Fatal(err)
	}
	// cleanup
	_, err = svc.Execute(ctx, "MATCH (n:TestStream) DETACH DELETE n", nil)

	// something went wrong with the cleanup
	if err != nil {
		fmt.Printf("Error when cleaning up %v", err)
	}

	stream, err := svc.ExecuteStream(ctx, "UNWIND range(1,3) AS i CREATE (n:TestStream {id:i}) RETURN n", nil, database.WithTransactionMode(database.Explicit))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("keys", stream.Keys())
	for rec, err := range stream.Records() {
		if err != nil {
			log.Fatal(err)
		}
		v, _ := rec.Get("n")
		fmt.Println("row", v)
	}
	if err := stream.Close(); err != nil {
		log.Fatal(err)
	}
	// verify committed
	res, err := svc.Execute(ctx, "MATCH (n:TestStream) RETURN count(n) as c", nil)
	if err != nil {
		log.Fatal(err)
	}
	c, _ := res.Records[0].Get("c")
	fmt.Println("committed count", c)
}
