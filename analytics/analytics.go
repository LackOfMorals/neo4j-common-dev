// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

package analytics

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/denisbrodbeck/machineid"
	"github.com/google/uuid"
	mixpanel "github.com/mixpanel/mixpanel-go"
)

const defaultHTTPTimeout = 10 * time.Second

// euMixpanelEndpoint is Mixpanel's EU-residency API endpoint. Projects
// created with EU data residency only accept events sent here — the default
// (US) endpoint silently drops them. See WithEuResidency.
const euMixpanelEndpoint = "https://api-eu.mixpanel.com"

// httpClientTransport adapts an HTTPClient into an http.RoundTripper, allowing
// the Mixpanel SDK to use an injectable client (including test stubs).
// The endpoint is stored here so we can rewrite the URL on every request —
// the SDK resolves its own internal URL before hitting the transport, which
// would otherwise bypass the configured endpoint.
type httpClientTransport struct {
	client   HTTPClient
	endpoint string
}

func (t *httpClientTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	path := strings.TrimLeft(req.URL.Path, "/")
	url := t.endpoint + "/" + path
	if req.URL.RawQuery != "" {
		url += "?" + req.URL.RawQuery
	}
	return t.client.Post(url, req.Header.Get("Content-Type"), req.Body)
}

// Service sends analytics events to Mixpanel. Every event carries a hashed
// machine ID automatically; every other property — including any per-run
// identifiers a caller wants — comes from WithCommonProperties or from the
// properties passed to Emit.
type Service struct {
	distinctID       string
	machineID        string
	mp               *mixpanel.ApiClient
	disabled         atomic.Bool
	commonProperties any
}

// serviceOptions holds the values Option functions configure before New
// constructs a Service.
type serviceOptions struct {
	httpClient       HTTPClient
	httpTimeout      time.Duration
	enabled          bool
	euResidency      bool
	commonProperties any
}

// Option configures a Service constructed by New.
type Option func(*serviceOptions)

// WithHTTPClient injects a custom HTTPClient, letting tests or callers
// intercept outbound Mixpanel calls instead of using the default http.Client.
func WithHTTPClient(client HTTPClient) Option {
	return func(o *serviceOptions) { o.httpClient = client }
}

// WithHTTPTimeout sets the timeout used by the default http.Client. Ignored
// if WithHTTPClient is also given.
func WithHTTPTimeout(d time.Duration) Option {
	return func(o *serviceOptions) { o.httpTimeout = d }
}

// WithEnabled sets the Service's initial enabled state. Defaults to true.
func WithEnabled(enabled bool) Option {
	return func(o *serviceOptions) { o.enabled = enabled }
}

// WithCommonProperties registers properties attached to every event sent via
// Emit, in addition to that call's own properties. Optional — a Service with
// no common properties simply omits this layer.
func WithCommonProperties(props any) Option {
	return func(o *serviceOptions) { o.commonProperties = props }
}

// WithEuResidency routes every request to Mixpanel's EU-residency endpoint,
// overriding whatever mixpanelEndpoint was passed to New — a project with EU
// data residency only accepts events sent there. Defaults to false.
func WithEuResidency(enabled bool) Option {
	return func(o *serviceOptions) { o.euResidency = enabled }
}

// New creates a Service that reports to Mixpanel using mixPanelToken and
// mixpanelEndpoint. appName seeds the HMAC salt used by GetMachineID, so each
// consuming app must choose its own value — reusing an app's existing salt
// preserves continuity of its already-collected device IDs, and picking a new
// one starts a fresh series.
//
// Since this package instruments apps that run on machines rather than
// individual end users, the machine ID also becomes Mixpanel's distinct_id
// — so Mixpanel's own unique-user, retention and funnel views reflect
// distinct machines rather than every process restart looking like a new
// anonymous user. If GetMachineID fails, distinct_id falls back to a fresh
// GetDistinctID UUID rather than leaving every affected install sharing an
// empty distinct_id.
func New(mixPanelToken, mixpanelEndpoint, appName string, opts ...Option) *Service {
	options := serviceOptions{
		httpTimeout: defaultHTTPTimeout,
		enabled:     true,
	}
	for _, opt := range opts {
		opt(&options)
	}

	endpoint := strings.TrimRight(mixpanelEndpoint, "/")
	if options.euResidency {
		endpoint = euMixpanelEndpoint
	}

	var mpClient *mixpanel.ApiClient
	if options.httpClient != nil {
		httpClient := &http.Client{Transport: &httpClientTransport{client: options.httpClient, endpoint: endpoint}}
		mpClient = mixpanel.NewApiClient(mixPanelToken,
			mixpanel.HttpClient(httpClient),
		)
	} else {
		mpClient = mixpanel.NewApiClient(mixPanelToken,
			mixpanel.ProxyApiLocation(endpoint),
			mixpanel.HttpClient(&http.Client{Timeout: options.httpTimeout}),
		)
	}

	machineID := GetMachineID(appName)
	distinctID := machineID
	if distinctID == "" {
		distinctID = GetDistinctID()
	}

	s := &Service{
		distinctID:       distinctID,
		machineID:        machineID,
		mp:               mpClient,
		commonProperties: options.commonProperties,
	}
	s.disabled.Store(!options.enabled)
	return s
}

