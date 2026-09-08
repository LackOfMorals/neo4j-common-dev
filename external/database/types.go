package database

import (
	"context"
	"time"
)

type TransactionMode int

const (
	Implicit TransactionMode = iota
	Explicit
)

type AccessMode int

const (
	Write AccessMode = iota
	Read
)

type QueryType string

const (
	QueryTypeUnknown   QueryType = ""
	QueryTypeRead      QueryType = "r"
	QueryTypeWrite     QueryType = "w"
	QueryTypeReadWrite QueryType = "rw"
	QueryTypeSchema    QueryType = "s"
)

// Counters reports how many graph elements a write statement touched.
// Every count field is zero for a read-only statement.
type Counters struct {
	NodesCreated, NodesDeleted                 int
	RelationshipsCreated, RelationshipsDeleted int
	PropertiesSet                              int
	LabelsAdded, LabelsRemoved                 int
	IndexesAdded, IndexesRemoved               int
	ConstraintsAdded, ConstraintsRemoved       int
	ContainsUpdates, ContainsSystemUpdates     bool
}

// NotificationPosition locates a Notification within the submitted query
// text. nil on the owning Notification when it isn't tied to a position.
type NotificationPosition struct {
	Offset int
	Line   int
	Column int
}

// Notification is a server-emitted diagnostic about a statement (unused
// variable, deprecated function, missing index, ...), independent of whether
// the statement also produced an error.
type Notification struct {
	Code        string
	Title       string
	Description string
	Severity    string
	Category    string
	Position    *NotificationPosition
}

// Summary carries execution metadata common to both backends.
type Summary struct {
	QueryType            QueryType
	Counters             Counters
	ResultAvailableAfter time.Duration
	ResultConsumedAfter  time.Duration
	Notifications        []Notification
	Database             string
	Bookmarks            []string
}

type Node struct {
	ElementID  string
	Labels     []string
	Properties map[string]any
}

type Relationship struct {
	ElementID      string
	Type           string
	StartElementID string
	EndElementID   string
	Properties     map[string]any
}

type Path struct {
	Nodes         []Node
	Relationships []Relationship
}

type Point struct {
	SRID int
	X, Y float64
	Z    *float64
}

type Duration struct {
	Months, Days, Seconds int64
	Nanos                 int
}

type Vector struct {
	Values []float64
}

type Record struct {
	keys   []string
	values []any
}

func (r Record) Keys() []string { return r.keys }
func (r Record) Values() []any  { return r.values }
func (r Record) Get(key string) (any, bool) {
	for i, k := range r.keys {
		if k == key {
			return r.values[i], true
		}
	}
	return nil, false
}
func (r Record) At(i int) (any, bool) {
	if i < 0 || i >= len(r.values) {
		return nil, false
	}
	return r.values[i], true
}
func (r Record) GetNode(key string) (Node, bool) {
	v, ok := r.Get(key)
	if !ok {
		return Node{}, false
	}
	n, ok := v.(Node)
	return n, ok
}
func (r Record) GetRelationship(key string) (Relationship, bool) {
	v, ok := r.Get(key)
	if !ok {
		return Relationship{}, false
	}
	rr, ok := v.(Relationship)
	return rr, ok
}

type Result struct {
	Keys    []string
	Records []Record
	Summary Summary
}

type StreamResult struct {
	keys    []string
	records []Record
	summary Summary
	closeFn func() error
}

func (r *StreamResult) Keys() []string { return r.keys }

func (r *StreamResult) Records() func(yield func(Record, error) bool) {
	return func(yield func(Record, error) bool) {
		for _, rec := range r.records {
			if !yield(rec, nil) {
				return
			}
		}
	}
}

func (r *StreamResult) Summary() Summary { return r.summary }

func (r *StreamResult) Close() error {
	if r.closeFn != nil {
		return r.closeFn()
	}
	return nil
}

type Tx struct {
	id      string
	backend interface {
		txRun(ctx context.Context, stmt string, params map[string]any) (*Result, error)
		txCommit(ctx context.Context) (*CommitResult, error)
		txRollback(ctx context.Context) error
	}
}

type CommitResult struct {
	Bookmarks []string
	Summary   Summary
}

func (t *Tx) Run(ctx context.Context, cypher string, params map[string]any) (*Result, error) {
	return t.backend.txRun(ctx, cypher, params)
}

func (t *Tx) Commit(ctx context.Context) (*CommitResult, error) {
	return t.backend.txCommit(ctx)
}

func (t *Tx) Rollback(ctx context.Context) error {
	return t.backend.txRollback(ctx)
}
