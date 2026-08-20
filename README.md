# neo4j-common

Common Go packages shared across Neo4j projects.

## Layout

* `external/` – packages intended for import by other repos.
  * `external/logger`, `external/config`, `external/httpclient`, `external/analytics`, `external/version`, `external/utils`, `external/database`
* `internal/` – implementation details not part of the public contract. Only code inside this module may import them.

## Packages

### External

- `external/logger` - Structured slog wrapper with LevelVar and sensitive key redaction
- `external/config` - Layered configuration loading from env vars, config files, and CLI flags
- `external/httpclient` - HTTP client with timeout and max response size limits
- `external/analytics` - Mixpanel tracker with privacy-safe identifiers
- `external/version` - User-Agent version resolution via debug.ReadBuildInfo
- `external/utils` - Small utility helpers
- `external/database` - Database facade with Bolt and Query API backends

### Internal

- `internal/database/queryapi` - Internal Query API backend used by `external/database`. Public Query API SDK lives in the separate `query-go-sdk` repo.

## Usage

```go
import "github.com/LackOfMorals/neo4j-common/external/logger"
```

## Migration

This repo is a work-in-progress extraction from internal packages in neo4j-mcp-canary, aura-go-sdk, query-go-sdk, etc.
