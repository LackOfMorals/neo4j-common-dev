# neo4jPackages – Gap Analysis & Implementation Plan

_Date: session handoff follow-up. Baseline: `go build ./external/... ./internal/...`, `go vet`, and `go test ./external/...` all pass. Verified against the live server at `http://localhost:7474` (Query API) and `bolt://neo4j:password@localhost:7687` in prior sessions._

## 1. What is done

**Repo shape**
- Module `github.com/LackOfMorals/neo4jPackages`, Go 1.25.0, LICENSE present.
- `external/`: `analytics`, `config`, `database`, `httpclient`, `logger`, `utils`, `version` – flat, one package per dir, functional-Option constructors, black-box tests for analytics/config/httpclient/logger.
- `internal/database/queryapi`: typed-JSON codec + wire structs (module-private by construction).
- `examples/`: config, database, httpclient, logger, sendEvent.
- `AGENTS.md` agent guide (one known doc/code mismatch, §3.E4).

**`external/httpclient`** – `Do` (buffered, 10 MiB cap) + `DoStreaming` (unbuffered) sharing `newRequest`; `WithMaxResponseSize`, `WithDefaultHeaders`, `WithHTTPClient`, `WithTimeout`. Tested.

**`external/database` facade**
- `New(uri, opts...)` scheme dispatch: `neo4j(+s|+ssc)://`, `bolt(+s|+ssc)://` → v6 driver; `http(s)://` → Query API. `Service.Close`, `Execute`, `ExecuteStream`, `BeginTx`.
- Constructor options: `WithBasicAuth`, `WithBearerToken`, `WithLogger`, `WithTimeout`, `WithDatabase`, `WithMaxResultBytes`. Per-call: `WithTransactionMode`, `WithAccessMode`, `WithDatabaseOverride`.
- Closed value set: `Node`, `Relationship`, `Path`, `Point`, `Duration`, `Vector`, `Date`, `LocalTime`, `Time`, `LocalDateTime`, `DateTime`, plus scalars/lists/maps. `Record` with `Keys/Values/Get/At/GetNode/GetRelationship`. `Result`, `StreamResult`, `Summary`, `Tx` + `CommitResult`.
- Typed errors: `VersionError`, `StatementError`, `TransportError`, `CommitError` (with `Ambiguous`).
- Query API backend: buffered implicit + explicit single-statement (begin → run → commit, best-effort rollback, `CommitError.Ambiguous` on transport loss); streaming implicit + explicit; `accessMode` in request body for Read; lazy version gate (`sync.Once`) with CalVer `2026.07.0` / SemVer `5.27` cutoff; bookmarks propagated (explicit commit → `Summary.Bookmarks`; summary events; buffered response).
- Bolt backend: `neo4j.ExecuteQuery` + `EagerResultTransformer`, reader/writer routing, per-call database override, value mapping for Node/Relationship/Path/Point2D/Point3D/Duration/Date/LocalTime/Time/LocalDateTime/lists/maps.
- Live-verified (prior sessions): buffered, streaming, explicit mode, Tx lifecycle, db override, Path/Relationship wire shapes, Vector decode.

**`internal/database/queryapi`** – `DecodeValue`/`EncodeValue` (scalars, Base64, List/Map, Node, Relationship, Path, Point, Duration, Vector, temporals, `RawTypedValue` fallback), `QueryResponse`/`QuerySummary`/`ExecuteRequest`/`StreamEvent` wire types, `ParseVersion`.

## 2. Gap inventory

Legend: **C** = correctness/contract, **S** = security, **T** = test, **D** = docs, **H** = hygiene, **A** = architecture decision.

### Correctness (C)

