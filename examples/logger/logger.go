// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

// Command logger demonstrates how another Go app wires up the logger
// package: resolve its level and format via config (flag/env, so the
// caller can adjust verbosity without a code change), optionally extend
// the redacted key set for app-specific secrets, and adjust the level at
// runtime.
//
// Try it with:
//
//	go run ./examples/logger --level debug --format text
//	LOG_LEVEL=debug go run ./examples/logger
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/LackOfMorals/neo4jPackages/external/logger"
	"github.com/LackOfMorals/neo4jPackages/external/config"
)

func main() {
	cfg := config.New(
		config.Field{Key: "level", Flag: "level", EnvVar: "LOG_LEVEL", Description: "log level (debug, info, notice, warning, error, critical, alert, emergency)", Default: "info"},
		config.Field{Key: "format", Flag: "format", EnvVar: "LOG_FORMAT", Description: "log format (text or json)", Default: "json"},
	)
	values, err := cfg.Read(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	svc := logger.New(values.String("level"), values.String("format"), os.Stdout,
		logger.WithSensitiveKeys("license_key"), // optional: redact app-specific secrets too
	)

	svc.Info("starting up", "license_key", "abc123", "password", "unused-here")

	svc.SetLevel("debug")
	svc.Debug("now visible after raising the level")
}
