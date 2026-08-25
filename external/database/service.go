package database

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"time"
)

type Service struct {
	backend backend
	logger  *slog.Logger
}

type Option func(*options)
type QueryOption func(*queryOptions)

type queryOptions struct {
	mode   TransactionMode
	access AccessMode
	database string
}

func WithTransactionMode(mode TransactionMode) QueryOption {
	return func(o *queryOptions) { o.mode = mode }
}

func WithAccessMode(access AccessMode) QueryOption {
	return func(o *queryOptions) { o.access = access }
}

func WithDatabaseOverride(name string) QueryOption {
	return func(o *queryOptions) { o.database = name }
}

type options struct {
	authKind authKind
	username, password string
	token string
	logger *slog.Logger
	timeout time.Duration
	database string
	maxResultBytes int64
}

func New(uri string, opts ...Option) (*Service, error) {
	o := &options{
		timeout: 30 * time.Second,
	}
	// Parse URI for embedded auth/database if not overridden
	if u, err := url.Parse(uri); err == nil {
		if u.User != nil {
			user := u.User.Username()
			pass, _ := u.User.Password()
			if o.authKind == authNone {
				o.authKind = authBasic
				o.username = user
				o.password = pass
			}
		}
	}
	for _, opt := range opts {
		opt(o)
	}
	scheme, err := parseURIScheme(uri)
	if err != nil {
		return nil, err
	}
	b, err := selectBackend(scheme, uri, *o)
	if err != nil {
		return nil, err
	}
	return &Service{backend: b, logger: o.logger}, nil
}

func WithMaxResultBytes(bytes int64) Option {
	return func(o *options) { o.maxResultBytes = bytes }
}

func WithBasicAuth(username, password string) Option {
	return func(o *options) {
		o.authKind = authBasic
		o.username = username
		o.password = password
	}
}

func WithBearerToken(token string) Option {
	return func(o *options) {
		o.authKind = authBearer
		o.token = token
	}
}

func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.logger = l }
}

func WithTimeout(d time.Duration) Option {
	return func(o *options) { if d > 0 { o.timeout = d } }
}

func WithDatabase(name string) Option {
	return func(o *options) { o.database = name }
}

func (s *Service) Close(ctx context.Context) error {
	if s.backend != nil {
		return s.backend.close(ctx)
	}
	return nil
}

func (s *Service) Execute(ctx context.Context, cypher string, params map[string]any, opts ...QueryOption) (*Result, error) {
	o := &queryOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return s.backend.executeBuffered(ctx, cypher, params, o.mode, o.access, o.database)
}

func (s *Service) ExecuteStream(ctx context.Context, cypher string, params map[string]any, opts ...QueryOption) (*StreamResult, error) {
	o := &queryOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return s.backend.executeStream(ctx, cypher, params, o.mode, o.access, o.database)
}

func (s *Service) BeginTx(ctx context.Context) (*Tx, error) {
	if tb, ok := s.backend.(interface{ beginTx(ctx context.Context) (*Tx, error) }); ok {
		return tb.beginTx(ctx)
	}
	return nil, fmt.Errorf("transactions not supported for this backend")
}

func parseURIScheme(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" {
		return "", ErrUnsupportedScheme
	}
	return u.Scheme, nil
}

func selectBackend(scheme, uri string, o options) (backend, error) {
	switch scheme {
	case "neo4j", "neo4j+s", "neo4j+ssc", "bolt", "bolt+s", "bolt+ssc":
		return newBoltBackend(uri, o)
	case "http", "https":
		return newQueryAPIBackend(uri, o)
	default:
		return nil, ErrUnsupportedScheme
	}
}

var ErrUnsupportedScheme = &errUnsupportedScheme{}

type errUnsupportedScheme struct{}
func (e *errUnsupportedScheme) Error() string { return "unsupported scheme" }

// Temporal types – added alongside Duration/Point as small package types
type Date struct{ Year, Month, Day int }
type LocalTime struct{ Hour, Minute, Second, Nano int }
type Time struct{ LocalTime LocalTime; Offset string }
type LocalDateTime struct{ Date Date; LocalTime LocalTime }
type DateTime struct{ LocalDateTime LocalDateTime; Offset string; Zone string }

// Streaming line size default
const defaultMaxStreamLineSize = 10 * 1024 * 1024
