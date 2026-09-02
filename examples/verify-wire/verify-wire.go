package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/LackOfMorals/neo4jPackages/external/database"
)

func main() {
	ctx := context.Background()
	svc, err := database.New("http://neo4j:password@localhost:7474", database.WithDatabase("neo4j"))
	if err != nil {
		log.Fatalf("New: %v", err)
	}
	// Setup data
	_, err = svc.Execute(ctx, "CREATE (a:Node {id:1})-[:REL {k:'v'}]->(b:Node {id:2})", nil)
	if err != nil {
		log.Fatalf("setup: %v", err)
	}
	// Path query
	res, err := svc.Execute(ctx, "MATCH p = (a:Node)-[r:REL]->(b:Node) RETURN p", nil)
	if err != nil {
		log.Fatalf("path query: %v", err)
	}
	if len(res.Records) == 0 {
		log.Fatal("no path")
	}
	raw, _ := json.Marshal(res.Records[0].Values()[0])
	fmt.Println("Path raw JSON:", string(raw))

	// Relationship fields
	res2, err := svc.Execute(ctx, "MATCH ()-[r:REL]->() RETURN r", nil)
	if err != nil {
		log.Fatalf("rel query: %v", err)
	}
	if len(res2.Records) > 0 {
		rel, ok := res2.Records[0].GetRelationship("r")
		if ok {
			fmt.Printf("Relationship ElementID=%s Type=%s Start=%s End=%s Props=%v\n", rel.ElementID, rel.Type, rel.StartElementID, rel.EndElementID, rel.Properties)
		}
	}

	fmt.Println("Done")
}
