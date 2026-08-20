// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

// Command logger demonstrates how another Go app wires up the logger
// package: build a Service once at startup, optionally extend the redacted
// key set for app-specific secrets, and adjust the level at runtime.
package main

import (
	"os"

	"github.com/LackOfMorals/neo4j-common/logger"
)

func main() {
	svc := logger.New("info", "json", os.Stdout,
		logger.WithSensitiveKeys("license_key"), // optional: redact app-specific secrets too
	)

	svc.Info("starting up", "license_key", "abc123", "password", "unused-here")

	svc.SetLevel("debug")
	svc.Debug("now visible after raising the level")
}
