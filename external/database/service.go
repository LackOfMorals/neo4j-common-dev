package database

import (
	"context"
	"log/slog"
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
	return nil, nil
}

func WithMaxResultBytes(bytes int64) Option {
	return func(o *options) { o.maxResultBytes = bytes }
}

func (s *Service) Close(ctx context.Context) error {
	return nil
}

// Temporal types – added alongside Duration/Point as small package types
type Date struct{ Year, Month, Day int }
type LocalTime struct{ Hour, Minute, Second, Nano int }
type Time struct{ LocalTime LocalTime; Offset string }
type LocalDateTime struct{ Date Date; LocalTime LocalTime }
type DateTime struct{ LocalDateTime LocalDateTime; Offset string; Zone string }

// Streaming line size default
const defaultMaxStreamLineSize = 10 * 1024 * 1024
