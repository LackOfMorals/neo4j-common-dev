# Agents Guide – neo4jPackages

This repo is a work-in-progress extraction of shared Go code used across Neo4j projects. Agents should follow the layout and conventions below.

## Repository layout

* `external/` – public packages meant for import by other repos.
  * `external/logger`, `external/config`, `external/httpclient`, `external/analytics`, `external/version`, `external/utils`, `external/database`
* `internal/` – implementation details not part of the public contract. Only code inside this module may import `internal/`.
  * `internal/database/queryapi` – internal Query API backend used by `external/database`. Public Query API SDK lives in the separate `query-go-sdk` repo.

Module: `github.com/LackOfMorals/neo4jPackages`
Go version: `1.25.0`

## Coding conventions

* One flat package per directory, no `internal/` sub-packages for public concerns. Existing packages are flat and independently testable.
* Constructors are pure – `New` never does network I/O. Version checks and discovery are lazy and run on first `Execute/ExecuteStream`.
* Use functional `Option` pattern for Service construction and per-call `QueryOption`.
* Prefer stdlib. Minimal third-party deps. Current deps: `github.com/neo4j/neo4j-go-driver/v6`, `github.com/google/uuid`, `github.com/mixpanel/mixpanel-go`, `github.com/denisbrodbeck/machineid`.
* Error handling: wrap with `%w`. Typed errors only for actionable cross-backend cases, e.g. `StatementError`, `TransportError`, `VersionError`, `CommitError`.
* Public API in `external/database`:
  * `Service`, `New(uri, opts...)`, `Execute`, `ExecuteStream`, `BeginTx`, `Close`
  * Types: `Node`, `Relationship`, `Path`, `Point`, `Duration`, `Vector`, temporal types `Date`, `LocalTime`, `Time`, `LocalDateTime`, `DateTime`, `Record`, `Result`, `StreamResult`, `Summary`, `Tx`
  * `TransactionMode` `Implicit|Explicit`, `AccessMode` `Write|Read`, `WithDatabaseOverride` per-call
* `external/httpclient` provides `Do` and `DoStreaming`. Request building is factored into `newRequest` to avoid duplication between buffered and streaming paths.

## Testing

* Real server available for integration tests: `http://localhost:7474`, user `neo4j`, password `password`. Bolt on `bolt://neo4j:password@localhost:7687`.
* Integration smoke files in repo root:
  * `integration_test.go` – buffered, streaming, explicit mode, Tx lifecycle, Vector decode
  * `smoke_main.go`
  * `test_explicit_stream.go`
  * `verify_wire.go`
  * `test_db_override.go`
* Run examples:
  ```bash
  go run examples/database/main.go
  ```
* Build only packages:
  ```bash
  go build ./external/... ./internal/...
  ```
* No remote push – work offline, local commits only.

## Database backend notes

* `external/database` facade selects backend by URI scheme:
  * `neo4j`, `neo4j+s`, `neo4j+ssc`, `bolt`, `bolt+s`, `bolt+ssc` → Bolt v6 driver
  * `http`, `https` → Query API
* Query API version gate: lazy `sync.Once` check on first call. Minimum is CalVer `2026.07.0` or SemVer `5.27`. Parsing uses `ParseCalVer`/`CompareCalVer`.
* Streaming Query API uses `application/vnd.neo4j.query.v1.1+jsonl`. Line size limit 10 MiB.
* Explicit single-statement mode begins a tx, runs the statement, commits; on commit failure `CommitError.Ambiguous` distinguishes transport loss vs server rejection. Best-effort rollback on error.
* `WithMaxResultBytes` option propagates to `httpclient` for buffered responses. Use `ExecuteStream` for large results.

## What not to do

* Do not import `internal/database/queryapi` from outside the module.
* Do not add network I/O to `New`. Keep constructors pure.
* Do not introduce `internal/` sub-packages for public concerns – repo has no `internal/` elsewhere.
* Do not push to remote. Local commits only.

## Useful files

* `external/database/service.go`, `backend.go`, `backend_queryapi.go`, `backend_bolt.go`, `types.go`
* `external/httpclient/client.go` – `Do` / `DoStreaming`
* `internal/database/queryapi/service.go`, `execute.go`, `stream.go`, `value.go`, `encode.go`, `version.go`
* `examples/database/main.go`

When making changes, run the smoke/integration files against localhost:7474 to confirm behavior before committing.
