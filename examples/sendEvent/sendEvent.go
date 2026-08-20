// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

// Command sendEvent demonstrates how another Go app wires up and uses the
// analytics package: resolve settings via config, log via logger, build a
// Service once at startup, register any properties common to every event,
// and call Emit per event. EU-resident Mixpanel projects only accept events
// at Mixpanel's EU endpoint — pass --eu-residency (or MIXPANEL_EU_RESIDENCY)
// rather than trying to point --mixpanel-endpoint at it manually.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/LackOfMorals/neo4j-common/external/analytics"
	"github.com/LackOfMorals/neo4j-common/external/config"
	"github.com/LackOfMorals/neo4j-common/external/logger"
)

func main() {
	cfg := config.New(
		config.Field{Key: "token", Flag: "mixpanel-token", EnvVar: "MIXPANEL_TOKEN", Description: "Mixpanel project token"},
		config.Field{Key: "endpoint", Flag: "mixpanel-endpoint", EnvVar: "MIXPANEL_ENDPOINT", Description: "Mixpanel API endpoint", Default: "https://api.mixpanel.com"},
		config.Field{Key: "euResidency", Flag: "eu-residency", EnvVar: "MIXPANEL_EU_RESIDENCY", Description: "send events to Mixpanel's EU-residency endpoint", Default: "true"},
	)
	values, err := cfg.Read(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	log := logger.New("info", "json", os.Stdout)

	token := values.String("token")
	endpoint := values.String("endpoint")

	svc := analytics.New(token, endpoint, "sendEvent-example",
		// Every property here is ours to choose — Service only ever adds the
		// machine ID automatically. Anything else an event needs (like
		// app_version below, or a real user/session identifier) is ours to
		// supply, either here as a common property or per-call to Emit.
		analytics.WithCommonProperties(map[string]any{"app_version": "1.0.0"}),
		analytics.WithEuResidency(values.Bool("euResidency")), // overrides endpoint when true
	)
	if token == "" {
		svc.Disable() // safe to run unconfigured: no network call happens
	}

	svc.Emit(context.Background(), "EXAMPLE_APP_STARTED", map[string]any{"example": true})

	// token is logged like any other field — logger redacts it automatically,
	// since "token" is one of its built-in sensitive keys.
	// configuredEndpoint is what was passed in — WithEuResidency(true) silently
	// overrides it inside Service, so it's not necessarily where events go.
	log.Info("analytics configured",
		"token", token,
		"configuredEndpoint", endpoint,
		"enabled", svc.IsEnabled(),
		"euResidency", values.Bool("euResidency"),
	)
}
