package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"time"
)

// databaseNamePattern is the set of characters a database name may contain.
// Names are interpolated into Query API request paths and Bolt session
// configuration, so anything outside this set (e.g. "/" or "..") is
// rejected rather than allowed to alter the request.
var databaseNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// validateDatabaseName reports whether name is a legal database name for
// this Service. The empty string is valid and means "the server's
// default/home database".
func validateDatabaseName(name string) error {
	if name == "" {
		return nil
	}
	if !databaseNamePattern.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidDatabase, name)
	}
	return nil
}

type Service struct {
	backend backend
	logger  *slog.Logger
}

type Option func(*options)
type QueryOption func(*queryOptions)

type queryOptions struct {
	mode     TransactionMode
	access   AccessMode
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
	authKind           authKind
	authOptions        int // counts WithBasicAuth/WithBearerToken applications
	username, password string
	token              string
	logger             *slog.Logger
	timeout            time.Duration
	database           string
	maxResultBytes     int64
}

// New creates a Service from uri, selecting the backend by scheme:
// neo4j(+s|+ssc):// and bolt(+s|+ssc):// use the Bolt driver; http(s)://
// use the Query API. New does no network I/O — it never dials the server or
// verifies credentials; the Query API's minimum-version check is deferred to
// the first Execute/ExecuteStream call.
//
// Note that uri may embed credentials (user:password@host); treat it as
// sensitive and do not log it. Conflicting auth options and invalid database
// names are constructor-time errors (ErrConflictingAuth, ErrInvalidDatabase);
// an unrecognized scheme returns ErrUnsupportedScheme.
func New(uri string, opts ...Option) (*Service, error) {
	o := &options{
		timeout: 30 * time.Second,
	}
	// Parse URI for embedded auth if no explicit auth option is given.
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
	if o.authOptions > 1 {
		return nil, ErrConflictingAuth
	}
	if err := validateDatabaseName(o.database); err != nil {
		return nil, err
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

// WithMaxResultBytes overrides the response-size cap for buffered Query API
// responses (a no-op for the Bolt backend). Use ExecuteStream for results
// that may exceed the cap.
func WithMaxResultBytes(bytes int64) Option {
	return func(o *options) { o.maxResultBytes = bytes }
}

// WithBasicAuth authenticates with username/password. Mutually exclusive
// with WithBearerToken — New returns ErrConflictingAuth if both are given.
func WithBasicAuth(username, password string) Option {
	return func(o *options) {
		o.authKind = authBasic
		o.authOptions++
		o.username = username
		o.password = password
	}
}

// WithBearerToken authenticates with a bearer token (e.g. an SSO-issued
// access token). Mutually exclusive with WithBasicAuth — New returns
// ErrConflictingAuth if both are given.
func WithBearerToken(token string) Option {
	return func(o *options) {
		o.authKind = authBearer
		o.authOptions++
		o.token = token
	}
}

// WithLogger sets the *slog.Logger the backends use for their own
// diagnostics (version checks, retries, explicit-mode warnings, commit and
// rollback outcomes). Any *slog.Logger works here, including one obtained
// via logger.New(...).Logger — this package depends only on log/slog.
// Defaults to a logger that discards everything.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.logger = l }
}

// WithTimeout sets the timeout used to establish connectivity: the Bolt
// driver's socket-connect timeout, or the Query API backend's underlying
// HTTP client timeout. It does not replace per-call ctx deadlines — every
// Execute/ExecuteStream call is still bounded by its own ctx as well. A
// non-positive value keeps the 30s default.
func WithTimeout(d time.Duration) Option {
	return func(o *options) {
		if d > 0 {
			o.timeout = d
		}
	}
}

// WithDatabase selects the database this Service targets. Defaults to ""
// (the server's default/home database). Per-call overrides are possible
// with WithDatabaseOverride. The name must match [A-Za-z0-9_-]+ or be
// empty; otherwise New returns ErrInvalidDatabase.
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
	if err := validateDatabaseName(o.database); err != nil {
		return nil, err
	}
	return s.backend.executeBuffered(ctx, cypher, params, o.mode, o.access, o.database)
}

func (s *Service) ExecuteStream(ctx context.Context, cypher string, params map[string]any, opts ...QueryOption) (*StreamResult, error) {
	o := &queryOptions{}
	for _, opt := range opts {
		opt(o)
	}
	if err := validateDatabaseName(o.database); err != nil {
		return nil, err
	}
	return s.backend.executeStream(ctx, cypher, params, o.mode, o.access, o.database)
}

// BeginTx starts a multi-statement explicit transaction. Currently only the
// Query API backend supports transactions; on the Bolt backend this returns
// a "transactions not supported" error. The transaction's access mode and
// per-call database override are taken from the given QueryOptions
// (WithAccessMode, WithDatabaseOverride); WithTransactionMode is accepted
// but ignored, since a BeginTx handle is by definition explicit.
func (s *Service) BeginTx(ctx context.Context, opts ...QueryOption) (*Tx, error) {
	o := &queryOptions{}
	for _, opt := range opts {
		opt(o)
	}
	if err := validateDatabaseName(o.database); err != nil {
		return nil, err
	}
	if tb, ok := s.backend.(interface {
		beginTx(ctx context.Context, access AccessMode, database string) (*Tx, error)
	}); ok {
		return tb.beginTx(ctx, o.access, o.database)
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
type Time struct {
	LocalTime LocalTime
	Offset    string
}
type LocalDateTime struct {
	Date      Date
	LocalTime LocalTime
}
type DateTime struct {
	LocalDateTime LocalDateTime
	Offset        string
	Zone          string
}

// Streaming line size default
const defaultMaxStreamLineSize = 10 * 1024 * 1024

var _ = errors.Is // keep errors import (used by callers via sentinels)
