package database

import (
	"context"
	"log/slog"
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
}

func New(uri string, opts ...Option) (*Service, error) {
	return nil, nil
}

func (s *Service) Close(ctx context.Context) error {
	return nil
}
