// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

package logger

import (
	"io"
	"log/slog"
	"maps"
	"strings"
)

// Service holds the logger and its level controller.
type Service struct {
	*slog.Logger
	level *slog.LevelVar
}

var defaultService *Service

const (
	LevelDebug     = slog.LevelDebug
	LevelInfo      = slog.LevelInfo
	LevelNotice    = slog.Level(2)
	LevelWarning   = slog.LevelWarn
	LevelError     = slog.LevelError
	LevelCritical  = slog.Level(10)
	LevelAlert     = slog.Level(12)
	LevelEmergency = slog.Level(16)
)

var ValidLogLevels = []string{
	"debug", "info", "notice", "warning", "error", "critical", "alert", "emergency",
}
var ValidLogFormats = []string{"text", "json"}

// options holds the values Option functions configure before New builds a Service.
type options struct {
	extraSensitiveKeys []string
}

// Option configures a Service constructed by New.
type Option func(*options)

// WithSensitiveKeys adds extra attribute keys (case-insensitive) that this
// Service redacts in its output, on top of the built-in defaults (password,
// token, secret, etc — see IsSensitiveKey). Each Service gets its own set:
// keys added here never affect any other Service instance.
func WithSensitiveKeys(keys ...string) Option {
	return func(o *options) { o.extraSensitiveKeys = append(o.extraSensitiveKeys, keys...) }
}

func (s *Service) SetLevel(level string) {
	s.level.Set(parseLevel(level))
}

// Init builds a default Service and installs it as both the package-level
// default (used by the free SetLevel function) and slog's global default logger.
func Init(level, format string, writer io.Writer, opts ...Option) {
	defaultService = New(level, format, writer, opts...)
	slog.SetDefault(defaultService.Logger)
}

// SetLevel changes the level of the Service installed by Init, if any.
func SetLevel(level string) {
	if defaultService != nil {
		defaultService.SetLevel(level)
	}
}

func New(level, format string, writer io.Writer, opts ...Option) *Service {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	sensitive := sensitiveKeySet(o.extraSensitiveKeys)

	levelVar := &slog.LevelVar{}
	levelVar.Set(parseLevel(level))
	handlerOpts := &slog.HandlerOptions{Level: levelVar, ReplaceAttr: newReplaceAttr(sensitive)}

	var handler slog.Handler
	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(writer, handlerOpts)
	default:
		handler = slog.NewTextHandler(writer, handlerOpts)
	}
	return &Service{Logger: slog.New(handler), level: levelVar}
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "notice":
		return LevelNotice
	case "warning":
		return LevelWarning
	case "error":
		return LevelError
	case "critical":
		return LevelCritical
	case "alert":
		return LevelAlert
	case "emergency":
		return LevelEmergency
	default:
		return LevelInfo
	}
}

var sensitiveKeys = map[string]bool{
	"password": true, "token": true, "secret": true, "api_key": true, "auth_token": true,
	"uri": true, "address": true, "host": true, "port": true, "bolt_uri": true,
}

// IsSensitiveKey reports whether key matches one of the package's built-in
// sensitive-key defaults. It does not know about any per-Service keys added
// via WithSensitiveKeys — those only affect that Service's own output.
func IsSensitiveKey(key string) bool {
	_, exists := sensitiveKeys[strings.ToLower(key)]
	return exists
}

// sensitiveKeySet returns the built-in sensitive keys unioned with extra,
// without mutating the shared package-level default map.
func sensitiveKeySet(extra []string) map[string]bool {
	if len(extra) == 0 {
		return sensitiveKeys
	}
	set := maps.Clone(sensitiveKeys)
	for _, k := range extra {
		set[strings.ToLower(k)] = true
	}
	return set
}

// newReplaceAttr builds a slog.HandlerOptions.ReplaceAttr bound to sensitive,
// so each Service can redact its own set of keys independently of any other.
func newReplaceAttr(sensitive map[string]bool) func([]string, slog.Attr) slog.Attr {
	return func(_ []string, a slog.Attr) slog.Attr {
		if a.Key == slog.LevelKey {
			level := a.Value.Any().(slog.Level)
			switch {
			case level < LevelInfo:
				a.Value = slog.StringValue("DEBUG")
			case level < LevelNotice:
				a.Value = slog.StringValue("INFO")
			case level < LevelWarning:
				a.Value = slog.StringValue("NOTICE")
			case level < LevelError:
				a.Value = slog.StringValue("WARNING")
			case level < LevelCritical:
				a.Value = slog.StringValue("ERROR")
			case level < LevelAlert:
				a.Value = slog.StringValue("CRITICAL")
			case level < LevelEmergency:
				a.Value = slog.StringValue("ALERT")
			default:
				a.Value = slog.StringValue("EMERGENCY")
			}
		}
		if sensitive[strings.ToLower(a.Key)] {
			a.Value = slog.StringValue("[REDACTED]")
		}
		return a
	}
}
