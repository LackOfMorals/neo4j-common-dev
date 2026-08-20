// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

package httpclient_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LackOfMorals/neo4j-common/httpclient"
)

func TestDo(t *testing.T) {
	t.Run("sends the request body", func(t *testing.T) {
		var gotBody []byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		svc := httpclient.New(srv.URL, 0)
		_, _, err := svc.Do(context.Background(), http.MethodPost, "/echo", nil, []byte(`{"hello":"world"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(gotBody) != `{"hello":"world"}` {
			t.Errorf("expected the request body to reach the server, got %q", gotBody)
		}
	})

	t.Run("sends method and endpoint correctly", func(t *testing.T) {
		var gotMethod, gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		svc := httpclient.New(srv.URL, 0)
		_, _, err := svc.Do(context.Background(), http.MethodGet, "/things/1", nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotMethod != http.MethodGet {
			t.Errorf("unexpected method: got %s, want %s", gotMethod, http.MethodGet)
		}
		if gotPath != "/things/1" {
			t.Errorf("unexpected path: got %s, want /things/1", gotPath)
		}
	})

	t.Run("returns the response body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("response body"))
		}))
		defer srv.Close()

		svc := httpclient.New(srv.URL, 0)
		resp, data, err := svc.Do(context.Background(), http.MethodGet, "/", nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("unexpected status: got %d, want %d", resp.StatusCode, http.StatusOK)
		}
		if string(data) != "response body" {
			t.Errorf("unexpected body: got %q, want %q", data, "response body")
		}
	})

	t.Run("per-call headers override WithDefaultHeaders on collision", func(t *testing.T) {
		var gotAuth, gotAccept string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotAccept = r.Header.Get("Accept")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		svc := httpclient.New(srv.URL, 0, httpclient.WithDefaultHeaders(map[string]string{
			"Authorization": "default-token",
			"Accept":        "application/json",
		}))
		_, _, err := svc.Do(context.Background(), http.MethodGet, "/", map[string]string{
			"Authorization": "per-call-token",
		}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotAuth != "per-call-token" {
			t.Errorf("expected per-call header to win, got %q", gotAuth)
		}
		if gotAccept != "application/json" {
			t.Errorf("expected default header to still be present, got %q", gotAccept)
		}
	})

	t.Run("WithMaxResponseSize errors when the response exceeds the cap", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("0123456789"))
		}))
		defer srv.Close()

		svc := httpclient.New(srv.URL, 0, httpclient.WithMaxResponseSize(5))
		_, _, err := svc.Do(context.Background(), http.MethodGet, "/", nil, nil)
		if err == nil {
			t.Fatalf("expected an error for an oversized response, got nil")
		}
	})

	t.Run("WithHTTPClient injects a custom client", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		custom := &http.Client{}
		svc := httpclient.New(srv.URL, 0, httpclient.WithHTTPClient(custom))
		_, _, err := svc.Do(context.Background(), http.MethodGet, "/", nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
