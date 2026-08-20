package analytics

import "log/slog"

type Tracker interface {
	Track(event string, properties map[string]any)
}

type NoopTracker struct{}

func (n NoopTracker) Track(event string, properties map[string]any) {}

var DefaultTracker Tracker = NoopTracker{}

func Track(event string, properties map[string]any) {
	DefaultTracker.Track(event, properties)
	slog.Info("analytics event", "event", event)
}