| # | Gap | Where | Notes |
|---|-----|-------|-------|
| C1 | Bolt `mapValue` falls through to `default: return v` for driver `time.Time` (ZonedDateTime) and generic `neo4j.Vector[T]`, leaking raw driver types into the closed value contract | `backend_bolt.go` mapValue | Verified against v6.2.0: graph type union includes `time.Time` and `Vector[T]` (int8…float64). Query API path returns our `DateTime`/`Vector` – backends are inconsistent. |
| C2 | `EncodeValue` silently stringifies unknown Go types (its `default` branch JSON-marshals the value and wraps it as `$type: String`) – an `int32`, `uint`, `float32`, or `time.Time` param silently becomes a **String** param, changing Cypher semantics | `internal/database/queryapi/encode.go` | Original design: error on unsupported types. Must handle all integer/float widths explicitly. |
| C3 | Lossy encoders: `encodeDuration` drops `Nanos`; `encodeLocalTime`/`encodeTime`/`encodeLocalDateTime`/`encodeDateTime` drop sub-second `Nano` | `encode.go` | Round-tripping a decoded facade value back as a param loses precision. |
| C4 | `encodePoint` emits invalid WKT for 3D points (`POINT (x y z)` must be `POINT Z (x y z)`); uses `%g` (may emit exponent form) | `encode.go` | 2D encoding is fine. |
| C5 | Version check latches **any** first-call failure forever (`sync.Once` + cached `versionErr`) – one transient network blip permanently poisons the service | `backend_queryapi.go` `checkVersion` | Design intent was: cache success; retry transient (transport) failures; latch only definitive failures (`VersionError`, 4xx). |
| C6 | Query API **buffered** response drops counters, result timings, and notifications entirely – the facade uses `queryapi.QueryResponse` which has no counters/timing/notifications fields; the richer `queryapi.QuerySummary` (with full counters) exists but is unused on this path | `backend_queryapi.go` executeBuffered, `response.go` | `Result.Summary` from a buffered Query API call currently has only `QueryType`, `Database`, `Bookmarks`. |
| C7 | Facade `Summary`/`Counters` are underspecified vs the design and vs both backends' capabilities: `Counters` missing `LabelsAdded/LabelsRemoved`, `IndexesAdded/IndexesRemoved`, `ConstraintsAdded/ConstraintsRemoved`, `ContainsUpdates`, `ContainsSystemUpdates`; `Summary` missing `Notifications` (and there is no `Notification`/`NotificationPosition` type) | `types.go` | Additive change, non-breaking. Bolt v6 `ResultSummary` exposes `Notifications() []Notification` and a full `Counters` interface. |
| C8 | Bolt `mapSummary` maps only 5 counters, no Notifications, no `Database`, no `ContainsUpdates` | `backend_bolt.go` | v6 `ResultSummary` has no `Bookmarks()` (v6 moved bookmarks to sessions – see A5); map what exists. |
| C9 | `txBackend.txRun` does not forward `AccessMode`; `Tx.Run` has no way to express it at all | `backend_queryapi.go` | The non-tx execute paths send `accessMode: Read`. Inconsistent. |
| C10 | Explicit-mode Query API streaming buffers **all** records into memory before returning `StreamResult`, then commits in `Close` (explicitly marked temporary); the implicit stream path also fully drains before returning | `backend_queryapi.go` executeStream | Defeats the purpose of streaming (unbounded memory, server-side tx held for the whole drain). Commit-response bookmarks are discarded. Close uses the caller's ctx, which may already be done. The facade `StreamResult` type itself is buffered-only (`records []Record`), so true laziness requires a type redesign (A1). |
| C11 | Bolt `executeStream` is a pure eager fallback (same `EagerResultTransformer` as buffered) | `backend_bolt.go` | v6 driver supports real streaming; needs a channel-backed transformer (A1). |
| C12 | Bolt `executeBuffered`/`executeStream` ignore `TransactionMode.Explicit` – Explicit on Bolt silently runs Implicit | `backend_bolt.go` | Silent semantics mismatch. Minimum: loud behavior + docs; better: implement (A3). |
| C13 | `WithTimeout` is a no-op for Bolt (driver built with no config); `WithLogger` is a no-op for **both** backends (stored on `Service`, never read) | `service.go`, `backend_bolt.go`, `backend_queryapi.go` | Users expecting timeout/logging get neither. |
| C14 | `decodePoint` swallows `strconv` errors on SRID/X/Y/Z (benign today because the regex pre-validates, but sloppy and breaks on overflow → silent `±Inf`) | `value.go` | Surface the errors. |
| C15 | Internal `queryapi.Service` + `Execute`/`ExecuteStream`/`ExecuteStreamResult` + `NewStreamScanner` + legacy `Result`/`Row` are **dead code** – the facade builds its own requests via `httpclient` and uses only the codec + wire types | `internal/database/queryapi/{service,execute,stream,response}.go` | ~300 lines duplicated against the facade, which has since diverged (per-call db override, accessMode, explicit tx, own error types). A2. |

### Security (S)

