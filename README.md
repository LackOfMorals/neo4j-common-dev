# neo4j-common

Common Go packages shared across Neo4j projects.

## Packages

- `logger` - Structured slog wrapper with LevelVar and sensitive key redaction
- `config` - Layered configuration loading from env vars, config files, and CLI flags
- `httpclient` - HTTP client with timeout and max response size limits
- `analytics` - Mixpanel tracker with privacy-safe identifiers
- `version` - User-Agent version resolution via debug.ReadBuildInfo
- `utils` - Small utility helpers

## Usage

```go
import "github.com/LackOfMorals/neo4j-common/logger"
```

## Migration

This repo is a work-in-progress extraction from internal packages in neo4j-mcp-canary, aura-go-sdk, query-go-sdk, etc.
