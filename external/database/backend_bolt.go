package database

import (
	"context"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v6/neo4j"
)

type boltBackend struct {
	driver   neo4j.Driver
	database string
}

func newBoltBackend(uri string, o options) (backend, error) {
	var auth neo4j.AuthToken
	switch o.authKind {
	case authBasic:
		auth = neo4j.BasicAuth(o.username, o.password, "")
	case authBearer:
		auth = neo4j.BearerAuth(o.token)
	default:
		auth = neo4j.NoAuth()
	}
	driver, err := neo4j.NewDriver(uri, auth)
	if err != nil {
		return nil, fmt.Errorf("new driver: %w", err)
	}
	return &boltBackend{
		driver:   driver,
		database: o.database,
	}, nil
}

func (b *boltBackend) executeBuffered(ctx context.Context, stmt string, params map[string]any, mode TransactionMode, access AccessMode, database string) (*Result, error) {
	db := b.database
	if database != "" {
		db = database
	}
	opts := []neo4j.ExecuteQueryConfigurationOption{}
	if db != "" {
		opts = append(opts, neo4j.ExecuteQueryWithDatabase(db))
	}
	if access == Read {
		opts = append(opts, neo4j.ExecuteQueryWithReadersRouting())
	} else {
		opts = append(opts, neo4j.ExecuteQueryWithWritersRouting())
	}
	// mode is parked for now
	eager, err := neo4j.ExecuteQuery(ctx, b.driver, stmt, params, neo4j.EagerResultTransformer, opts...)
	if err != nil {
		return nil, err
	}
	summary := mapSummary(eager.Summary)
	res := &Result{
		Keys:    eager.Keys,
		Summary: summary,
	}
	for _, r := range eager.Records {
		values := make([]any, len(eager.Keys))
		for i, k := range eager.Keys {
			v, _ := r.Get(k)
			values[i] = mapValue(v)
		}
		res.Records = append(res.Records, Record{
			keys:   eager.Keys,
			values: values,
		})
	}
	return res, nil
}

func (b *boltBackend) executeStream(ctx context.Context, stmt string, params map[string]any, mode TransactionMode, access AccessMode, database string) (*StreamResult, error) {
	// Minimal fallback: reuse buffered execution for now. True streaming can be added later.
	db := b.database
	if database != "" {
		db = database
	}
	opts := []neo4j.ExecuteQueryConfigurationOption{}
	if db != "" {
		opts = append(opts, neo4j.ExecuteQueryWithDatabase(db))
	}
	if access == Read {
		opts = append(opts, neo4j.ExecuteQueryWithReadersRouting())
	} else {
		opts = append(opts, neo4j.ExecuteQueryWithWritersRouting())
	}
	eager, err := neo4j.ExecuteQuery(ctx, b.driver, stmt, params, neo4j.EagerResultTransformer, opts...)
	if err != nil {
		return nil, err
	}
	summary := mapSummary(eager.Summary)
	records := []Record{}
	keys := eager.Keys
	for _, r := range eager.Records {
		values := make([]any, len(keys))
		for i, k := range keys {
			v, _ := r.Get(k)
			values[i] = mapValue(v)
		}
		records = append(records, Record{keys: keys, values: values})
	}
	return &StreamResult{
		keys:    keys,
		records: records,
		summary: summary,
		closeFn: func() error { return nil },
	}, nil
}

func (b *boltBackend) close(ctx context.Context) error {
	return b.driver.Close(ctx)
}

func mapSummary(s neo4j.ResultSummary) Summary {
	if s == nil {
		return Summary{}
	}
	qt := s.QueryType()
	var qtype QueryType
	switch qt.String() {
	case "READ_ONLY":
		qtype = QueryTypeRead
	case "READ_WRITE":
		qtype = QueryTypeReadWrite
	case "WRITE_ONLY":
		qtype = QueryTypeWrite
	case "SCHEMA_WRITE":
		qtype = QueryTypeSchema
	default:
		qtype = QueryTypeUnknown
	}
	c := s.Counters()
	counters := Counters{
		NodesCreated:         c.NodesCreated(),
		NodesDeleted:         c.NodesDeleted(),
		RelationshipsCreated: c.RelationshipsCreated(),
		RelationshipsDeleted: c.RelationshipsDeleted(),
		PropertiesSet:        c.PropertiesSet(),
	}
	return Summary{
		QueryType:            qtype,
		Counters:             counters,
		ResultAvailableAfter: s.ResultAvailableAfter(),
		ResultConsumedAfter:  s.ResultConsumedAfter(),
	}
}

func mapValue(v any) any {
	switch x := v.(type) {
	case neo4j.Node:
		return Node{
			ElementID:  x.ElementId,
			Labels:     x.Labels,
			Properties: x.Props,
		}
	case neo4j.Relationship:
		return Relationship{
			ElementID:      x.ElementId,
			Type:           x.Type,
			StartElementID: x.StartElementId,
			EndElementID:   x.EndElementId,
			Properties:     x.Props,
		}
	case neo4j.Path:
		nodes := make([]Node, len(x.Nodes))
		for i, n := range x.Nodes {
			nodes[i] = Node{
				ElementID:  n.ElementId,
				Labels:     n.Labels,
				Properties: n.Props,
			}
		}
		rels := make([]Relationship, len(x.Relationships))
		for i, r := range x.Relationships {
			rels[i] = Relationship{
				ElementID:      r.ElementId,
				Type:           r.Type,
				StartElementID: r.StartElementId,
				EndElementID:   r.EndElementId,
				Properties:     r.Props,
			}
		}
		return Path{
			Nodes:         nodes,
			Relationships: rels,
		}
	case neo4j.Point2D:
		return Point{
			SRID: int(x.SpatialRefId),
			X:    x.X,
			Y:    x.Y,
			Z:    nil,
		}
	case neo4j.Point3D:
		z := x.Z
		return Point{
			SRID: int(x.SpatialRefId),
			X:    x.X,
			Y:    x.Y,
			Z:    &z,
		}
	case neo4j.Duration:
		return Duration{
			Months:  x.Months,
			Days:    x.Days,
			Seconds: x.Seconds,
			Nanos:   x.Nanos,
		}
	case neo4j.Date:
		t := x.Time()
		return Date{
			Year:  t.Year(),
			Month: int(t.Month()),
			Day:   t.Day(),
		}
	case neo4j.LocalTime:
		t := x.Time()
		return LocalTime{
			Hour:   t.Hour(),
			Minute: t.Minute(),
			Second: t.Second(),
			Nano:   t.Nanosecond(),
		}
	case neo4j.Time:
		t := x.Time()
		lt := LocalTime{
			Hour:   t.Hour(),
			Minute: t.Minute(),
			Second: t.Second(),
			Nano:   t.Nanosecond(),
		}
		return Time{
			LocalTime: lt,
			Offset:    t.Format(time.RFC3339),
		}
	case neo4j.LocalDateTime:
		t := x.Time()
		return LocalDateTime{
			Date: Date{
				Year:  t.Year(),
				Month: int(t.Month()),
				Day:   t.Day(),
			},
			LocalTime: LocalTime{
				Hour:   t.Hour(),
				Minute: t.Minute(),
				Second: t.Second(),
				Nano:   t.Nanosecond(),
			},
		}
	case []any:
		out := make([]any, len(x))
		for i, v2 := range x {
			out[i] = mapValue(v2)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v2 := range x {
			out[k] = mapValue(v2)
		}
		return out
	default:
		return v
	}
}