| # | Gap | Where |
|---|-----|-------|
| S1 | Database name interpolated into URLs with no validation (`/db/%s/query/...`) – a name containing `/` or `..` alters the request path; also flows into Bolt session config | `backend_queryapi.go` (endpoints), `service.go` `WithDatabaseOverride`/`WithDatabase` |
| S2 | `WithBasicAuth` + `WithBearerToken` given together: last-one-wins silently (security-relevant footgun) | `service.go` `New` |
| S3 | Undocumented data-sensitivity: `TransportError.Body` holds raw server responses (may echo statement text/params); URIs carry embedded credentials; `DoStreaming` has no size cap | `errors.go`, `README` |

### Tests (T)

| # | Gap |
|---|-----|
| T1 | `external/database` and `internal/database/queryapi` have **zero** test files. |
| T2 | The five repo-root "integration" files (`integration_test.go`, `smoke_main.go`, `test_explicit_stream.go`, `verify_wire.go`, `test_db_override.go`) are `package main` programs – `go test` ignores them, `go run ./...` fails on the collision, and each had to be copied to `/tmp` to run. `integration_test.go` is misnamed. |
| T3 | No unit coverage for the value codec (encode/decode round-trips), version parsing, or streaming scan logic (httptest stubs exist as a convention in this repo). |

### Docs (D)

| # | Gap |
|---|-----|
| D1 | No `// Package database` doc comment and **no** doc comments on any exported symbol in `external/database` (6 files). `httpclient` has partial docs; audit the rest. |
| D2 | README: title still "neo4j-common"; import path still `github.com/LackOfMorals/neo4j-common/...`; no Getting Started, no working examples, no options reference, no security notes. |
| D3 | No `ExampleXxx` doc tests for `database` (or httpclient). |
| D4 | `AGENTS.md` says version parsing uses `ParseCalVer`/`CompareCalVer` – actual functions are unexported facade-local `parseCalVer`/`compareCalVer` (`backend_queryapi.go`) with a different `ParseVersion` in `internal/queryapi/version.go`. Also references the root smoke files (§T2) which are moving. |
| D5 | No `CONTRIBUTING.md` / `CHANGELOG.md` / `llms.txt` (optional, pre-contrib-repo). |

### Hygiene (H)

| # | Gap |
|---|-----|
| H1 | Uncommitted: `go.mod` (driver moved indirect → direct – correct, commit) and `examples/database/main.go` (switched to local `MATCH (c:Company)` queries – not portable; recommend revert to generic queries). |
| H2 | Four `.DS_Store` files are **tracked** in git (`./.DS_Store`, `examples/.DS_Store`, `internal/.DS_Store`, `internal/database/.DS_Store`); `.gitignore` has no `.DS_Store` entry. |
| H3 | 16 files fail `gofmt -l` (all of `external/database`, 7 of `internal/queryapi`, 2 root mains, `examples/logger/logger.go`). |
| H4 | `SHARED_CODE_ANALYSIS.md` at repo root – working doc; decide keep / move to `docs/`. |

## 3. Architecture decisions (need user input)

**A1 – True lazy streaming (the big one).** Redesign the facade `StreamResult` from a buffered slice to a lazy iterator (keep the public shape: `Keys()`, `Records()`, `Summary()`, `Close()`):
- Query API implicit: pull from a `bufio.Scanner` on demand; `Close` = close body.
- Query API explicit: pull on demand; **commit when the stream completes successfully** (Summary event/EOF), rollback on mid-stream error or early `Close`; commit-response bookmarks land in `Summary().Bookmarks`; use a fresh bounded context for the commit call so a cancelled caller ctx can't kill the commit.
- Bolt: v6 cursor consumed by a background goroutine pushing mapped records through a channel; the facade iterator reads the channel; summary from `Consume`.
This is a non-breaking change to the public method surface but changes internals of both backends. **Recommend: yes, do it** – it's the core value of `ExecuteStream`.

**A2 – Fate of the dead internal `queryapi.Service`.** Options: (a) delete the HTTP plumbing, keep the codec + wire types as the internal package's purpose; (b) refactor the facade to route through it (requires adding per-call db override, accessMode, explicit tx, facade error mapping to it – i.e., re-implementing the divergence). **Recommend: (a) delete** – the facade has diverged too far; the public Query API SDK lives in `query-go-sdk` anyway. Also consolidate version parsing: move the user-supplied `ParseCalVer`/`CompareCalVer` into `internal/queryapi/version.go` (module-visible) and have the facade call it, retiring both the facade-local `parseCalVer`/`compareCalVer` and the divergent `ParseVersion`.

