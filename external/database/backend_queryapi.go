package database

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/LackOfMorals/neo4jPackages/external/httpclient"
	"github.com/LackOfMorals/neo4jPackages/internal/database/queryapi"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type queryAPIBackend struct {
	http           *httpclient.Service
	baseURL        string
	database       string
	maxResultBytes int64
	versionOnce    sync.Once
	versionErr     error
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
	} else if o.authKind == authBasic {
		cred := base64.StdEncoding.EncodeToString([]byte(o.username + ":" + o.password))
		headers["Authorization"] = "Basic " + cred
	}
	opts := []httpclient.Option{
		httpclient.WithDefaultHeaders(headers),
	}
	if o.maxResultBytes > 0 {
		opts = append(opts, httpclient.WithMaxResponseSize(o.maxResultBytes))
	}
	svc := httpclient.New(baseURL, o.timeout, opts...)
	return &queryAPIBackend{
		http:           svc,
		baseURL:        baseURL,
		database:       o.database,
		maxResultBytes: o.maxResultBytes,
	}, nil
}

func (b *queryAPIBackend) executeBuffered(ctx context.Context, stmt string, params map[string]any, mode TransactionMode, access AccessMode, database string) (*Result, error) {
	if err := b.checkVersion(); err != nil {
		return nil, err
	}
	db := b.database
	if database != "" {
		db = database
	}
	if mode == Explicit {
		// Explicit single-statement transaction with proper commit error handling
		tx, err := b.beginTx(ctx)
		if err != nil {
			return nil, err
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback(ctx)
			}
		}()
		res, err := tx.Run(ctx, stmt, params)
		if err != nil {
			return nil, err
		}
		commitRes, err := tx.Commit(ctx)
		if err != nil {
			// Wrap in CommitError with ambiguous flag for transport errors
			if _, ok := err.(*TransportError); ok {
				return nil, &CommitError{TransactionID: tx.id, Ambiguous: true, Err: err}
			}
			return nil, &CommitError{TransactionID: tx.id, Ambiguous: false, Err: err}
		}
		committed = true
		res.Summary.Bookmarks = commitRes.Bookmarks
		return res, nil
	}
	encodedParams := make(map[string]json.RawMessage, len(params))
	for k, v := range params {
		raw, err := queryapi.EncodeValue(v)
		if err != nil {
			return nil, fmt.Errorf("encode param %s: %w", k, err)
		}
		encodedParams[k] = raw
	}
	req := map[string]any{
		"statement":  stmt,
		"parameters": encodedParams,
	}
	if access == Read {
		req["accessMode"] = "Read"
	}
	body, _ := json.Marshal(req)
	endpoint := fmt.Sprintf("/db/%s/query/v2", db)
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/vnd.neo4j.query.v1.1",
	}
	resp, respBody, err := b.http.Do(ctx, "POST", endpoint, headers, body)
	if err != nil {
		return nil, err
	}
	if resp != nil && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return nil, &TransportError{StatusCode: resp.StatusCode, Body: respBody, Err: fmt.Errorf("query request failed with status %d", resp.StatusCode)}
	}
	var qr queryapi.QueryResponse
	if err := json.Unmarshal(respBody, &qr); err != nil {
		return nil, fmt.Errorf("decode query response: %w", err)
	}
	if len(qr.Errors) > 0 {
		e := qr.Errors[0]
		return nil, &StatementError{Code: e.Code, Message: e.Message}
	}
	result := &Result{
		Keys: qr.Data.Fields,
	}
	summary := Summary{
		Database:  db,
		Bookmarks: qr.Bookmarks,
	}
	if qr.QueryType != "" {
		switch qr.QueryType {
		case "r":
			summary.QueryType = QueryTypeRead
		case "w":
			summary.QueryType = QueryTypeWrite
		case "rw":
			summary.QueryType = QueryTypeReadWrite
		case "s":
			summary.QueryType = QueryTypeSchema
		}
	}
	result.Summary = summary
	for _, rowVals := range qr.Data.Values {
		values := make([]any, len(rowVals))
		for i, raw := range rowVals {
			v, err := queryapi.DecodeValue(raw)
			if err != nil {
				return nil, fmt.Errorf("decode row value: %w", err)
			}
			values[i] = mapQueryValue(v)
		}
		result.Records = append(result.Records, Record{keys: qr.Data.Fields, values: values})
	}
	return result, nil
}

