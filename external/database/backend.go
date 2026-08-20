package database

import "context"

type backend interface {
	executeBuffered(ctx context.Context, stmt string, params map[string]any, mode TransactionMode, access AccessMode) (*Result, error)
	executeStream(ctx context.Context, stmt string, params map[string]any, mode TransactionMode, access AccessMode) (*StreamResult, error)
	close(ctx context.Context) error
}

type authKind int

const (
	authNone authKind = iota
	authBasic
	authBearer
)