**A3 – Bolt `Explicit` mode.** Currently silently runs Implicit. Options: (a) implement single-statement explicit via a v6 session (begin → run → commit, same `CommitError` semantics); (b) keep implicit for now, log a warning, document loudly. **Recommend: (b) for this pass** (explicit-mode semantics were explicitly parked until implementation is stable – i.e., after A1), but make the behavior documented + logged rather than silent.

**A4 – Bolt `BeginTx` (multi-statement).** Currently returns "transactions not supported". Options: (a) implement via v6 session for parity; (b) document as Query-API-only in v1. **Recommend: (b)** – it's a real feature, not a gap-fix; do it as a follow-up. (A loud error is already the behavior.)

**A5 – Bolt bookmarks.** v6 `ResultSummary` no longer exposes `Bookmarks()` (moved to `Session.LastBookmarks()` / status objects). The `ExecuteQuery`+transformer path used today has no obvious bookmark handle. **Recommend:** research during A1 implementation (session-based execution may be required to surface them); until then leave `Summary.Bookmarks` empty on Bolt and document. Additive either way.

**A6 – `DateTime` consistency for Bolt.** Map driver `time.Time` → facade `DateTime{LocalDateTime, Offset, Zone}` (same shape the Query API path produces), instead of leaking `time.Time`. Zone will often be a fixed-offset representation (Bolt carries instant + offset). **Recommend: yes** – backends must agree on the closed value set.

**A7 – Add `Notification` types + full `Counters`/`Summary` (C6–C8).** Additive, non-breaking, both backends can populate. **Recommend: yes.**

**A8 – `Tx` access mode (C9).** Add `TxOption` (e.g., `WithAccessMode`) to `BeginTx`, stored on the tx, sent as `accessMode` on each tx run. **Recommend: yes, small.**

## 4. Phased plan

Each phase ends green: `gofmt -l` clean, `go build ./...`, `go vet ./...`, `go test ./...` (unit tests; integration tests skip when the local servers are down), and a live-server smoke of `examples/database`.

### Phase 0 – Hygiene & baseline (~small)
1. `gofmt -w` the 16 files (H3).
2. `git rm --cached` the four `.DS_Store` files + add `.DS_Store` to `.gitignore` (H2).
3. Commit the `go.mod` driver fix; revert `examples/database/main.go` to portable queries (H1) – confirm A-n/a (default: revert).
4. Move the five root mains into `examples/` with sane names (one dir each, e.g., `examples/smoke-buffered/`, `examples/smoke-explicit-stream/`, `examples/verify-wire/`, `examples/db-override/`; merge `smoke_main.go` into the first). This fixes `go run ./...` and the misnamed `integration_test.go` (T2, H4).
5. Update `AGENTS.md` references accordingly (D4 partial).
6. Decide `SHARED_CODE_ANALYSIS.md`: keep at root or move to `docs/` (H4) – default: keep at root, it's short.

**Commit:** one or two small commits.