func mapQueryValue(v any) any {
	switch x := v.(type) {
	case queryapi.Node:
		return Node{ElementID: x.ElementID, Labels: x.Labels, Properties: x.Properties}
	case queryapi.Relationship:
		return Relationship{ElementID: x.ElementID, StartElementID: x.StartElementID, EndElementID: x.EndElementID, Type: x.Type, Properties: x.Properties}
	case queryapi.Point:
		return Point{SRID: x.SRID, X: x.X, Y: x.Y, Z: x.Z}
	case queryapi.Duration:
		return Duration{Months: x.Months, Days: x.Days, Seconds: x.Seconds, Nanos: x.Nanos}
	case queryapi.Vector:
		return Vector{Values: x.Values}
	case queryapi.Date:
		return x
	case queryapi.LocalTime:
		return x
	case queryapi.Time:
		return x
	case queryapi.LocalDateTime:
		return x
	case queryapi.DateTime:
		return x
	default:
		return v
	}
}

func (b *queryAPIBackend) executeStream(ctx context.Context, stmt string, params map[string]any, mode TransactionMode, access AccessMode, database string) (*StreamResult, error) {
	if err := b.checkVersion(); err != nil {
		return nil, err
	}
	db := b.database
	if database != "" {
		db = database
	}
	if mode == Explicit {
		// Explicit mode: lazy streaming transaction
		// Begin transaction
		beginEndpoint := fmt.Sprintf("/db/%s/query/v2/tx", db)
		beginHeaders := map[string]string{"Content-Type": "application/json", "Accept": "application/json"}
		beginResp, beginBody, err := b.http.Do(ctx, "POST", beginEndpoint, beginHeaders, nil)
		if err != nil {
			return nil, err
		}
		if beginResp.StatusCode < 200 || beginResp.StatusCode >= 300 {
			return nil, &TransportError{StatusCode: beginResp.StatusCode, Body: beginBody, Err: fmt.Errorf("begin tx failed with status %d", beginResp.StatusCode)}
		}
		var beginResult struct {
			Transaction struct {
				ID string `json:"id"`
			} `json:"transaction"`
		}
		if err := json.Unmarshal(beginBody, &beginResult); err != nil {
			return nil, fmt.Errorf("begin tx decode: %w", err)
		}
		txID := beginResult.Transaction.ID
		if txID == "" {
			return nil, fmt.Errorf("begin tx: no transaction id")
		}
		// Encode params
		encodedParams := make(map[string]json.RawMessage, len(params))
		for k, v := range params {
			raw, err := queryapi.EncodeValue(v)
			if err != nil {
				return nil, fmt.Errorf("encode param %s: %w", k, err)
			}
			encodedParams[k] = raw
		}
		reqBody := map[string]any{"statement": stmt, "parameters": encodedParams}
		if access == Read {
			reqBody["accessMode"] = "Read"
		}
		body, _ := json.Marshal(reqBody)
		endpoint := fmt.Sprintf("/db/%s/query/v2/tx/%s", db, txID)
		headers := map[string]string{"Content-Type": "application/json", "Accept": "application/vnd.neo4j.query.v1.1+jsonl"}
		resp, err := b.http.DoStreaming(ctx, "POST", endpoint, headers, body)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			buf := make([]byte, 4096)
			n, _ := resp.Body.Read(buf)
			resp.Body.Close()
			return nil, &TransportError{StatusCode: resp.StatusCode, Body: buf[:n], Err: fmt.Errorf("stream tx run failed with status %d", resp.StatusCode)}
		}
		// Stream parsing
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
		if !scanner.Scan() {
			resp.Body.Close()
			return nil, fmt.Errorf("stream: no data")
		}
		var ev struct {
			Event string          `json:"$event"`
			Body  json.RawMessage `json:"_body"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("stream header decode: %w", err)
		}
		if ev.Event != "Header" {
			resp.Body.Close()
			return nil, fmt.Errorf("stream: expected Header event, got %s", ev.Event)
		}
		var header struct {
			Fields []string `json:"fields"`
		}
		if err := json.Unmarshal(ev.Body, &header); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("stream header body decode: %w", err)
		}
		keys := header.Fields
		var summary Summary
		streamErr := error(nil)
		records := []Record{}
		// We will stream lazily via iterator, but for simplicity we buffer records here and commit on close.
		// To keep it lazy, we return a StreamResult with a custom closeFn that commits/rollbacks.
		for scanner.Scan() {
			line := scanner.Bytes()
			var e struct {
				Event string          `json:"$event"`
				Body  json.RawMessage `json:"_body"`
			}
			if err := json.Unmarshal(line, &e); err != nil {
				streamErr = fmt.Errorf("stream event decode: %w", err)
				break
			}
			switch e.Event {
			case "Record":
				var vals []json.RawMessage
				if err := json.Unmarshal(e.Body, &vals); err != nil {
					streamErr = fmt.Errorf("record decode: %w", err)
					break
				}
				values := make([]any, len(vals))
				for i, raw := range vals {
					v, err := queryapi.DecodeValue(raw)
					if err != nil {
						streamErr = fmt.Errorf("decode value: %w", err)
						break
					}
					values[i] = mapQueryValue(v)
				}
				if streamErr != nil {
					break
				}
				records = append(records, Record{keys: keys, values: values})
			case "Summary":
				var s struct {
					QueryType string `json:"queryType"`
					Counters  struct {
						NodesCreated         int `json:"nodesCreated"`
						NodesDeleted         int `json:"nodesDeleted"`
						RelationshipsCreated int `json:"relationshipsCreated"`
						RelationshipsDeleted int `json:"relationshipsDeleted"`
						PropertiesSet        int `json:"propertiesSet"`
					} `json:"counters"`
				}
				_ = json.Unmarshal(e.Body, &s)
				summary.QueryType = QueryType(s.QueryType)
				summary.Counters = Counters{
					NodesCreated:         s.Counters.NodesCreated,
					NodesDeleted:         s.Counters.NodesDeleted,
					RelationshipsCreated: s.Counters.RelationshipsCreated,
					RelationshipsDeleted: s.Counters.RelationshipsDeleted,
					PropertiesSet:        s.Counters.PropertiesSet,
				}
				break
			case "Error":
				streamErr = fmt.Errorf("stream error event")
				break
			}
			if streamErr != nil {
				break
			}
		}
		if scanner.Err() != nil {
			streamErr = fmt.Errorf("scan error: %w", scanner.Err())
		}
		// CloseFn will commit if no error, else rollback
		committed := false
		closeFn := func() error {
			if committed {
				return nil
			}
			if streamErr != nil {
				// rollback
				_, _, _ = b.http.Do(ctx, "DELETE", fmt.Sprintf("/db/%s/query/v2/tx/%s", db, txID), nil, nil)
				return streamErr
			}
			// commit
			commitEndpoint := fmt.Sprintf("/db/%s/query/v2/tx/%s/commit", db, txID)
			commitResp, commitBody, err := b.http.Do(ctx, "POST", commitEndpoint, map[string]string{"Content-Type": "application/json", "Accept": "application/json"}, nil)
			if err != nil {
				return err
			}
			if commitResp.StatusCode < 200 || commitResp.StatusCode >= 300 {
				return &TransportError{StatusCode: commitResp.StatusCode, Body: commitBody, Err: fmt.Errorf("commit failed")}
			}
			committed = true
			return nil
		}
		return &StreamResult{
			keys:    keys,
			records: records,
			summary: summary,
			closeFn: closeFn,
		}, nil
	}
	encodedParams := make(map[string]json.RawMessage, len(params))
	for k, v := range params {
		raw, err := queryapi.EncodeValue(v)
		if err != nil {
			return nil, fmt.Errorf("encode param %s: %w", k, err)
		}
		encodedParams[k] = raw
	}
	reqBody := map[string]any{
		"statement":  stmt,
		"parameters": encodedParams,
	}
	if access == Read {
		reqBody["accessMode"] = "Read"
	}
	body, _ := json.Marshal(reqBody)
	endpoint := fmt.Sprintf("/db/%s/query/v2", db)
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/vnd.neo4j.query.v1.1+jsonl",
	}
	resp, err := b.http.DoStreaming(ctx, "POST", endpoint, headers, body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read limited body for diagnostics
		buf := make([]byte, 4096)
		n, _ := resp.Body.Read(buf)
		resp.Body.Close()
		return nil, &TransportError{StatusCode: resp.StatusCode, Body: buf[:n], Err: fmt.Errorf("stream request failed with status %d", resp.StatusCode)}
	}
	// Ensure body is closed on StreamResult.Close
	defer func() {
		// If we return early due to error, close body
	}()
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

	// Read Header event
	if !scanner.Scan() {
		resp.Body.Close()
		return nil, fmt.Errorf("stream: no data")
	}
	var ev struct {
		Event string          `json:"$event"`
		Body  json.RawMessage `json:"_body"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("stream header decode: %w", err)
	}
	if ev.Event != "Header" {
		resp.Body.Close()
		return nil, fmt.Errorf("stream: expected Header event, got %s", ev.Event)
	}
	var header struct {
		Fields []string `json:"fields"`
	}
	if err := json.Unmarshal(ev.Body, &header); err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("stream header body decode: %w", err)
	}
	keys := header.Fields

	records := []Record{}
	var summary Summary
	var streamErr error

	for scanner.Scan() {
		line := scanner.Bytes()
		var e struct {
			Event string          `json:"$event"`
			Body  json.RawMessage `json:"_body"`
		}
		if err := json.Unmarshal(line, &e); err != nil {
			streamErr = fmt.Errorf("stream event decode: %w", err)
			break
		}
		switch e.Event {
		case "Record":
			var vals []json.RawMessage
			if err := json.Unmarshal(e.Body, &vals); err != nil {
				streamErr = fmt.Errorf("record decode: %w", err)
				break
			}
			values := make([]any, len(vals))
			for i, raw := range vals {
				v, err := queryapi.DecodeValue(raw)
				if err != nil {
					streamErr = fmt.Errorf("decode value: %w", err)
					break
				}
				values[i] = mapQueryValue(v)
			}
			if streamErr != nil {
				break
			}
			records = append(records, Record{keys: keys, values: values})
		case "Summary":
			var s struct {
				QueryType string `json:"queryType"`
				Counters  struct {
					NodesCreated         int `json:"nodesCreated"`
					NodesDeleted         int `json:"nodesDeleted"`
					RelationshipsCreated int `json:"relationshipsCreated"`
					RelationshipsDeleted int `json:"relationshipsDeleted"`
					PropertiesSet        int `json:"propertiesSet"`
				} `json:"counters"`
				ResultAvailableAfter int64    `json:"resultAvailableAfter"`
				ResultConsumedAfter  int64    `json:"resultConsumedAfter"`
				Bookmarks            []string `json:"bookmarks"`
				Database             string   `json:"database"`
			}
			if err := json.Unmarshal(e.Body, &s); err == nil {
				var qt QueryType
				switch s.QueryType {
				case "r":
					qt = QueryTypeRead
				case "w":
					qt = QueryTypeWrite
				case "rw":
					qt = QueryTypeReadWrite
				case "s":
					qt = QueryTypeSchema
				default:
					qt = QueryTypeUnknown
				}
				summary = Summary{
					QueryType: qt,
					Counters: Counters{
						NodesCreated:         s.Counters.NodesCreated,
						NodesDeleted:         s.Counters.NodesDeleted,
						RelationshipsCreated: s.Counters.RelationshipsCreated,
						RelationshipsDeleted: s.Counters.RelationshipsDeleted,
						PropertiesSet:        s.Counters.PropertiesSet,
					},
					ResultAvailableAfter: time.Duration(s.ResultAvailableAfter),
					ResultConsumedAfter:  time.Duration(s.ResultConsumedAfter),
					Database:             s.Database,
					Bookmarks:            s.Bookmarks,
				}
			}
		case "Error":
			streamErr = fmt.Errorf("server error event")
		}
		if streamErr != nil {
			break
		}
	}
	if err := scanner.Err(); err != nil && streamErr == nil {
		streamErr = fmt.Errorf("stream scan error: %w", err)
	}

	res := &StreamResult{
		keys:    keys,
		records: records,
		summary: summary,
		closeFn: func() error {
			return resp.Body.Close()
		},
	}
	if streamErr != nil {
		// Close body before returning error
		resp.Body.Close()
		return nil, streamErr
	}
	return res, nil
}

