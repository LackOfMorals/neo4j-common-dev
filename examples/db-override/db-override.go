package main

import (
	"context"
	"fmt"
	"github.com/LackOfMorals/neo4jPackages/external/database"
	"log"
)

func main() {
	ctx := context.Background()
	svc, err := database.New("http://neo4j:password@localhost:7474", database.WithDatabase("neo4j"))
	if err != nil {
		log.Fatal(err)
	}
	res, err := svc.Execute(ctx, "RETURN 'ok' as v", nil, database.WithDatabaseOverride("neo4j"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res.Records[0].Get("v"))
}
