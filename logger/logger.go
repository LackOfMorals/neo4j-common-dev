package logger

import (
	"io"
	"log/slog"
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

func (s *Service) SetLevel(level string) {
	s.level.Set(parseLevel(level))
}

func Init(level, format string, writer io.Writer) {
	defaultService = New(level, format, writer)
	slog.SetDefault(defaultService.Logger)
}

func SetLevel(level string) {
	if defaultService != nil {
		defaultService.SetLevel(level)
	}
}

func New(level, format string, writer io.Writer) *Service {
	levelVar := &slog.LevelVar{}
	levelVar.Set(parseLevel(level))
	opts := &slog.HandlerOptions{Level: levelVar, ReplaceAttr: replaceAttr}
	var handler slog.Handler
	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(writer, opts)
	default:
		handler = slog.NewTextHandler(writer, opts)
	}
	return &Service{Logger: slog.New(handler), level: levelVar}
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug": return LevelDebug
	case "info": return LevelInfo
	case "notice": return LevelNotice
	case "warning": return LevelWarning
	case "error": return LevelError
	case "critical": return LevelCritical
	case "alert": return LevelAlert
	case "emergency": return LevelEmergency
	default: return LevelInfo
	}
}

var sensitiveKeys = map[string]bool{
	"password": true, "token": true, "secret": true, "api_key": true, "auth_token": true,
	"uri": true, "address": true, "host": true, "port": true, "bolt_uri": true,
}

func IsSensitiveKey(key string) bool {
	_, exists := sensitiveKeys[strings.ToLower(key)]
	return exists
}

func replaceAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.LevelKey {
		level := a.Value.Any().(slog.Level)
		switch {
		case level < LevelInfo: a.Value = slog.StringValue("DEBUG")
		case level < LevelNotice: a.Value = slog.StringValue("INFO")
		case level < LevelWarning: a.Value = slog.StringValue("NOTICE")
		case level < LevelError: a.Value = slog.StringValue("WARNING")
		case level < LevelCritical: a.Value = slog.StringValue("ERROR")
		case level < LevelAlert: a.Value = slog.StringValue("CRITICAL")
		case level < LevelEmergency: a.Value = slog.StringValue("ALERT")
		default: a.Value = slog.StringValue("EMERGENCY")
		}
	}
	if IsSensitiveKey(a.Key) {
		a.Value = slog.StringValue("[REDACTED]")
	}
	return a
}
