// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

package analytics

import (
	"io"
	"net/http"
)

// HTTPClient is the subset of *http.Client used by Service, allowing injection
// of a custom transport (e.g. via httptest.Server or a hand-written stub) in tests.
type HTTPClient interface {
	Post(url, contentType string, body io.Reader) (*http.Response, error)
}
