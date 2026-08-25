# Shared Code Analysis for Neo4j Go Projects

## Objective
Identify code that is duplicated across Neo4j Go projects and can be extracted into a common external module for reuse.

## Findings

### 1. Analytics / Mixpanel
**Projects:**
- `neo4j-mcp-canary/internal/analytics`
- `neo4j-mcp-canary-1/internal/analytics`
- `mcpCanary-v2/internal/events/mixpanel`

**Shared patterns:**
- `NewAnalytics` / `NewAnalyticsWithClient` with injectable HTTP client
- `EmitEvent`, `sendTrackEvent` using mixpanel-go SDK
- DistinctID generation via `github.com/google/uuid`
- MachineID via `github.com/denisbrodbeck/machineid`
- Binary path redaction for privacy
- Event factory helpers in `events.go`

**Extract candidate:** `github.com/neo4j-contrib/go-analytics`
- Core `Tracker` interface, Mixpanel implementation
- Privacy helpers: `GetMachineID`, `GetDistinctID`, `GetBinaryPath`, `redactPath`

### 2. Logging
**Projects:**
- `neo4j-mcp-canary/internal/logger`
- `neo4j-mcp-canary-1/internal/logger`
- `official-neo4j-mcp/internal/logger`

**Shared patterns:**
- `Service` struct wrapping `*slog.Logger` + `*slog.LevelVar`
- Custom log levels: Debug, Info, Notice, Warning, Error, Critical, Alert, Emergency
- `replaceAttr` for uppercase level mapping + sensitive key redaction
- `ValidLogLevels`, `ValidLogFormats`
- `New`, `SetLevel`, `Init` global helper

**Extract candidate:** `github.com/neo4j-contrib/go-logger`
- Slog wrapper with level var and redaction

### 3. Configuration
**Projects:**
- `neo4j-mcp-canary/internal/config`
- `neo4j-mcp-canary-1/internal/config`
- `mcpCanary-v2/internal/config`
- `official-neo4j-mcp/internal/config`

**Shared patterns:**
- Env var loading with defaults: `GetEnv`, `GetEnvWithDefault`
- Bool/int32 parsing helpers: `ParseBool`, `ParseInt32`
- CLI overrides precedence: flags > env > config file > defaults
- Validation logic
- Deprecation warnings for old env vars

**Extract candidate:** `github.com/neo4j-contrib/go-config`
- `Config` loader with layered sources
- Typed getters, validation helpers

### 4. HTTP Client
**Projects:**
- `httpClient/httpClient.go`
- `neo4jtfl/internal/httpClient/httpClient.go`
- `aura-go-sdk/internal/httpclient/httpclient.go`
- `query-go-sdk/internal/httpclient/httpclient.go`
- `mcpCanary-v2/internal/httpclient/httpclient.go`

**Shared patterns:**
- `HTTPRequestsService` with configurable timeout
- `DefaultMaxResponseSize = 10 MiB`
- Response size limiting via `io.LimitReader`
- Context-aware request creation
- Status code validation

**Extract candidate:** `github.com/neo4j-contrib/go-httpclient`
- Generic HTTP executor with timeout, max response size, retry hooks

### 5. User-Agent Version Handling
**Projects:**
- `aura-go-sdk/client.go`
- `query-go-sdk/client.go`

**Shared patterns:**
- `clientVersionFallback = "development"`
- `moduleName` constant
- `resolveClientVersion` using `runtime/debug.ReadBuildInfo`
- `ClientVersion` var set in `init()`
- User-Agent default: `"aura-go-sdk/" + ClientVersion`

**Extract candidate:** `github.com/neo4j-contrib/go-version`
- `ResolveModuleVersion(modulePath)` helper
- `UserAgent` builder

### 6. Utilities
- Redaction helpers, base64 auth, JSON marshal/unmarshal
- Date validation

## Recommended Common Repo Structure

```
github.com/neo4j-contrib/neo4j-common
├── logger/
│   └── logger.go
├── config/
│   ├── loader.go
│   ├── env.go
│   └── types.go
├── httpclient/
│   ├── client.go
│   └── options.go
├── analytics/
│   ├── tracker.go
│   ├── mixpanel.go
│   └── privacy.go
├── version/
│   └── version.go
└── utils/
    └── utils.go
```

## Next Steps
1. Create `github.com/neo4j-contrib/neo4j-common` repo
2. Extract each package with minimal dependencies
3. Add tests mirroring existing tests
4. Update `go.mod` in each consumer to replace internal packages with external import
5. Maintain compatibility via thin adapter shims during migration

## Notes
- Mixpanel is optional dependency; keep analytics package interface-driven
- Logging redaction keys may need to be configurable per project
- Config loader should remain flexible for project-specific fields
