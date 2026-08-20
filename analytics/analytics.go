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
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/denisbrodbeck/machineid"
	"github.com/google/uuid"
	mixpanel "github.com/mixpanel/mixpanel-go"
)

const defaultHTTPTimeout = 10 * time.Second

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

// Service sends analytics events to Mixpanel, attaching privacy-safe
// identifiers (a per-run distinct ID, a hashed machine ID, a redacted binary
// path) and, optionally, caller-defined common properties to every event.
type Service struct {
	distinctID       string
	machineID        string
	binaryPath       string
	token            string
	startupTime      int64
	isAura           bool
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

// New creates a Service that reports to Mixpanel using mixPanelToken and
// mixpanelEndpoint. appName seeds the HMAC salt used by GetMachineID, so each
// consuming app must choose its own value — reusing an app's existing salt
// preserves continuity of its already-collected device IDs, and picking a new
// one starts a fresh series. uri is the Neo4j connection string used to
// detect Aura databases for the isAura base property.
func New(mixPanelToken, mixpanelEndpoint, appName, uri string, opts ...Option) *Service {
	options := serviceOptions{
		httpTimeout: defaultHTTPTimeout,
		enabled:     true,
	}
	for _, opt := range opts {
		opt(&options)
	}

	endpoint := strings.TrimRight(mixpanelEndpoint, "/")

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

	s := &Service{
		distinctID:       GetDistinctID(),
		machineID:        GetMachineID(appName),
		binaryPath:       GetBinaryPath(),
		token:            mixPanelToken,
		startupTime:      time.Now().Unix(),
		isAura:           isAura(uri),
		mp:               mpClient,
		commonProperties: options.commonProperties,
	}
	s.disabled.Store(!options.enabled)
	return s
}

// isAura returns true if uri looks like a Neo4j Aura connection string.
// With multi-DB, this could be either databases.neo4j.io or instances.neo4j.io.
var auraURIPattern = regexp.MustCompile(`(databases|instances)\.neo4j\.io\b`)

func isAura(uri string) bool {
	return auraURIPattern.MatchString(uri)
}

func (s *Service) Enable()         { s.disabled.Store(false) }
func (s *Service) Disable()        { s.disabled.Store(true) }
func (s *Service) IsEnabled() bool { return !s.disabled.Load() }

// EmitEvent sends event exactly as given, with no property merging. Use Emit
// for the common case of sending a named event with base/common/specific
// properties merged automatically.
func (s *Service) EmitEvent(event TrackEvent) {
	if s.disabled.Load() {
		return
	}
	slog.Info("Sending event to Mixpanel", "event", event.Event)
	if err := s.sendTrackEvent([]TrackEvent{event}); err != nil {
		slog.Error("Error while sending analytics events", "error", err.Error())
	}
}

// Emit sends eventName with the merge of three property layers: the
// Service's own base properties (distinct id, machine id, os/arch, binary
// path, uptime, insert id, token, aura flag), the common properties
// registered via WithCommonProperties (if any), and properties, specific to
// this one event. On key collision, properties wins over common properties,
// and base properties always win over both.
func (s *Service) Emit(eventName string, properties any) {
	if s.disabled.Load() {
		return
	}
	merged, err := combineProperties(s.getBaseProperties(), s.commonProperties, properties)
	if err != nil {
		slog.Error("Error while building analytics event properties", "event", eventName, "error", err.Error())
		return
	}
	s.EmitEvent(TrackEvent{Event: eventName, Properties: merged})
}

func (s *Service) sendTrackEvent(events []TrackEvent) error {
	sdkEvents := make([]*mixpanel.Event, 0, len(events))
	for _, e := range events {
		props, err := toPropertiesMap(e.Properties)
		if err != nil {
			return fmt.Errorf("marshal properties for event %q: %w", e.Event, err)
		}
		sdkEvents = append(sdkEvents, s.mp.NewEvent(e.Event, s.distinctID, props))
	}

	if err := s.mp.Track(context.Background(), sdkEvents); err != nil {
		return fmt.Errorf("mixpanel track error: %w", err)
	}
	slog.Info("Sent event to Mixpanel", "event", sdkEvents[0].Name)
	return nil
}

// GetBinaryPath returns the absolute path of the running binary via os.Executable.
// Symlinks are resolved so the real on-disk path is reported.
// Any occurrence of the user's home directory or username is redacted.
// Returns an empty string on failure.
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

// GetDistinctID returns a fresh UUID for use as a per-run distinct ID.
func GetDistinctID() string {
	id, err := uuid.NewV6()
	if err != nil {
		slog.Error("Error generating distinct ID for analytics", "error", err.Error())
		return ""
	}
	return id.String()
}
