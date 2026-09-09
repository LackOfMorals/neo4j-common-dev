package queryapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (s *Service) Execute(ctx context.Context, req ExecuteRequest) (*QueryResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	url := fmt.Sprintf("%s/db/%s/query/v2", s.BaseURL, s.Database)
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	hr.Header.Set("Content-Type", "application/json")
	hr.Header.Set("Accept", "application/vnd.neo4j.query.v1.1")
	if s.AuthHeader != "" {
		hr.Header.Set("Authorization", s.AuthHeader)
	}
	resp, err := s.HTTPClient.Do(hr)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("query request failed with status %d: %s", resp.StatusCode, string(data))
	}
	var qr QueryResponse
	if err := json.Unmarshal(data, &qr); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &qr, nil
}

func (s *Service) ExecuteStream(ctx context.Context, req ExecuteRequest) (<-chan StreamEvent, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	url := fmt.Sprintf("%s/db/%s/query/v2", s.BaseURL, s.Database)
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	hr.Header.Set("Content-Type", "application/json")
	hr.Header.Set("Accept", "application/vnd.neo4j.query.v1.1+jsonl")
	if s.AuthHeader != "" {
		hr.Header.Set("Authorization", s.AuthHeader)
	}
	resp, err := s.HTTPClient.Do(hr)
	if err != nil {
		return nil, err
	}
	ch := make(chan StreamEvent)
	go func() {
		defer close(ch)
		defer func() { _ = resp.Body.Close() }()
		dec := json.NewDecoder(resp.Body)
		for {
			var ev StreamEvent
			if err := dec.Decode(&ev); err != nil {
				if err == io.EOF {
					break
				}
				break
			}
			select {
			case ch <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

// QuerySummary is the body of a stream Summary event. Timings are in
// milliseconds; notifications use the same wire shape as the buffered
// response's top-level notifications array.
type QuerySummary struct {
	QueryType string `json:"queryType"`
	Counters  struct {
		NodesCreated         int `json:"nodesCreated"`
		NodesDeleted         int `json:"nodesDeleted"`
		RelationshipsCreated int `json:"relationshipsCreated"`
		RelationshipsDeleted int `json:"relationshipsDeleted"`
		PropertiesSet        int `json:"propertiesSet"`
		LabelsAdded          int `json:"labelsAdded"`
		LabelsRemoved        int `json:"labelsRemoved"`
		IndexesAdded         int `json:"indexesAdded"`
		IndexesRemoved       int `json:"indexesRemoved"`
		ConstraintsAdded     int `json:"constraintsAdded"`
		ConstraintsRemoved   int `json:"constraintsRemoved"`
	} `json:"counters"`
	ResultAvailableAfter int64              `json:"resultAvailableAfter"`
	ResultConsumedAfter  int64              `json:"resultConsumedAfter"`
	Notifications        []NotificationWire `json:"notifications"`
	Bookmarks            []string           `json:"bookmarks"`
	Database             string             `json:"database"`
}

// StreamResult is a scanner-based streaming result matching the facade's StreamResult shape.
type StreamResult struct {
	keys    []string
	scanner *bufio.Scanner
	body    io.ReadCloser
	closed  bool
	summary *QuerySummary
}

func (r *StreamResult) Keys() []string { return r.keys }
func (r *StreamResult) Scan() bool {
	if !r.scanner.Scan() {
		return false
	}
	var ev StreamEvent
	if err := json.Unmarshal(r.scanner.Bytes(), &ev); err != nil {
		return false
	}
	if ev.Event == "Summary" {
		var s QuerySummary
		if err := json.Unmarshal(ev.Body, &s); err == nil {
			r.summary = &s
		}
		// rewind scanner? We already consumed it. Simpler: return event as Summary.
		// Keep event available via Event()
	}
	return true
}
func (r *StreamResult) Event() StreamEvent {
	var ev StreamEvent
	if err := json.Unmarshal(r.scanner.Bytes(), &ev); err != nil {
		return StreamEvent{}
	}
	return ev
}
func (r *StreamResult) Summary() *QuerySummary { return r.summary }
func (r *StreamResult) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return r.body.Close()
}

func (s *Service) ExecuteStreamResult(ctx context.Context, req ExecuteRequest) (*StreamResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	url := fmt.Sprintf("%s/db/%s/query/v2", s.BaseURL, s.Database)
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	hr.Header.Set("Content-Type", "application/json")
	hr.Header.Set("Accept", "application/vnd.neo4j.query.v1.1+jsonl")
	if s.AuthHeader != "" {
		hr.Header.Set("Authorization", s.AuthHeader)
	}
	resp, err := s.HTTPClient.Do(hr)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("stream request failed with status %d", resp.StatusCode)
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	if !scanner.Scan() {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("stream: no data")
	}
	var ev StreamEvent
	if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("stream header decode: %w", err)
	}
	if ev.Event != "Header" {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("stream: expected Header event, got %s", ev.Event)
	}
	var header struct {
		Fields []string `json:"fields"`
	}
	if err := json.Unmarshal(ev.Body, &header); err != nil {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("stream header body decode: %w", err)
	}
	return &StreamResult{
		keys:    header.Fields,
		scanner: scanner,
		body:    resp.Body,
	}, nil
}
