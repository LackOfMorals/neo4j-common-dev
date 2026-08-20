// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

// Command httpclient demonstrates how another Go app wires up the
// httpclient package: build a Service once with a base URL and default
// headers, then issue requests via Do. It runs against a local httptest
// server so it works offline and deterministically.
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/LackOfMorals/neo4j-common/httpclient"
)

func main() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fmt.Printf("server received: %s %s %q\n", r.Method, r.URL.Path, body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	svc := httpclient.New(srv.URL, 5*time.Second,
		httpclient.WithDefaultHeaders(map[string]string{"Content-Type": "application/json"}),
	)

	resp, data, err := svc.Do(context.Background(), http.MethodPost, "/events", nil, []byte(`{"event":"example"}`))
	if err != nil {
		fmt.Println("request failed:", err)
		return
	}
	fmt.Printf("client received: %d %s\n", resp.StatusCode, data)
}
