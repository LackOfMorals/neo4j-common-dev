// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"time"
)

// DefaultMaxResponseSize is the default cap on a response body read by Do,
// used unless WithMaxResponseSize overrides it.
const DefaultMaxResponseSize = 10 * 1024 * 1024

// defaultTimeout is used when New is given a non-positive timeout and no
// WithHTTPClient override — every external call should have a timeout, so a
// zero value here means "use a sane default", not "wait forever".
const defaultTimeout = 30 * time.Second

// Service is a thin HTTP client wrapper with a base URL, a response-size
// cap, and optional default headers applied to every request.
type Service struct {
	baseURL         string
	client          *http.Client
	maxResponseSize int64
	defaultHeaders  map[string]string
}

// options holds the values Option functions configure before New builds a Service.
type options struct {
	httpClient      *http.Client
	maxResponseSize int64
	defaultHeaders  map[string]string
}

// Option configures a Service constructed by New.
type Option func(*options)

// WithHTTPClient injects a custom *http.Client (custom transport, proxying,
// tracing middleware, or a test double), overriding the timeout passed to New.
func WithHTTPClient(client *http.Client) Option {
	return func(o *options) { o.httpClient = client }
}

// WithMaxResponseSize overrides DefaultMaxResponseSize for this Service.
func WithMaxResponseSize(n int64) Option {
	return func(o *options) { o.maxResponseSize = n }
}

// WithDefaultHeaders registers headers sent on every request made by this
// Service, in addition to any headers passed to a specific Do call. A
// per-call header with the same name overrides the default for that call.
func WithDefaultHeaders(headers map[string]string) Option {
	return func(o *options) {
		if o.defaultHeaders == nil {
			o.defaultHeaders = make(map[string]string, len(headers))
		}
		maps.Copy(o.defaultHeaders, headers)
	}
}

// New creates a Service targeting baseURL. timeout configures the default
// http.Client's timeout; it is ignored if WithHTTPClient is also given, and
// a non-positive value falls back to defaultTimeout rather than disabling
// the timeout entirely.
func New(baseURL string, timeout time.Duration, opts ...Option) *Service {
	o := options{maxResponseSize: DefaultMaxResponseSize}
	for _, opt := range opts {
		opt(&o)
	}

	client := o.httpClient
	if client == nil {
		if timeout <= 0 {
			timeout = defaultTimeout
		}
		client = &http.Client{Timeout: timeout}
	}

	return &Service{
		baseURL:         baseURL,
		client:          client,
		maxResponseSize: o.maxResponseSize,
		defaultHeaders:  o.defaultHeaders,
	}
}

// Do sends an HTTP request to baseURL+endpoint and returns the raw response
// alongside its body, read up to this Service's max response size. headers
// are merged over any WithDefaultHeaders values, taking precedence on a
// shared header name. body is the request body, and may be nil/empty for
// methods like GET that don't send one.
func (s *Service) Do(ctx context.Context, method, endpoint string, headers map[string]string, body []byte) (*http.Response, []byte, error) {
	url := s.baseURL + endpoint

	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, nil, err
	}

	for k, v := range s.defaultHeaders {
		req.Header.Set(k, v)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, s.maxResponseSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return resp, nil, fmt.Errorf("read response: %w", err)
	}
	if int64(len(data)) > s.maxResponseSize {
		return resp, nil, fmt.Errorf("response body exceeds maximum size of %d bytes", s.maxResponseSize)
	}
	return resp, data, nil
}
