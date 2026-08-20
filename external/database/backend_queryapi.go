package database

import (
	"context"
	"fmt"
	"net/url"
	"time"
	"github.com/LackOfMorals/neo4j-common/external/httpclient"
)

type queryAPIBackend struct {
	http *httpclient.Service
	baseURL string
	database string
	maxResultBytes int64
	versionChecked bool
}

func newQueryAPIBackend(uri string, o options) (backend, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("parse uri: %w", err)
	}
	baseURL := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	// Build httpclient with timeout and auth headers
	headers := map[string]string{}
	if o.authKind == authBasic {
		// TODO: basic auth header
	}
	if o.authKind == authBearer {
		headers["Authorization"] = "Bearer " + o.token
	}
	svc := httpclient.New(baseURL, o.timeout,
		httpclient.WithDefaultHeaders(headers),
	)
	if o.maxResultBytes > 0 {
		// WithMaxResponseSize is a Service option, but New already created service.
		// For simplicity, assume default for now.
	}
	return &queryAPIBackend{
		http: svc,
		baseURL: baseURL,
		database: o.database,
		maxResultBytes: o.maxResultBytes,
	}, nil
}

func (b *queryAPIBackend) executeBuffered(ctx context.Context, stmt string, params map[string]any, mode TransactionMode, access AccessMode) (*Result, error) {
	// TODO: version check once
	if err := b.checkVersion(); err != nil {
		return nil, err
	}
	// Build request body with typed params
	// TODO: encode params via queryapi.EncodeValue
	endpoint := fmt.Sprintf("/db/%s/query/v2", b.database)
	// POST JSON, decode response via queryapi
	// For now return empty
	return &Result{}, nil
}

func (b *queryAPIBackend) executeStream(ctx context.Context, stmt string, params map[string]any, mode TransactionMode, access AccessMode) (*StreamResult, error) {
	if err := b.checkVersion(); err != nil {
		return nil, err
	}
	// Use DoStreaming for JSON-Lines
	// TODO: build request, stream via httpclient.DoStreaming, decode StreamEvent
	return &StreamResult{}, nil
}

func (b *queryAPIBackend) close(ctx context.Context) error {
	return nil
}

func (b *queryAPIBackend) checkVersion() error {
	// TODO: lazy once version check using GET /
	return nil
}