func (b *queryAPIBackend) close(ctx context.Context) error {
	return nil
}

type txBackend struct {
	qb *queryAPIBackend
	id string
}

func (t *txBackend) txRun(ctx context.Context, stmt string, params map[string]any) (*Result, error) {
	encodedParams := make(map[string]json.RawMessage, len(params))
	for k, v := range params {
		raw, err := queryapi.EncodeValue(v)
		if err != nil {
			return nil, fmt.Errorf("encode param %s: %w", k, err)
		}
		encodedParams[k] = raw
	}
	req := map[string]any{"statement": stmt, "parameters": encodedParams}
	body, _ := json.Marshal(req)
	endpoint := fmt.Sprintf("/db/%s/query/v2/tx/%s", t.qb.database, t.id)
	headers := map[string]string{"Content-Type": "application/json", "Accept": "application/json"}
	resp, respBody, err := t.qb.http.Do(ctx, "POST", endpoint, headers, body)
	if err != nil {
		return nil, err
	}
	if resp != nil && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return nil, &TransportError{StatusCode: resp.StatusCode, Body: respBody, Err: fmt.Errorf("tx run failed with status %d", resp.StatusCode)}
	}
	var qr queryapi.QueryResponse
	if err := json.Unmarshal(respBody, &qr); err != nil {
		return nil, fmt.Errorf("decode tx run response: %w", err)
	}
	if len(qr.Errors) > 0 {
		return nil, &StatementError{Code: qr.Errors[0].Code, Message: qr.Errors[0].Message}
	}
	res := &Result{Keys: qr.Data.Fields}
	for _, rowVals := range qr.Data.Values {
		values := make([]any, len(rowVals))
		for i, raw := range rowVals {
			v, err := queryapi.DecodeValue(raw)
			if err != nil {
				return nil, err
			}
			values[i] = mapQueryValue(v)
		}
		res.Records = append(res.Records, Record{keys: qr.Data.Fields, values: values})
	}
	return res, nil
}

