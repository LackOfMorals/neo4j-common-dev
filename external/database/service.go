package database

import (
	"context"
	"log/slog"
	"net/url"
	"time"
)

type Service struct {
	backend backend
	logger  *slog.Logger
}

type Option func(*options)

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

func (s *Service) Close(ctx context.Context) error {
	if s.backend != nil {
		return s.backend.close(ctx)
	}
	return nil
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