### Phase 1 – Correctness & security fixes (no public API shape changes beyond A7/A8 additions)
1. **C1/A6** – Bolt `mapValue`: add `time.Time` → `DateTime` and `neo4j.Vector[T]` (all element types) → `Vector` cases.
2. **C2** – `EncodeValue`: explicit cases for `int8/16/32/64`, `uint/8/16/32/64`, `float32`; **remove the silent-String fallback – return an error** for unsupported types (document which types are encodable on the function).
3. **C3/C4** – fix `encodeDuration` (nanos), temporal encoders (sub-second nanos), `encodePoint` (3D `POINT Z`, `strconv.FormatFloat` full precision).
4. **C14** – surface `decodePoint` parse errors.
5. **C5** – version check: cache success permanently; retry transport failures (don't latch); latch `VersionError` and clean 4xx. Keep the caller-independent context + 10 s bound.
6. **C7/A7** – extend facade `Counters` (+5 fields), add `Notification`/`NotificationPosition`, `Summary.Notifications`; extend `queryapi` wire types so the **buffered** response carries counters/timings/notifications (C6); map them in both backends (Bolt `mapSummary` full mapping incl. `Database`, `ContainsUpdates` – C8).
7. **C9/A8** – `BeginTx(ctx, opts ...TxOption)` with `WithAccessMode`; `txBackend` sends `accessMode` on runs.
8. **S1** – validate database names (`^[A-Za-z0-9_-]+$`) in one facade choke point (`WithDatabase`, `WithDatabaseOverride`, and URI-parsed value if any) – fail `New`/the call with a named error.
9. **S2** – `ErrConflictingAuth` when both auth options are set.
10. **C13** – wire `WithTimeout` into Bolt driver config; thread `WithLogger` into both backends (log version check result, explicit-mode warning from A3, commit/rollback outcomes at debug).
11. **A3** – Bolt `Explicit`: document + `logger.Warn` (kept working as implicit) until post-A1.
12. **S3** – doc comments: `TransportError.Body` (may contain sensitive data – don't log verbatim), `DoStreaming` (no size cap; caller's responsibility), `New`/URI creds (don't log URIs).

**Tests:** unit tests for `EncodeValue` (all int/float widths, error cases), duration/point encode-decode round-trips, version cutoff table (calver/semver/edge strings), db-name validation, auth-conflict. All httptest/table-driven, no live server.

### Phase 2 – True lazy streaming (A1) + A2 cleanup
1. **A2** – delete dead `queryapi.Service` HTTP plumbing + legacy `Result`/`Row` + `NewStreamScanner`; move `ParseCalVer`/`CompareCalVer` into `internal/queryapi/version.go`, facade consumes them; fix `AGENTS.md` naming (D4).
2. Redesign `StreamResult` internals: `keys`, a `next() (Record, bool)` pull fn (or equivalent), a summary supplier, and `closeFn`. Public methods unchanged. Document the "yields one error then stops" + "drain or Close" contract on the type.
3. Query API implicit: scanner-pull iterator (line cap = `defaultMaxStreamLineSize`, surfaced as a real error).
4. Query API explicit: begin → run-stream; commit on successful completion with commit bookmarks → `Summary()`; rollback on error/early close (best-effort, never masks the original error); bounded fresh ctx for commit/rollback.
5. Bolt: channel-backed lazy transformer (background consume goroutine, `Consume` for summary; error propagation; early `Close` stops the goroutine). Investigate bookmarks while here (A5).
6. **T2/T3** – real `external/database/integration_test.go` (`package database_test`): connect-ping both servers, `t.Skip` when unreachable (no build tags, per earlier decision). Cover buffered/streaming/explicit/Tx/bookmarks/db-override and the full type set (Node/Rel/Path/Point/Duration/Vector/Date/Time/LocalTime/LocalDateTime/DateTime/Int/Float/String/Base64/Null) on both backends. The five example mains from Phase 0 fold their coverage in here (or stay as runnable demos – keep both: tests assert, examples demonstrate).
7. Unit tests for streaming scan logic (httptest): header/record/summary/error event sequences, oversized line, mid-stream cancel, early close, explicit commit-on-drain vs rollback-on-error (assert the DELETE/commit endpoint hits).

### Phase 3 – Docs (ship-ready surface)
1. `// Package database` doc comment: closed value set, backend dispatch, streaming contract, error model, security notes (URIs, `TransportError.Body`).
2. Doc comments on every exported symbol in `external/database`; audit `httpclient`/`utils`/`version` (fill gaps only).
3. README rewrite: correct title/module path, Getting Started (both backends, from `examples/database`), options reference tables (constructor + query options), streaming + Tx examples, error-handling examples (`errors.As` for `StatementError`/`CommitError`), security notes, migration note.
4. `ExampleXxx` doc tests in `external/database` using an `httptest` Query API stub (deterministic, runnable in `go test` without a live server).
5. Final `AGENTS.md` pass (layout after Phase 0/2 moves, function names, test commands).
6. Optional (flag for user): `CONTRIBUTING.md`, `CHANGELOG.md` (v0.1.0 entry), `llms.txt`.

### Phase 4 – Follow-ups (separate passes, not in this plan's scope)
- Bolt explicit mode via session (A3 → implement).
- Bolt multi-statement `BeginTx` (A4).
- Bolt bookmark surfacing if research (A5) shows it needs session-based execution.
- Contrib-repo move + module rename when ready.

## 5. Verification (per phase and at the end)
- `gofmt -l .` → empty; `go build ./...`; `go vet ./...`; `go test ./...` (unit + skip-aware integration).
- `go run examples/database` against the live servers – all sections print.
- Phase 2 additionally: streaming a >10 MiB single-row query is bounded by the line-cap error; `ExecuteStream` early-`break` on the explicit path results in a server-side rollback (verify with a follow-up `COUNT` query).

## 6. Out of scope
- `query-go-sdk` repo changes (separate repo).
- Analytics/logger/config/version/utils feature work (they're stable; docs audit only).
- CI setup, release pipeline.
