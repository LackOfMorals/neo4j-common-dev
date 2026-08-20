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

	"github.com/LackOfMorals/neo4j-common/analytics"
	"github.com/LackOfMorals/neo4j-common/config"
	"github.com/LackOfMorals/neo4j-common/logger"
)

func main() {
	cfg := config.New(
		config.Field{Key: "token", Flag: "mixpanel-token", EnvVar: "MIXPANEL_TOKEN", Description: "Mixpanel project token"},
		config.Field{Key: "endpoint", Flag: "mixpanel-endpoint", EnvVar: "MIXPANEL_ENDPOINT", Description: "Mixpanel API endpoint", Default: "https://api.mixpanel.com"},
		config.Field{Key: "uri", Flag: "neo4j-uri", EnvVar: "NEO4J_URI", Description: "Neo4j connection URI", Default: "bolt://localhost:7687"},
		config.Field{Key: "disableGeoTracking", Flag: "disable-geo-tracking", EnvVar: "MIXPANEL_DISABLE_GEO_TRACKING", Description: "opt every event out of Mixpanel's IP-based geolocation", Default: "false"},
		config.Field{Key: "euResidency", Flag: "eu-residency", EnvVar: "MIXPANEL_EU_RESIDENCY", Description: "send events to Mixpanel's EU-residency endpoint", Default: "false"},
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
	uri := values.String("uri")

	svc := analytics.New(token, endpoint, "sendEvent-example", uri,
		analytics.WithCommonProperties(map[string]any{"app_version": "1.0.0"}), // optional
		analytics.WithGeoIPTracking(!values.Bool("disableGeoTracking")),        // opt-out toggle
		analytics.WithEuResidency(values.Bool("euResidency")),                  // overrides endpoint when true
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
		"geoIPTracking", svc.IsGeoIPTrackingEnabled(),
		"euResidency", values.Bool("euResidency"),
	)
}
