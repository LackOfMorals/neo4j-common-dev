// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

// Command config demonstrates how another Go app wires up the config
// package: describe each setting once (its flag, env var, description,
// and default), then resolve them all with a single Read call.
//
// Try it with:
//
//	go run ./examples/config --uri bolt://localhost:7687
//	NEO4J_URI=bolt://example.com:7687 go run ./examples/config
//	go run ./examples/config -h
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/LackOfMorals/neo4j-common/external/config"
)

func main() {
	svc := config.New(
		config.Field{Key: "uri", Flag: "uri", EnvVar: "NEO4J_URI", Description: "Neo4j connection URI", Default: "bolt://localhost:7687"},
		config.Field{Key: "debug", Flag: "debug", EnvVar: "DEBUG", Description: "enable debug logging", Default: "false"},
	)

	values, err := svc.Read(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	fmt.Printf("uri=%s debug=%t\n", values.String("uri"), values.Bool("debug"))
}