func (t *txBackend) txCommit(ctx context.Context) (*CommitResult, error) {
	endpoint := fmt.Sprintf("/db/%s/query/v2/tx/%s/commit", t.qb.database, t.id)
	headers := map[string]string{"Content-Type": "application/json", "Accept": "application/json"}
	resp, body, err := t.qb.http.Do(ctx, "POST", endpoint, headers, nil)
	if err != nil {
		return nil, err
	}
	if resp != nil && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return nil, &TransportError{StatusCode: resp.StatusCode, Body: body, Err: fmt.Errorf("tx commit failed with status %d", resp.StatusCode)}
	}
	var result struct {
		Bookmarks []string `json:"bookmarks"`
	}
	_ = json.Unmarshal(body, &result)
	return &CommitResult{Bookmarks: result.Bookmarks}, nil
}

func (t *txBackend) txRollback(ctx context.Context) error {
	endpoint := fmt.Sprintf("/db/%s/query/v2/tx/%s", t.qb.database, t.id)
	_, _, err := t.qb.http.Do(ctx, "DELETE", endpoint, nil, nil)
	return err
}

func (b *queryAPIBackend) beginTx(ctx context.Context) (*Tx, error) {
	endpoint := fmt.Sprintf("/db/%s/query/v2/tx", b.database)
	headers := map[string]string{"Content-Type": "application/json", "Accept": "application/json"}
	resp, body, err := b.http.Do(ctx, "POST", endpoint, headers, nil)
	if err != nil {
		return nil, err
	}
	if resp != nil && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return nil, &TransportError{StatusCode: resp.StatusCode, Body: body, Err: fmt.Errorf("begin tx failed with status %d", resp.StatusCode)}
	}
	var result struct {
		Transaction struct {
			ID string `json:"id"`
		} `json:"transaction"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("begin tx decode: %w", err)
	}
	if result.Transaction.ID == "" {
		return nil, fmt.Errorf("begin tx: no transaction id in response")
	}
	return &Tx{id: result.Transaction.ID, backend: &txBackend{qb: b, id: result.Transaction.ID}}, nil
}

func (b *queryAPIBackend) checkVersion() error {
	b.versionOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, body, err := b.http.Do(ctx, "GET", "/", nil, nil)
		if err != nil {
			b.versionErr = &TransportError{Err: err}
			return
		}
		var info struct {
			Neo4jVersion string `json:"neo4j_version"`
		}
		if err := json.Unmarshal(body, &info); err != nil {
			b.versionErr = fmt.Errorf("parse version info: %w", err)
			return
		}
		if !meetsVersionCutoff(info.Neo4jVersion) {
			b.versionErr = &VersionError{ServerVersion: info.Neo4jVersion, MinRequired: "2026.07"}
			return
		}
	})
	return b.versionErr
}

func parseCalVer(v string) ([3]int, error) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return [3]int{}, fmt.Errorf("invalid calver %q", v)
	}
	var res [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, fmt.Errorf("invalid calver %q", v)
		}
		res[i] = n
	}
	return res, nil
}

func compareCalVer(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

func meetsVersionCutoff(v string) bool {
	// Try CalVer first: YEAR.MONTH.PATCH where YEAR >= 2000
	if cal, err := parseCalVer(v); err == nil && cal[0] >= 2000 {
		min, _ := parseCalVer("2026.07.0")
		return compareCalVer(cal, min) >= 0
	}
	// Fall back to legacy SemVer
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	if major > 5 {
		return true
	}
	if major == 5 && minor >= 27 {
		return true
	}
	return false
}