func (s *Service) Enable()         { s.disabled.Store(false) }
func (s *Service) Disable()        { s.disabled.Store(true) }
func (s *Service) IsEnabled() bool { return !s.disabled.Load() }

// EmitEvent sends event exactly as given, with no property merging. Use Emit
// for the common case of sending a named event with the machine ID and
// common/specific properties merged automatically. ctx governs the outbound
// Mixpanel request — cancelling it (e.g. on caller shutdown) aborts the send
// instead of blocking for the full HTTP timeout; it does not affect the
// swallow-and-log error handling below, which is deliberate so a telemetry
// failure never breaks the caller's own control flow.
func (s *Service) EmitEvent(ctx context.Context, event TrackEvent) {
	if s.disabled.Load() {
		return
	}
	slog.Info("Sending event to Mixpanel", "event", event.Event)
	if err := s.sendTrackEvent(ctx, []TrackEvent{event}); err != nil {
		slog.Error("Error while sending analytics events", "error", err.Error())
	}
}

// Emit sends eventName with the merge of three property layers: the
// Service's machine ID, the common properties registered via
// WithCommonProperties (if any), and properties, specific to this one
// event. On key collision, properties wins over common properties, and the
// machine ID always wins over both. See EmitEvent for how ctx is used.
func (s *Service) Emit(ctx context.Context, eventName string, properties any) {
	if s.disabled.Load() {
		return
	}
	merged, err := combineProperties(s.getBaseProperties(), s.commonProperties, properties)
	if err != nil {
		slog.Error("Error while building analytics event properties", "event", eventName, "error", err.Error())
		return
	}
	s.EmitEvent(ctx, TrackEvent{Event: eventName, Properties: merged})
}

func (s *Service) sendTrackEvent(ctx context.Context, events []TrackEvent) error {
	sdkEvents := make([]*mixpanel.Event, 0, len(events))
	for _, e := range events {
		props, err := toPropertiesMap(e.Properties)
		if err != nil {
			return fmt.Errorf("marshal properties for event %q: %w", e.Event, err)
		}
		sdkEvents = append(sdkEvents, s.mp.NewEvent(e.Event, s.distinctID, props))
	}

	if err := s.mp.Track(ctx, sdkEvents); err != nil {
		return fmt.Errorf("mixpanel track error: %w", err)
	}
	slog.Info("Sent event to Mixpanel", "event", sdkEvents[0].Name)
	return nil
}

// GetBinaryPath returns the absolute path of the running binary via os.Executable.
// Symlinks are resolved so the real on-disk path is reported.
// Any occurrence of the user's home directory or username is redacted.
// Returns an empty string on failure. Not called automatically by Service —
// include it in WithCommonProperties or an Emit call if a caller wants it.
func GetBinaryPath() string {
	path, err := os.Executable()
	if err != nil {
		slog.Warn("Could not determine binary path for analytics", "error", err)
		return ""
	}

	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		slog.Warn("Could not resolve binary path symlinks for analytics", "error", err)
		// Continue with the unresolved path rather than returning empty.
	}

	return redactPath(path)
}

// redactPath removes personally identifiable segments from a file path.
// It replaces the home directory prefix first (most specific), then falls back
// to replacing any remaining occurrences of the username.
func redactPath(path string) string {
	// 1. Replace home directory prefix — works on Linux (/home/user),
	//    macOS (/Users/user) and Windows (C:\Users\user).
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		// filepath.Rel gives us the portion after the home dir, without
		// needing to worry about slash style differences on Windows.
		if rel, err := filepath.Rel(home, path); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.Join("<home>", rel)
		}
	}

	// 2. Fallback: replace the username directly in case the home dir lookup
	//    failed or the binary lives outside the home dir but still contains
	//    the username (e.g. /tmp/username/bin).
	if user, err := user.Current(); err == nil && user.Username != "" {
		// On Windows, Current().Username may be "DOMAIN\user" — strip the domain.
		username := user.Username
		if idx := strings.LastIndex(username, `\`); idx != -1 {
			username = username[idx+1:]
		}
		path = strings.ReplaceAll(path, username, "<user>")
	}

	return path
}

// GetMachineID returns a stable, privacy-safe machine identifier using the
// OS-provided hardware UUID, HMAC-hashed with appName so the raw system UUID
// is never exposed. appName also scopes the ID to the calling app — pass the
// same value consistently to preserve continuity of previously-collected
// device IDs. Returns an empty string on failure (e.g. insufficient
// permissions on some Linux configs).
func GetMachineID(appName string) string {
	id, err := machineid.ProtectedID(appName)
	if err != nil {
		slog.Warn("Could not retrieve machine ID for analytics", "error", err)
		return ""
	}
	return id
}

// GetDistinctID returns a fresh, per-run UUID. New uses this only as a
// fallback distinct_id when GetMachineID fails; it's exported in case a
// caller wants a per-run identifier of its own for some other purpose.
func GetDistinctID() string {
	id, err := uuid.NewV6()
	if err != nil {
		slog.Error("Error generating distinct ID for analytics", "error", err.Error())
		return ""
	}
	return id.String()
}
