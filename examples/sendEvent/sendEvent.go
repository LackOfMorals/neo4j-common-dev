// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

// Command sendEvent demonstrates how another Go app wires up and uses the
// analytics package: build a Service once at startup, register any
// properties common to every event, and call Emit per event.
package main

import (
	"fmt"

	"github.com/LackOfMorals/neo4j-common/analytics"
	"github.com/LackOfMorals/neo4j-common/config"
)

func main() {
	token := config.GetEnv("MIXPANEL_TOKEN")
	endpoint := config.GetEnvWithDefault("MIXPANEL_ENDPOINT", "https://api.mixpanel.com")
	uri := config.GetEnvWithDefault("NEO4J_URI", "bolt://localhost:7687")

	svc := analytics.New(token, endpoint, "sendEvent-example", uri,
		analytics.WithCommonProperties(map[string]any{"app_version": "1.0.0"}), // optional
	)
	if token == "" {
		svc.Disable() // safe to run unconfigured: no network call happens
	}

	svc.Emit("EXAMPLE_APP_STARTED", map[string]any{"example": true})

	if svc.IsEnabled() {
		fmt.Println("event sent")
	} else {
		fmt.Println("analytics disabled (no MIXPANEL_TOKEN set) — nothing sent")
	}
}
