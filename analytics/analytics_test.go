// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

package analytics_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/LackOfMorals/neo4j-common/analytics"
)

// stubHTTPClient implements analytics.HTTPClient with a plain function,
// letting tests intercept outbound calls without a mocking framework.
type stubHTTPClient func(url, contentType string, body io.Reader) (*http.Response, error)

func (f stubHTTPClient) Post(url, contentType string, body io.Reader) (*http.Response, error) {
	return f(url, contentType, body)
}

func okResponse() *http.Response {
	// The request is sent with verbose=1, so the SDK decodes the body as
	// {"error":..., "status":...} — a bare "1" (a valid Mixpanel response in
	// non-verbose mode) fails that decode and logs a spurious error.
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"status":1}`))}
}

func decodeTrackedEvents(t *testing.T, body io.Reader) []analytics.TrackEvent {
	t.Helper()
	b, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("error reading body: %v", err)
	}
	var events []analytics.TrackEvent
	if err := json.Unmarshal(b, &events); err != nil {
		t.Fatalf("error unmarshalling body: %v", err)
	}
	return events
}

func newTestService(t *testing.T, client analytics.HTTPClient, opts ...analytics.Option) *analytics.Service {
	t.Helper()
	allOpts := append([]analytics.Option{analytics.WithHTTPClient(client)}, opts...)
	return analytics.New("test-token", "http://localhost", "analytics-test", "bolt://localhost:7687", allOpts...)
}

func TestEnableDisable(t *testing.T) {
	t.Run("enabled by default", func(t *testing.T) {
		svc := newTestService(t, nil)
		if !svc.IsEnabled() {
			t.Errorf("expected service to be enabled by default")
		}
	})

	t.Run("WithEnabled(false) disables at construction", func(t *testing.T) {
		svc := newTestService(t, nil, analytics.WithEnabled(false))
		if svc.IsEnabled() {
			t.Errorf("expected service to be disabled")
		}
	})

	t.Run("Disable/Enable toggle IsEnabled", func(t *testing.T) {
		svc := newTestService(t, nil)
		svc.Disable()
		if svc.IsEnabled() {
			t.Errorf("expected service to be disabled after Disable()")
		}
		svc.Enable()
		if !svc.IsEnabled() {
			t.Errorf("expected service to be enabled after Enable()")
		}
	})
}

func TestEmitEvent(t *testing.T) {
	t.Run("does not call HTTP client when disabled", func(t *testing.T) {
		client := stubHTTPClient(func(url, contentType string, body io.Reader) (*http.Response, error) {
			t.Fatalf("HTTP client should not be called while disabled")
			return nil, nil
		})
		svc := newTestService(t, client, analytics.WithEnabled(false))
		svc.EmitEvent(context.Background(), analytics.TrackEvent{Event: "test_event"})
	})

	t.Run("sends the given event as-is when enabled", func(t *testing.T) {
		var called bool
		client := stubHTTPClient(func(url, contentType string, body io.Reader) (*http.Response, error) {
			called = true
			events := decodeTrackedEvents(t, body)
			if len(events) != 1 {
				t.Fatalf("expected 1 event, got %d", len(events))
			}
			if events[0].Event != "specific_event" {
				t.Errorf("unexpected event name: got %s, want specific_event", events[0].Event)
			}
			props, ok := events[0].Properties.(map[string]any)
			if !ok {
				t.Fatalf("properties is not a map[string]any")
			}
			if props["key"] != "value" {
				t.Errorf("unexpected properties[key]: got %v, want value", props["key"])
			}
			return okResponse(), nil
		})
		svc := newTestService(t, client)
		svc.EmitEvent(context.Background(), analytics.TrackEvent{
			Event:      "specific_event",
			Properties: map[string]any{"key": "value"},
		})
		if !called {
			t.Errorf("expected HTTP client to be called")
		}
	})

	t.Run("constructs the correct URL regardless of endpoint trailing slashes", func(t *testing.T) {
		testCases := []struct {
			name             string
			mixpanelEndpoint string
			expectedURL      string
		}{
			{"no trailing slash", "http://localhost", "http://localhost/track?verbose=1"},
			{"one trailing slash", "http://localhost/", "http://localhost/track?verbose=1"},
			{"multiple trailing slashes", "http://localhost//", "http://localhost/track?verbose=1"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				var gotURL string
				client := stubHTTPClient(func(url, contentType string, body io.Reader) (*http.Response, error) {
					gotURL = url
					return okResponse(), nil
				})
				svc := analytics.New("test-token", tc.mixpanelEndpoint, "analytics-test", "bolt://localhost:7687",
					analytics.WithHTTPClient(client))
				svc.EmitEvent(context.Background(), analytics.TrackEvent{Event: "test_event"})
				if gotURL != tc.expectedURL {
					t.Errorf("unexpected URL: got %s, want %s", gotURL, tc.expectedURL)
				}
			})
		}
	})
}

func TestEmit(t *testing.T) {
	t.Run("merges base and specific properties when no common properties are registered", func(t *testing.T) {
		var props map[string]any
		client := stubHTTPClient(func(url, contentType string, body io.Reader) (*http.Response, error) {
			events := decodeTrackedEvents(t, body)
			props, _ = events[0].Properties.(map[string]any)
			return okResponse(), nil
		})
		svc := newTestService(t, client)
		svc.Emit(context.Background(), "APP_STARTED", map[string]any{"feature": "x"})

		if props["feature"] != "x" {
			t.Errorf("expected specific property to be present, got %v", props["feature"])
		}
		if props["token"] != "test-token" {
			t.Errorf("expected base property token to be present, got %v", props["token"])
		}
	})

	t.Run("merges base, common and specific properties, specific wins over common", func(t *testing.T) {
		var props map[string]any
		client := stubHTTPClient(func(url, contentType string, body io.Reader) (*http.Response, error) {
			events := decodeTrackedEvents(t, body)
			props, _ = events[0].Properties.(map[string]any)
			return okResponse(), nil
		})
		svc := newTestService(t, client,
			analytics.WithCommonProperties(map[string]any{"app_version": "1.0.0", "shared_key": "common"}))
		svc.Emit(context.Background(), "APP_STARTED", map[string]any{"feature": "x", "shared_key": "specific"})

		if props["app_version"] != "1.0.0" {
			t.Errorf("expected common property to be present, got %v", props["app_version"])
		}
		if props["feature"] != "x" {
			t.Errorf("expected specific property to be present, got %v", props["feature"])
		}
		if props["shared_key"] != "specific" {
			t.Errorf("expected specific property to win over common on collision, got %v", props["shared_key"])
		}
		if props["token"] != "test-token" {
			t.Errorf("expected base property token to be present, got %v", props["token"])
		}
	})

	t.Run("base properties win over common and specific on collision", func(t *testing.T) {
		var props map[string]any
		client := stubHTTPClient(func(url, contentType string, body io.Reader) (*http.Response, error) {
			events := decodeTrackedEvents(t, body)
			props, _ = events[0].Properties.(map[string]any)
			return okResponse(), nil
		})
		svc := newTestService(t, client,
			analytics.WithCommonProperties(map[string]any{"token": "not-the-real-token"}))
		svc.Emit(context.Background(), "APP_STARTED", map[string]any{"token": "also-not-the-real-token"})

		if props["token"] != "test-token" {
			t.Errorf("expected base token to win over common/specific, got %v", props["token"])
		}
	})

	t.Run("does not call HTTP client when disabled", func(t *testing.T) {
		client := stubHTTPClient(func(url, contentType string, body io.Reader) (*http.Response, error) {
			t.Fatalf("HTTP client should not be called while disabled")
			return nil, nil
		})
		svc := newTestService(t, client, analytics.WithEnabled(false))
		svc.Emit(context.Background(), "APP_STARTED", nil)
	})
}

func TestAuraDetection(t *testing.T) {
	testCases := []struct {
		name string
		uri  string
		want bool
	}{
		{"plain bolt URI", "bolt://localhost:7687", false},
		{"databases.neo4j.io", "neo4j+s://mydb.databases.neo4j.io", true},
		{"instances.neo4j.io", "neo4j+s://mydb.instances.neo4j.io", true},
		{"unrelated URI", "bolt://example.com:7687", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var props map[string]any
			client := stubHTTPClient(func(url, contentType string, body io.Reader) (*http.Response, error) {
				events := decodeTrackedEvents(t, body)
				props, _ = events[0].Properties.(map[string]any)
				return okResponse(), nil
			})
			svc := analytics.New("test-token", "http://localhost", "analytics-test", tc.uri,
				analytics.WithHTTPClient(client))
			svc.Emit(context.Background(), "APP_STARTED", nil)

			if props["isAura"] != tc.want {
				t.Errorf("unexpected isAura for %q: got %v, want %v", tc.uri, props["isAura"], tc.want)
			}
		})
	}
}

func TestEuResidency(t *testing.T) {
	t.Run("disabled by default: uses the endpoint passed to New", func(t *testing.T) {
		var gotURL string
		client := stubHTTPClient(func(url, contentType string, body io.Reader) (*http.Response, error) {
			gotURL = url
			return okResponse(), nil
		})
		svc := analytics.New("test-token", "http://localhost", "analytics-test", "bolt://localhost:7687",
			analytics.WithHTTPClient(client))
		svc.EmitEvent(context.Background(), analytics.TrackEvent{Event: "test_event"})

		if gotURL != "http://localhost/track?verbose=1" {
			t.Errorf("unexpected URL: got %s, want %s", gotURL, "http://localhost/track?verbose=1")
		}
	})

	t.Run("WithEuResidency(true) overrides the given endpoint with Mixpanel's EU endpoint", func(t *testing.T) {
		var gotURL string
		client := stubHTTPClient(func(url, contentType string, body io.Reader) (*http.Response, error) {
			gotURL = url
			return okResponse(), nil
		})
		svc := analytics.New("test-token", "http://localhost", "analytics-test", "bolt://localhost:7687",
			analytics.WithHTTPClient(client), analytics.WithEuResidency(true))
		svc.EmitEvent(context.Background(), analytics.TrackEvent{Event: "test_event"})

		if gotURL != "https://api-eu.mixpanel.com/track?verbose=1" {
			t.Errorf("unexpected URL: got %s, want %s", gotURL, "https://api-eu.mixpanel.com/track?verbose=1")
		}
	})
}

func TestGeoIPTracking(t *testing.T) {
	emit := func(t *testing.T, opts ...analytics.Option) map[string]any {
		t.Helper()
		var props map[string]any
		client := stubHTTPClient(func(url, contentType string, body io.Reader) (*http.Response, error) {
			events := decodeTrackedEvents(t, body)
			props, _ = events[0].Properties.(map[string]any)
			return okResponse(), nil
		})
		svc := newTestService(t, client, opts...)
		svc.Emit(context.Background(), "APP_STARTED", nil)
		return props
	}

	t.Run("enabled by default: no ip property is sent, letting Mixpanel geolocate from the request", func(t *testing.T) {
		props := emit(t)
		if !newTestService(t, nil).IsGeoIPTrackingEnabled() {
			t.Errorf("expected GeoIP tracking to be enabled by default")
		}
		if _, exists := props["ip"]; exists {
			t.Errorf("expected no ip property when enabled, got %v", props["ip"])
		}
	})

	t.Run("WithGeoIPTracking(false) sends ip=0 to opt every event out", func(t *testing.T) {
		props := emit(t, analytics.WithGeoIPTracking(false))
		if props["ip"] != "0" {
			t.Errorf("expected ip property to be \"0\", got %v", props["ip"])
		}
	})

	t.Run("DisableGeoIPTracking flips an existing Service at runtime", func(t *testing.T) {
		var props map[string]any
		client := stubHTTPClient(func(url, contentType string, body io.Reader) (*http.Response, error) {
			events := decodeTrackedEvents(t, body)
			props, _ = events[0].Properties.(map[string]any)
			return okResponse(), nil
		})
		svc := newTestService(t, client)

		svc.Emit(context.Background(), "BEFORE_DISABLE", nil)
		if _, exists := props["ip"]; exists {
			t.Errorf("expected no ip property before DisableGeoIPTracking, got %v", props["ip"])
		}

		svc.DisableGeoIPTracking()
		if svc.IsGeoIPTrackingEnabled() {
			t.Errorf("expected IsGeoIPTrackingEnabled to report false after DisableGeoIPTracking")
		}
		svc.Emit(context.Background(), "AFTER_DISABLE", nil)
		if props["ip"] != "0" {
			t.Errorf("expected ip property to be \"0\" after DisableGeoIPTracking, got %v", props["ip"])
		}

		svc.EnableGeoIPTracking()
		svc.Emit(context.Background(), "AFTER_ENABLE", nil)
		if _, exists := props["ip"]; exists {
			t.Errorf("expected no ip property after EnableGeoIPTracking, got %v", props["ip"])
		}
	})
}

func TestIdentifierHelpers(t *testing.T) {
	// machineid.ProtectedID and os.Executable can legitimately fail in some
	// sandboxed CI environments, so these only assert the helpers don't panic
	// and return a string — not that the string is non-empty.
	t.Run("GetBinaryPath does not panic", func(t *testing.T) {
		_ = analytics.GetBinaryPath()
	})

	t.Run("GetMachineID does not panic", func(t *testing.T) {
		_ = analytics.GetMachineID("analytics-test")
	})

	t.Run("GetDistinctID returns a UUID-shaped string", func(t *testing.T) {
		id := analytics.GetDistinctID()
		if id == "" {
			t.Errorf("expected a non-empty distinct ID")
		}
	})
}
