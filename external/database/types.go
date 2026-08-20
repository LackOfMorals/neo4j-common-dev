package database

import (
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
	QueryTypeRead QueryType = "r"
	QueryTypeWrite QueryType = "w"
	QueryTypeReadWrite QueryType = "rw"
	QueryTypeSchema QueryType = "s"
)

type Counters struct {
	NodesCreated, NodesDeleted int
	RelationshipsCreated, RelationshipsDeleted int
	PropertiesSet int
}

type Summary struct {
	QueryType QueryType
	Counters Counters
	ResultAvailableAfter time.Duration
	ResultConsumedAfter time.Duration
	Database string
}

type Node struct {
	ElementID string
	Labels []string
	Properties map[string]any
}

type Relationship struct {
	ElementID string
	Type string
	StartElementID string
	EndElementID string
	Properties map[string]any
}

type Path struct {
	Nodes []Node
	Relationships []Relationship
}

type Point struct {
	SRID int
	X, Y float64
	Z *float64
}

type Duration struct {
	Months, Days, Seconds int64
	Nanos int
}

type Record struct {
	keys []string
	values []any
}

type Result struct {
	Keys []string
	Records []Record
	Summary Summary
}

type StreamResult struct {
	keys []string
	// implementation details hidden
}

func (r *StreamResult) Keys() []string { return r.keys }
