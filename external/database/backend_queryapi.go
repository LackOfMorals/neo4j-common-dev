package database

import (
	"context"
	"github.com/LackOfMorals/neo4j-common/external/httpclient"
	"github.com/LackOfMorals/neo4j-common/internal/database/queryapi"
)

type queryAPIBackend struct {
	http *httpclient.Service
	baseURL string
	database string
	maxResultBytes int64
}

func newQueryAPIBackend(uri string, o options) (backend, error) {
	// TODO: parse uri, build httpclient.Service with timeout and auth
	return &queryAPIBackend{}, nil
}

func (b *queryAPIBackend) executeBuffered(ctx context.Context, stmt string, params map[string]any, mode TransactionMode, access AccessMode) (*Result, error) {
	// TODO: build ExecuteRequest, call httpclient, decode via queryapi.DecodeValue, map to public types
	return &Result{}, nil
}

func (b *queryAPIBackend) executeStream(ctx context.Context, stmt string, params map[string]any, mode TransactionMode, access AccessMode) (*StreamResult, error) {
	// TODO: use DoStreaming, stream JSON-Lines via queryapi.StreamEvent, decode rows
	return &StreamResult{}, nil
}

func (b *queryAPIBackend) close(ctx context.Context) error {
	return nil
}

// version check placeholder
func (b *queryAPIBackend) checkVersion() error {
	// TODO: GET / with queryapi.VersionInfo, compare cutoff
	return nil
}
