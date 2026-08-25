// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/LackOfMorals/neo4jPackages/external/logger"
)

func decodeJSONLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("error unmarshalling log line %q: %v", buf.String(), err)
	}
	return m
}

func TestNewHandlerSelection(t *testing.T) {
	t.Run("json format uses the JSON handler", func(t *testing.T) {
		var buf bytes.Buffer
		svc := logger.New("info", "json", &buf)
		svc.Info("hello")
		m := decodeJSONLine(t, &buf)
		if m["msg"] != "hello" {
			t.Errorf("unexpected msg: got %v, want hello", m["msg"])
		}
	})

	t.Run("json format is case-insensitive", func(t *testing.T) {
		var buf bytes.Buffer
		svc := logger.New("info", "JSON", &buf)
		svc.Info("hello")
		decodeJSONLine(t, &buf) // fails the test if this isn't valid JSON
	})

	t.Run("unknown format falls back to the text handler", func(t *testing.T) {
		var buf bytes.Buffer
		svc := logger.New("info", "nonsense", &buf)
		svc.Info("hello")
		if strings.HasPrefix(strings.TrimSpace(buf.String()), "{") {
			t.Errorf("expected text output, got JSON-looking line: %s", buf.String())
		}
		if !strings.Contains(buf.String(), "hello") {
			t.Errorf("expected output to contain the message, got: %s", buf.String())
		}
	})
}

func TestLevelFiltering(t *testing.T) {
	t.Run("messages below the configured level are suppressed", func(t *testing.T) {
		var buf bytes.Buffer
		svc := logger.New("warning", "text", &buf)
		svc.Info("should not appear")
		if buf.Len() != 0 {
			t.Errorf("expected no output below the configured level, got: %s", buf.String())
		}
	})

	t.Run("messages at or above the configured level are emitted", func(t *testing.T) {
		var buf bytes.Buffer
		svc := logger.New("warning", "text", &buf)
		svc.Warn("should appear")
		if !strings.Contains(buf.String(), "should appear") {
			t.Errorf("expected output to contain the message, got: %s", buf.String())
		}
	})

	t.Run("unknown level string defaults to info", func(t *testing.T) {
		var buf bytes.Buffer
		svc := logger.New("nonsense", "text", &buf)
		svc.Debug("should not appear")
		svc.Info("should appear")
		if strings.Contains(buf.String(), "should not appear") {
			t.Errorf("expected debug output to be suppressed, got: %s", buf.String())
		}
		if !strings.Contains(buf.String(), "should appear") {
			t.Errorf("expected info output to be present, got: %s", buf.String())
		}
	})

	t.Run("SetLevel changes the threshold at runtime", func(t *testing.T) {
		var buf bytes.Buffer
		svc := logger.New("error", "text", &buf)
		svc.Warn("suppressed before SetLevel")
		svc.SetLevel("warning")
		svc.Warn("visible after SetLevel")

		if strings.Contains(buf.String(), "suppressed before SetLevel") {
			t.Errorf("expected first message to be suppressed, got: %s", buf.String())
		}
		if !strings.Contains(buf.String(), "visible after SetLevel") {
			t.Errorf("expected second message to be present, got: %s", buf.String())
		}
	})
}

func TestLevelLabels(t *testing.T) {
	testCases := []struct {
		level string
		want  string
	}{
		{"debug", "DEBUG"},
		{"info", "INFO"},
		{"notice", "NOTICE"},
		{"warning", "WARNING"},
		{"error", "ERROR"},
		{"critical", "CRITICAL"},
		{"alert", "ALERT"},
		{"emergency", "EMERGENCY"},
	}

	for _, tc := range testCases {
		t.Run(tc.level, func(t *testing.T) {
			var buf bytes.Buffer
			svc := logger.New(tc.level, "json", &buf)
			svc.Log(context.Background(), mustLevel(tc.level), "msg")
			m := decodeJSONLine(t, &buf)
			if m["level"] != tc.want {
				t.Errorf("unexpected level label for %q: got %v, want %v", tc.level, m["level"], tc.want)
			}
		})
	}
}

func mustLevel(name string) slog.Level {
	switch strings.ToLower(name) {
	case "debug":
		return logger.LevelDebug
	case "info":
		return logger.LevelInfo
	case "notice":
		return logger.LevelNotice
	case "warning":
		return logger.LevelWarning
	case "error":
		return logger.LevelError
	case "critical":
		return logger.LevelCritical
	case "alert":
		return logger.LevelAlert
	case "emergency":
		return logger.LevelEmergency
	}
	return logger.LevelInfo
}

func TestSensitiveKeyRedaction(t *testing.T) {
	t.Run("IsSensitiveKey matches the built-in defaults, case-insensitively", func(t *testing.T) {
		if !logger.IsSensitiveKey("Password") {
			t.Errorf("expected 'Password' to be recognized as sensitive")
		}
		if logger.IsSensitiveKey("username") {
			t.Errorf("expected 'username' not to be recognized as sensitive")
		}
	})

	t.Run("default Service redacts built-in sensitive keys", func(t *testing.T) {
		var buf bytes.Buffer
		svc := logger.New("info", "json", &buf)
		svc.Info("connecting", "password", "super-secret")
		m := decodeJSONLine(t, &buf)
		if m["password"] != "[REDACTED]" {
			t.Errorf("expected password to be redacted, got %v", m["password"])
		}
	})

	t.Run("WithSensitiveKeys redacts an app-specific key on that Service only", func(t *testing.T) {
		var bufWith, bufWithout bytes.Buffer
		withExtra := logger.New("info", "json", &bufWith, logger.WithSensitiveKeys("license_key"))
		withoutExtra := logger.New("info", "json", &bufWithout)

		withExtra.Info("licensing", "license_key", "abc123")
		withoutExtra.Info("licensing", "license_key", "abc123")

		got := decodeJSONLine(t, &bufWith)
		if got["license_key"] != "[REDACTED]" {
			t.Errorf("expected license_key to be redacted on the configured Service, got %v", got["license_key"])
		}

		notGot := decodeJSONLine(t, &bufWithout)
		if notGot["license_key"] == "[REDACTED]" {
			t.Errorf("expected license_key to be untouched on a Service without WithSensitiveKeys, got %v", notGot["license_key"])
		}
	})
}

func TestInitAndGlobalSetLevel(t *testing.T) {
	prevDefault := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prevDefault) })

	var buf bytes.Buffer
	logger.Init("warning", "text", &buf)

	slog.Info("suppressed by Init's level")
	logger.SetLevel("info")
	slog.Info("visible after SetLevel")

	if strings.Contains(buf.String(), "suppressed by Init's level") {
		t.Errorf("expected first message to be suppressed, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "visible after SetLevel") {
		t.Errorf("expected second message to be present, got: %s", buf.String())
	}
}
