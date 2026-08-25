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

	"github.com/LackOfMorals/neo4jPackages/external/analytics"
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
	return analytics.New("test-token", "http://localhost", "analytics-test", allOpts...)
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

	t.Run("sends the given event as-is when enabled, with no properties added", func(t *testing.T) {
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
			// EmitEvent does no merging — the SDK still adds its own token/
			// distinct_id/mp_lib bookkeeping, but we shouldn't have added
			// machine_id (that's only added by Emit).
			if _, exists := props["machine_id"]; exists {
				t.Errorf("expected no machine_id property from EmitEvent, got %v", props["machine_id"])
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
				svc := analytics.New("test-token", tc.mixpanelEndpoint, "analytics-test",
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
	t.Run("merges the machine ID with specific properties when no common properties are registered", func(t *testing.T) {
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
		if _, exists := props["machine_id"]; !exists {
			t.Errorf("expected machine_id to be present")
		}
	})

	t.Run("merges machine ID, common and specific properties, specific wins over common", func(t *testing.T) {
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
	})

	t.Run("caller-supplied properties do not require any package defaults to be present", func(t *testing.T) {
		var props map[string]any
		client := stubHTTPClient(func(url, contentType string, body io.Reader) (*http.Response, error) {
			events := decodeTrackedEvents(t, body)
			props, _ = events[0].Properties.(map[string]any)
			return okResponse(), nil
		})
		svc := newTestService(t, client)
		svc.Emit(context.Background(), "APP_STARTED", nil)

		for _, removed := range []string{"time", "uptime", "$os", "os_arch", "isAura", "$insert_id", "binary_path"} {
			if _, exists := props[removed]; exists {
				t.Errorf("expected %q to no longer be a package default, but it was present: %v", removed, props[removed])
			}
		}
	})

	t.Run("machine ID always wins over common and specific properties on collision", func(t *testing.T) {
		var props map[string]any
		client := stubHTTPClient(func(url, contentType string, body io.Reader) (*http.Response, error) {
			events := decodeTrackedEvents(t, body)
			props, _ = events[0].Properties.(map[string]any)
			return okResponse(), nil
		})
		svc := newTestService(t, client,
			analytics.WithCommonProperties(map[string]any{"machine_id": "bogus-common"}))
		svc.Emit(context.Background(), "APP_STARTED", map[string]any{"machine_id": "bogus-specific"})

		if props["machine_id"] == "bogus-common" || props["machine_id"] == "bogus-specific" {
			t.Errorf("expected the real machine_id to win over caller-supplied values, got %v", props["machine_id"])
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

func TestDistinctID(t *testing.T) {
	t.Run("the machine ID is used as Mixpanel's distinct_id when available", func(t *testing.T) {
		// machineid.ProtectedID can fail in some sandboxed CI environments
		// (see TestIdentifierHelpers) — skip rather than fail in that case,
		// since the fallback-to-GetDistinctID path this would otherwise be
		// exercising isn't what this test is about.
		machineID := analytics.GetMachineID("analytics-test")
		if machineID == "" {
			t.Skip("GetMachineID unavailable in this environment")
		}

		var props map[string]any
		client := stubHTTPClient(func(url, contentType string, body io.Reader) (*http.Response, error) {
			events := decodeTrackedEvents(t, body)
			props, _ = events[0].Properties.(map[string]any)
			return okResponse(), nil
		})
		svc := newTestService(t, client)
		svc.EmitEvent(context.Background(), analytics.TrackEvent{Event: "test_event"})

		if props["distinct_id"] != machineID {
			t.Errorf("expected distinct_id to be the machine ID %q, got %v", machineID, props["distinct_id"])
		}
	})
}

func TestEuResidency(t *testing.T) {
	t.Run("disabled by default: uses the endpoint passed to New", func(t *testing.T) {
		var gotURL string
		client := stubHTTPClient(func(url, contentType string, body io.Reader) (*http.Response, error) {
			gotURL = url
			return okResponse(), nil
		})
		svc := analytics.New("test-token", "http://localhost", "analytics-test",
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
		svc := analytics.New("test-token", "http://localhost", "analytics-test",
			analytics.WithHTTPClient(client), analytics.WithEuResidency(true))
		svc.EmitEvent(context.Background(), analytics.TrackEvent{Event: "test_event"})

		if gotURL != "https://api-eu.mixpanel.com/track?verbose=1" {
			t.Errorf("unexpected URL: got %s, want %s", gotURL, "https://api-eu.mixpanel.com/track?verbose=1")
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
