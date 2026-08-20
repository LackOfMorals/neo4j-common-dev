// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

// Command httpclient demonstrates how another Go app wires up the
// httpclient package: resolve settings via config, log via logger, build a
// Service once with a base URL and default headers, then issue requests
// via Do. With no base URL configured, it runs against a local httptest
// server so it works offline and deterministically.
//
// Try it with:
//
//	go run ./examples/httpclient
//	go run ./examples/httpclient --base-url https://example.com --log-level debug
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/LackOfMorals/neo4j-common/config"
	"github.com/LackOfMorals/neo4j-common/httpclient"
	"github.com/LackOfMorals/neo4j-common/logger"
)

func main() {
	cfg := config.New(
		config.Field{Key: "baseURL", Flag: "base-url", EnvVar: "HTTPCLIENT_BASE_URL", Description: "base URL to call instead of the built-in demo server"},
		config.Field{Key: "level", Flag: "log-level", EnvVar: "LOG_LEVEL", Description: "log level", Default: "info"},
	)
	values, err := cfg.Read(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	log := logger.New(values.String("level"), "json", os.Stdout)

	baseURL := values.String("baseURL")
	if baseURL == "" {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Info("demo server received request", "method", r.Method, "path", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}))
		defer srv.Close()
		baseURL = srv.URL
		log.Info("no base URL configured, using the built-in demo server", "url", baseURL)
	}

	svc := httpclient.New(baseURL, 5*time.Second,
		httpclient.WithDefaultHeaders(map[string]string{"Content-Type": "application/json"}),
	)

	resp, data, err := svc.Do(context.Background(), http.MethodPost, "/events", nil, []byte(`{"event":"example"}`))
	if err != nil {
		log.Error("request failed", "error", err.Error())
		os.Exit(1)
	}
	log.Info("request succeeded", "status", resp.StatusCode, "body", string(data))
}
