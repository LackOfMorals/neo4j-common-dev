package database

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"github.com/LackOfMorals/neo4j-common/external/httpclient"
	"github.com/LackOfMorals/neo4j-common/internal/database/queryapi"
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
	headers := map[string]string{}
	if o.authKind == authBearer {
		headers["Authorization"] = "Bearer " + o.token
	}
	svc := httpclient.New(baseURL, o.timeout,
		httpclient.WithDefaultHeaders(headers),
	)
	return &queryAPIBackend{
		http: svc,
		baseURL: baseURL,
		database: o.database,
		maxResultBytes: o.maxResultBytes,
	}, nil
}

func (b *queryAPIBackend) executeBuffered(ctx context.Context, stmt string, params map[string]any, mode TransactionMode, access AccessMode) (*Result, error) {
	if err := b.checkVersion(); err != nil {
		return nil, err
	}
	// Encode params using queryapi.EncodeValue
	encodedParams := make(map[string]json.RawMessage, len(params))
	for k, v := range params {
		raw, err := queryapi.EncodeValue(v)
		if err != nil {
			return nil, fmt.Errorf("encode param %s: %w", k, err)
		}
		encodedParams[k] = raw
	}
	reqBody := map[string]any{
		"statements": []map[string]any{
			{
				"statement": stmt,
				"parameters": encodedParams,
			},
		},
	}
	body, _ := json.Marshal(reqBody)
	endpoint := fmt.Sprintf("/db/%s/query/v2", b.database)
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept": "application/vnd.neo4j.query.v1.2+json",
	}
	_, respBody, err := b.http.Do(ctx, "POST", endpoint, headers, body)
	if err != nil {
		return nil, err
	}
	// TODO: decode respBody via queryapi.QueryResponse, map to public Result
	_ = respBody
	return &Result{}, nil
}

func (b *queryAPIBackend) executeStream(ctx context.Context, stmt string, params map[string]any, mode TransactionMode, access AccessMode) (*StreamResult, error) {
	if err := b.checkVersion(); err != nil {
		return nil, err
	}
	// Encode params
	encodedParams := make(map[string]json.RawMessage, len(params))
	for k, v := range params {
		raw, err := queryapi.EncodeValue(v)
		if err != nil { return nil, err }
		encodedParams[k] = raw
	}
	reqBody := map[string]any{
		"statements": []map[string]any{
			{"statement": stmt, "parameters": encodedParams},
		},
	}
	body, _ := json.Marshal(reqBody)
	endpoint := fmt.Sprintf("/db/%s/query/v2", b.database)
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept": "application/vnd.neo4j.query.v1.2+jsonl",
	}
	resp, err := b.http.DoStreaming(ctx, "POST", endpoint, headers, body)
	if err != nil {
		return nil, err
	}
	// TODO: stream decode via queryapi.StreamEvent and DecodeValue
	// For now return empty
	return &StreamResult{}, nil
}

func (b *queryAPIBackend) close(ctx context.Context) error {
	return nil
}

func (b *queryAPIBackend) checkVersion() error {
	// TODO: lazy once version check
	return nil
}
