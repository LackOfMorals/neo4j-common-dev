package main

import (
	"context"
	"fmt"
	"github.com/LackOfMorals/neo4jPackages/external/database"
)

func main() {
	svc, err := database.New("http://neo4j:password@localhost:7474", database.WithDatabase("neo4j"))
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	res, err := svc.Execute(ctx, "RETURN 1 AS n", nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Keys: %v\n", res.Keys)
	for _, r := range res.Records {
		fmt.Printf("Record: %v\n", r)
	}
}
