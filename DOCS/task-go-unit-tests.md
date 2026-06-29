# Task: Add Go Unit Tests

## Objective

Establish a unit-test suite for the Go backend (`space-mosquito/`). Tests must exercise Go packages in isolation using standard `testing` patterns (`httptest`, `t.TempDir`, `httptest.Server`, table-driven tests).

**Out of scope for this task:**

- Docker Compose, container startup, or volume-mount behavior
- Browser extension code (`firefox-extension/`, `chrome-extension/`)
- End-to-end shell scripts in `tests/` (e.g. `test_phase5_mcp.sh`)
- Live Confluence instances, real SSO flows, or headless Chromium (go-rod) launches
- PostgreSQL integration tests requiring a running database (defer to a separate task if needed)

## Current State

| Package | Test file | Coverage |
|---------|-----------|----------|
| `internal/session` | `session_test.go` | Good — store encrypt/decrypt, expiry, validation guards, `lastSlash` |
| `internal/scraper` | `discovery_test.go` | Minimal — JSON unmarshalling only; no logic tests |
| All other packages | — | None |

Run tests:

```bash
cd space-mosquito
go test ./...
```

## Testing Conventions

- Use `t.TempDir()` for filesystem tests; never write to repo paths or `/tmp` in tests.
- Use `httptest.NewServer` / `httptest.NewRecorder` for HTTP handlers and external API mocks.
- Use table-driven tests for URL parsing, config defaults, and pure functions.
- Inject dependencies via constructors already in place (`NewStore`, `NewWriter`, `NewJobManager`, etc.).
- For DB-dependent code, either:
  - test handler/MCP layers with a small `db` interface stub, or
  - mark `//go:build integration` tests separately (not part of this task's deliverable).
- Keep `rateLimit` / `retryDelay` sleeps out of the hot path in tests — use short timeouts or refactor injectable clocks only if strictly necessary (prefer mocking HTTP at the transport layer).
- Target: all new tests pass with `go test -race ./...`.

---

## Test Inventory by Package

### 1. `internal/config`

**Functions to test**

| Function | What to verify |
|----------|----------------|
| `Load` | YAML parsing, default values applied when fields are empty |
| `DatabaseConfig.DSN` | DSN string format; `DATABASE_URL` env override |
| `ParseCronDuration` | Valid/invalid duration strings |
| `CronJobConfig.ParseMaxDuration` | Default `4h` when nil/empty; custom values |

**Edge cases**

- Missing config file → error
- Malformed YAML → error
- Partial config (only `database` section) → other defaults filled in
- `database.port: 0` → defaults to `5432`
- `mcp.port: 0` → defaults to `8081`
- `mcp.session_timeout: 0` → defaults to `3600`
- `embedder.model` empty → defaults to `nomic-embed-text`
- Cron intervals empty → `full_crawl` → `24h`, `incremental` → `2h`
- Incremental `detection` empty → defaults to `dom`
- `DATABASE_URL` set in env → `DSN()` returns env value, ignores struct fields
- Invalid duration in `ParseMaxDuration` (e.g. `"not-a-duration"`) → error

---

### 2. `internal/session`

**Already covered:** store round-trip, wrong key, delete, missing file, empty key, expiry, validation guards (no URL/cookies/bad URL), file permissions, key truncation/padding, `lastSlash`.

**Additional tests needed**

| Function | What to verify |
|----------|----------------|
| `Session.AsHeaders` | Cookie header format, `X-Atlassian-Token`, `Accept` |
| `GetSpaceKeyFromURL` | Cloud, custom domain, Server `/display/` paths |
| `GetSpaceNameFromURL` | Uppercased key from URL |
| `extractConfluenceRoot` | Scheme + host extraction; malformed URLs |
| `ValidateWithConfluence` | Mock HTTP server: 200 JSON, 401, 302 redirect, HTML response |

**Edge cases**

- `AsHeaders` with zero cookies → no `Cookie` header, other headers still set
- `AsHeaders` with multiple cookies → `"; "` joined
- `GetSpaceKeyFromURL`:
  - `https://company.atlassian.net/wiki/spaces/PROJ/overview` → `PROJ`
  - `https://wiki.company.com/spaces/PROJ` → `PROJ`
  - `https://confluence.company.com/display/PROJ/Home` → `PROJ`
  - URL with trailing path segments after key
  - URL with no space key → `""`
- `extractConfluenceRoot`: empty string, missing scheme, port in host
- `ValidateWithConfluence` via `httptest.Server`:
  - `200` + `application/json` + `displayName` → valid, flavor detected
  - `200` + `text/html` → invalid (SSO login wall; see `DOCS/task-validation-sso-fix.md`)
  - `302` redirect (client must not follow) → invalid
  - `401` / `403` → invalid, "session expired" message
  - All probes return `404` → invalid, "API not found"
  - Cloud probe succeeds before Server probe is tried
- Corrupt ciphertext (truncated file) on `Load` → error
- `Save` to nested path (`/dir/sub/session.enc`) → parent dir created with `0700`

---

### 3. `internal/storage`

#### `writer.go`

| Function | What to verify |
|----------|----------------|
| `sanitizeFilename` | Slash/colon/backslash replacement, trim, 100-char truncation |
| `MakePageDir` | Directory created under `basePath/spaceKey/safeTitle` |
| `SaveHTML` / `SaveRawHTML` | Files written to `index.html` / `raw.html` |
| `SaveMetadata` | Valid JSON written, round-trip unmarshals |
| `GetSavedPath` | Path construction |

**Edge cases**

- Title with `/`, `\`, `:` → sanitized
- Title longer than 100 chars → truncated
- Title with only whitespace → empty or trimmed name
- `SaveMetadata` with nil pointer fields → valid JSON
- Unicode titles in `sanitizeFilename`

#### `asset.go`

| Function | What to verify |
|----------|----------------|
| `AssetDownloader.RewriteURL` | Attachment vs image URL rewriting, hash-based filenames |
| `AssetDownloader.Download` | Mock server: 200 OK, retries on failure, skip if file exists |

**Edge cases**

- `RewriteURL`: invalid URL → returns original
- `RewriteURL`: `/download/attachments/` path → `attachments/` prefix
- `RewriteURL`: `confluence-attachments` host → `images/` prefix
- `Download`: HTTP 404/500 → retries then error
- `Download`: file already on disk → returns path without re-download
- `Download`: response with no file extension → infers from `Content-Type`
- `Download`: write failure mid-stream → temp file removed

---

### 4. `internal/scraper`

Focus on **pure logic and HTTP-mocked API paths**. Do not launch go-rod or require Chromium.

#### URL / parsing helpers (`discovery.go`, `page.go`)

| Function | What to verify |
|----------|----------------|
| `parseConfluenceID` | `/pages/12345` extraction; no match → `0` |
| `resolveURL` | Absolute href unchanged; relative href joined to base |
| `extractConfluenceBaseURL` | `scheme://host` from full URL |
| `extractTextFromHTML` | Plain text extraction, whitespace collapse, 50k truncation |
| `extractSpaceKey` | Regex from URL; fallback `"unknown"` |
| `extractSpaceName` | DOM selectors return first non-empty text |
| `parseSidebar` | Links filtered by space key, deduplication, default titles |
| `fetchPageListAPI` | Mock paginated API responses (Cloud + Server flavors) |

**Edge cases**

- `parseConfluenceID`: malformed href, multiple `/pages/` segments
- `resolveURL`: href starting with `http`, href with/without leading `/`
- `extractTextFromHTML`: empty HTML, nested tags, script tags (if present)
- `parseSidebar`: links from other spaces excluded
- `parseSidebar`: duplicate hrefs deduplicated
- `parseSidebar`: empty link text → `page-{id}` default title
- `fetchPageListAPI`:
  - Cloud vs Server URL construction
  - Paginated results (`limit=50`, multiple pages)
  - Empty `results` array → empty slice
  - Non-200 status → error
  - Malformed JSON → error
  - Relative `webui` link prefixed with `rootURL`
  - Missing `version` field → version `0`
- `extractSpaceKey`: URL with hyphenated keys (`MY-SPACE`)

#### HTML extraction (`page.go`)

| Function | What to verify |
|----------|----------------|
| `stripChrome` | Known selectors removed; scripts/styles removed |
| `rewriteInternalLinks` | `/spaces/.../page/NNN` → `#` + `data-original-href` |
| `cleanupEmptyElements` | Empty `div`/`span` removed |
| `extractContent` | End-to-end on fixture HTML (no network) |

**Edge cases**

- HTML with no chrome elements → `stripChrome` returns `0`
- Internal links that are already absolute → unchanged
- `img` with `data:` URI → skipped
- `img` with `/download/attachments/` → processed (mock downloader)
- Malformed HTML → error from goquery

#### Crawl jobs (`job.go`)

| Function | What to verify |
|----------|----------------|
| `CreateJob` | UUID assigned, status `pending` |
| `GetJob` | Found / not found |
| `ListJobs` | Snapshot counts by status |
| `CancelJob` | Pending/running cancellable; completed/failed not |
| `Cleanup` | Removes terminal jobs; keeps running/pending |

**Edge cases**

- `CancelJob` on non-existent job → error
- `CancelJob` on `completed` job → error
- `ListJobs` with mixed statuses → correct `Running`/`Completed`/etc. counts
- `RunJob` on non-pending job → error (do not run full crawl in unit test)
- Concurrent `CreateJob` / `ListJobs` under `-race`

**Deferred (integration or heavy mock):** `CrawlRunner.Run`, `discoverSpaceWeb`, `findPagesByRod`, `ScrapePage`, `LaunchBrowser` — require browser or full DB; out of scope unless refactored behind interfaces.

---

### 5. `internal/cron`

#### `manager.go` (file-backed config)

| Function | What to verify |
|----------|----------------|
| `NewManager` | Missing file → empty configs |
| `Upsert` | Insert new; update existing by `SpaceKey` |
| `Delete` | Removes entry; no-op for unknown key |
| `GetOverride` / `GetSpaceURL` | Found / not found |
| `List` | Returns copy (mutations don't affect internal state) |

**Edge cases**

- Corrupt JSON in config file → empty configs, no panic
- `Upsert` persists to disk; reload via new `Manager` sees changes
- `Delete` last entry → empty array persisted
- Concurrent `Upsert` under `-race`

#### `scheduler.go`

| Function | What to verify |
|----------|----------------|
| `sanitizeSpaceKey` | Extract key from space URL (if exported or test via same package) |

**Deferred:** `Start`, `runFullCrawl`, `runIncremental` — depend on gocron + scraper + DB; integration scope.

---

### 6. `internal/api`

Use `httptest.NewRecorder` and stub/mock dependencies.

#### Middleware

| Function | What to verify |
|----------|----------------|
| `CORSMiddleware` | `Access-Control-*` headers; `OPTIONS` → `204`, handler not called |
| `LoggingMiddleware` | Passes through; status captured (smoke test) |

**Edge cases**

- `OPTIONS` preflight does not invoke inner handler
- Non-OPTIONS methods get CORS headers and reach handler

#### Handlers (with mocked `db.DB` / `session.Store`)

| Handler | What to verify |
|---------|----------------|
| `Handler.CreateSession` | Valid body → 200; missing key → 400; bad JSON → 400 |
| `Handler.DeleteSession` | Deletes and returns success |
| `Handler.SessionStatus` | No session / valid session states |
| `Handler.ValidateSession` | Delegates to session validation |
| `SearchHandler.Search` | Missing `q` → 400; valid query → 200 JSON |
| `SearchHandler.Reindex` | POST only; `confluence_id` without `space_key` → 400 |
| `CrawlHandler.Create` | Missing `space_url` → 400; creates job |
| `CrawlHandler.Status` | Missing `job_id` → 400; unknown job → 404 |
| `CrawlHandler.Cancel` | Valid cancel / invalid state |
| `spacesHandler` | List, add (valid/invalid URL), get, delete |

**Edge cases**

- Wrong HTTP method → `405`
- `Search`: invalid `limit` (non-numeric, zero, negative) → default `10`
- `CreateSession`: empty cookies array → 400
- `CreateSession`: missing `encryption_key` in config → 500
- `CrawlHandler.Create`: duplicate concurrent crawl requests (if applicable)

**Note:** Full handler tests require extracting a `DB` interface or using a test double. Define minimal interfaces in test files or a `internal/db/mock` package as part of implementation.

---

### 7. `internal/mcp`

| Function | What to verify |
|----------|----------------|
| `processMessage` | `initialize`, `tools/list`, `ping`, unknown method |
| `handleToolsCall` | Each tool name; missing required args |
| `toolSearch` / `toolListSpaces` / `toolListSpace` / `toolGetPage` | Arg validation with mock DB |
| `sendResponse` / `sendError` | JSON-RPC 2.0 shape on channel |

**Edge cases**

- Invalid JSON body → parse error (`-32700`)
- Unknown method → `-32601`
- `tools/call` missing `name` → `-32602`
- `confluence_search` with empty `query` → error in tool result
- `confluence_get_page` with missing `page_id` → error
- `limit` passed as JSON number (float64) → converted correctly
- `notifications/initialized` → no response sent

**Deferred:** `HandleRequest` SSE streaming loop — difficult in unit tests; smoke-test via integration or refactor send logic.

---

### 8. `pkg/logging` / `pkg/logger`

| Function | What to verify |
|----------|----------------|
| `logging.Sugar.Enabled` | Nil logger → false |
| `logger.DefaultConfig` | Sensible defaults |
| `logger.NewProductionWithLevel` | Valid levels; invalid level → error |

**Edge cases**

- `Enabled()` with zero-value `Sugar` → `false` (already relied on in session tests)

---

### 9. `internal/db`

**Out of scope for pure unit tests** — all methods require PostgreSQL.

If a follow-up task adds integration tests, prioritize:

| Function | Edge cases |
|----------|------------|
| `SearchPages` | Empty query, space filter, FTS ranking |
| `UpsertPage` | `ON CONFLICT` updates version |
| `DeleteStalePages` | Only deletes pages older than `crawlStart` |
| `IndexPageContent` | `tsvector` populated |

For this task: optionally add **SQL-free** tests for any query-building helpers if extracted; otherwise skip.

---

## Suggested Implementation Order

1. **Pure functions** — `config`, `session` URL helpers, `storage.sanitizeFilename`, scraper parsers
2. **Filesystem packages** — `storage.Writer`, `cron.Manager`, extend `session.Store`
3. **HTTP-mocked logic** — `fetchPageListAPI`, `ValidateWithConfluence`, `AssetDownloader.Download`
4. **In-memory state machines** — `scraper.CrawlJobManager`, `mcp.processMessage`
5. **API handlers** — after introducing test doubles for DB/session

## Acceptance Criteria

- [x] `go test ./...` passes from `space-mosquito/`
- [x] `go test -race ./...` passes
- [x] No test requires Docker, a browser, or a live PostgreSQL instance
- [x] Each package listed above has at least one `_test.go` file (except `cmd/` and `internal/app/`, which are thin wiring)
- [x] Existing tests in `session` and `scraper` remain passing; `discovery_test.go` expanded beyond JSON fixture check
- [x] `DEVELOPMENT.md` updated with `go test ./...` instruction
- [x] GitHub Actions runs `go test -race ./...` on push/PR (`.github/workflows/go-test.yml`)

## Resolved Questions

1. **DB interface extraction** — API handler tests limited to validation/error paths only; no `Store` interface in this task.
2. **Test fixtures** — `internal/scraper/testdata/` (`confluence_page_list.json`, `page_with_chrome.html`, `sidebar.html`).
3. **CI** — `.github/workflows/go-test.yml` runs on push/PR for `space-mosquito/**` changes.

## Open Questions

_None — all resolved for this task._
