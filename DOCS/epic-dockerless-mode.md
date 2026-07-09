# Epic: Dockerless Mode (End-User Distribution)

## Objective

Make SpaceMosquito usable by end users **without Docker**. A user should be able to download a binary for their OS, run a single init command, capture a Confluence session via the existing browser extension, and crawl/search locally — with no container runtime, no separate database service, and no manual Chromium install.

Docker remains supported for developers and power users who prefer the current Compose workflow.

## Target Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  User machine                                               │
│                                                             │
│  spacemosquito (binary)          Pirate Mosquito extension  │
│    ~/.spacemosquito/                   │                    │
│      config.yaml                       │ cookies           │
│      spacemosquito.db  (SQLite+FTS5)   ▼                    │
│      session.enc              localhost:8081                │
│      saved/              (HTML + assets)                    │
│      browser/            (rod Chromium, optional)           │
│                                                             │
│  MCP clients ──► localhost:8081/mcp                         │
└─────────────────────────────────────────────────────────────┘
```

### Design principles

| Principle | Detail |
|-----------|--------|
| **API-first scraping** | Confluence REST API is the primary path; browser is fallback only |
| **Lazy Chromium** | Do not download or launch browser at startup; fetch on first fallback need |
| **Single data directory** | All state under `~/.spacemosquito/` (or portable `./data/` mode) |
| **Embedded database** | SQLite replaces PostgreSQL for default/end-user installs |
| **Docker optional** | Compose stack continues to work; Postgres path becomes `database.driver: postgres` |
| **Extension unchanged** | Still talks to `http://localhost:8081`; no extension rewrite in v1 |

## Current State vs Target

| Area | Today (Docker-centric) | Target (Dockerless) |
|------|------------------------|---------------------|
| **Distribution** | `docker compose up` | GitHub Release binaries per OS |
| **Database** | PostgreSQL + pgvector in container | SQLite + FTS5 embedded |
| **Search** | PostgreSQL `tsvector` / `ts_rank` | SQLite FTS5 / `bm25()` |
| **Config** | `./config.yaml` volume-mounted to `/app/` | `~/.spacemosquito/config.yaml` or `CONFIG_PATH` |
| **Session** | `./session.enc` → `/app/session.enc` | `~/.spacemosquito/session.enc` |
| **Saved pages** | `./saved-data` → `/app/saved` | `~/.spacemosquito/saved/` |
| **Cron overrides** | `./cron-config.json` volume | `~/.spacemosquito/cron-config.json` |
| **Migrations** | `$(pwd)/migrations` (must run from `space-mosquito/`) | Bundled in binary or resolved from executable dir |
| **Chromium** | Hardcoded `/usr/bin/chromium` in Docker image | rod auto-download → `~/.spacemosquito/browser/` |
| **Browser at startup** | `LaunchBrowser()` on every server start | Lazy launch on API fallback only |
| **Embeddings** | Schema exists (pgvector); not wired | Deferred; SQLite path uses FTS only in v1 |

## Out of Scope (This Epic)

- Publishing extension to AMO / Chrome Web Store (sideload docs only)
- OAuth / API-token auth replacing the extension
- Semantic embeddings / ONNX runtime in the binary
- Windows ARM64 support (verify before promising)
- Auto-update mechanism for binaries
- Native menu-bar / tray UI

---

## Implementation Phases

Work in order. Each phase should leave `go test ./...` green and Docker Compose still functional until Phase 6.

### Phase 0 — Safety Net (Tests Before Refactoring)

Add tests that lock down current behavior **before** structural changes. See [Recommended Tests](#recommended-tests) below.

**Deliverable:** CI runs `go test -race ./...` on every PR.

**Dependency:** Implements priorities from `DOCS/task-go-unit-tests.md`.

---

### Phase 1 — Storage Abstraction

Introduce a database interface so PostgreSQL and SQLite can coexist.

**Files / packages**

| File / area | Change |
|-------------|--------|
| `internal/db/` | Extract `Store` interface: `CreateSpace`, `GetSpaceByKey`, `ListSpaces`, `UpsertPage`, `GetPage`, `ListPages`, `SearchPages`, `GetPageStats`, `IndexPageContent`, `IndexAllPageContents`, `DeleteStalePages`, `UpdateSpaceLastCrawled` |
| `internal/db/postgres.go` | Existing implementation behind interface |
| `internal/db/sqlite.go` | New SQLite implementation |
| `internal/config/config.go` | Add `database.driver: sqlite \| postgres` (default `sqlite` for new installs) |
| `migrations/` | Duplicate or adapt schema for SQLite (no `pgvector`, no `gen_random_uuid()`, FTS5 instead of `tsvector`) |

**SQLite schema notes**

- `spaces`, `pages` tables — same logical model as Postgres
- FTS5 virtual table or `content_vector` column equivalent for search
- Drop `page_embeddings` table in SQLite v1 (unused in app today)
- UUIDs stored as `TEXT`; generated in Go
- WAL mode enabled for concurrent reads during crawl

**Edge cases to handle in implementation**

- Migration from existing Postgres data → manual export/import (document only in v1)
- `ON CONFLICT` syntax works in both drivers
- Search ranking will differ slightly between Postgres FTS and SQLite FTS5 — acceptable

---

### Phase 2 — Path & Config Normalization

Remove hardcoded Docker paths; centralize data-directory resolution.

**New helper:** `internal/paths` (or `internal/config/paths.go`)

```
ResolveDataDir()     → ~/.spacemosquito  (or SPACEMOSQUITO_DATA_DIR / --data-dir)
ResolveConfig()      → $DATA_DIR/config.yaml
ResolveSession()     → $DATA_DIR/session.enc
ResolveSaved()       → $DATA_DIR/saved/
ResolveCronConfig()  → $DATA_DIR/cron-config.json
ResolveDB()          → $DATA_DIR/spacemosquito.db
ResolveBrowser()     → $DATA_DIR/browser/
ResolveMigrations()  → embedded FS or adjacent to executable
```

**Files to update**

| File | Hardcoded path today | Fix |
|------|---------------------|-----|
| `internal/scraper/scraper.go` | `/usr/bin/chromium` | See Phase 3 |
| `internal/scraper/discovery.go` | `/app/saved/.debug.html`, `/tmp/confluence_*.png` | Use `ResolveSaved()` + temp dir |
| `internal/app/server.go` | `migrations` from CWD; `CRON_CONFIG_PATH` default `./cron-config.json` | Use path resolver |
| `cmd/cli/main.go` | Same `CONFIG_PATH` logic as server | Share resolver |
| `config.yaml.example` | Mixed relative paths | Document dockerless defaults |

**New CLI command**

```bash
spacemosquito init
  --data-dir <path>     # default ~/.spacemosquito
  --download-browser    # optional pre-fetch of Chromium
  --encryption-key <k>  # or auto-generate and print once
```

Creates data dir, writes default `config.yaml`, runs migrations, sets file permissions (`0600` on session path).

---

### Phase 3 — Chromium Launcher (Lazy + Auto-Download)

Replace hardcoded Docker Chromium with a resolution chain.

**Resolution order**

1. `CHROMIUM_PATH` env or `browser.path` in config
2. `launcher.LookPath()` — system Chrome/Chromium/Edge
3. rod auto-download via `launcher.NewBrowser().RootDir(ResolveBrowser()).Get()`

**Behavior changes**

| Current | Target |
|---------|--------|
| `LaunchBrowser()` at server startup (`server.go`) | Remove; lazy launch inside scraper fallback paths only |
| `.Bin("/usr/bin/chromium")` always | Only call `.Bin()` when path resolved |
| `NoSandbox(true)` always | Set on Linux only (containers + some distros) |
| `LEAKLESS=0` in Dockerfile | Keep for Docker; use rod default on desktop |

**Files**

- `internal/scraper/scraper.go` — `resolveChromiumBin()`, refactor `LaunchBrowser()`
- `internal/scraper/job.go`, `internal/cron/scheduler.go`, `cmd/cli/main.go` — remove eager `LaunchBrowser()` where not needed

**User-facing**

- First fallback triggers download (~150 MB); log progress
- `spacemosquito init --download-browser` for offline-first setup

---

### Phase 4 — Binary Releases

**Build matrix (v1)**

| Target | GOOS/GOARCH |
|--------|-------------|
| macOS Apple Silicon | `darwin/arm64` |
| macOS Intel | `darwin/amd64` |
| Linux x64 | `linux/amd64` |
| Windows x64 | `windows/amd64` |

**Artifacts per release**

- `spacemosquito-{os}-{arch}` (server + CLI as one binary with subcommands, or two binaries)
- `SHA256SUMS`
- `README-dockerless.md` — install, init, extension, first crawl

**Go build notes**

- Embed migrations: `//go:embed migrations/*.sql`
- SQLite driver: prefer `modernc.org/sqlite` (pure Go, no CGO) for cross-compile simplicity
- Version stamp: `-ldflags "-X main.version=..."`

**CI**

- GitHub Actions: test → build matrix → attach to Release on tag

---

### Phase 5 — Docker Coexistence & Docs

Docker becomes optional, not removed.

| Item | Action |
|------|--------|
| `docker-compose.yml` | Keep; document as "developer / Postgres mode" |
| `config.yaml.example` | Add `database.driver: postgres` example for Docker |
| `README.md` | Two quick-start paths: Dockerless (recommended for end users) and Docker (developers) |
| `DEVELOPMENT.md` | Local `go run` with SQLite |
| `ARCHITECTURE.md` | Update diagrams |
| New ADR | `014-dockerless-sqlite-distribution.md` |

---

### Phase 6 — Deprecation Cleanup (Optional, Later)

- Remove pgvector from Postgres migrations path if unused
- Simplify Dockerfile (smaller image if Chromium not needed in container — API-only in Docker too)
- Consider making SQLite the only default in examples

---

## Recommended Tests

Tests exist to catch regressions **during** the refactor. Implement in Phase 0 before Phase 1 code changes. Full inventory in `DOCS/task-go-unit-tests.md`; below is the **priority subset** for this epic.

### Tier 1 — Add First (Block Refactoring)

These tests define contracts that must not break across storage/launcher/path changes.

#### `internal/db` — contract tests via interface

| Test | Why |
|------|-----|
| `TestStore_Contract` (shared test suite) | Run same test cases against Postgres (CI service) **and** SQLite (in-memory/temp file) implementations |
| `CreateSpace` + `GetSpaceByKey` round-trip | Core identity |
| `UpsertPage` + `GetPage` with `version` | Incremental crawl depends on this |
| `UpsertPage` ON CONFLICT updates title/content/version | Idempotent re-crawl |
| `SearchPages` — known content returns expected page | **Critical:** FTS migration risk |
| `SearchPages` — empty query returns nil/empty | API contract |
| `SearchPages` — `spaceKey` filter | MCP + API filter |
| `ListPages` limit | MCP `confluence_list_space` |
| `DeleteStalePages` — only deletes old rows | Sweep logic |
| `GetPageStats` counts | `/api/search/stats` |

**Implementation pattern:**

```go
func TestStoreContract(t *testing.T) {
    for name, newStore := range map[string]func(t *testing.T) Store{
        "sqlite": newSQLiteTestStore,
        // "postgres": newPostgresTestStore,  // tag: integration
    } {
        t.Run(name, func(t *testing.T) { runStoreContractTests(t, newStore(t)) })
    }
}
```

#### `internal/scraper` — parsing & API discovery

| Test | Why |
|------|-----|
| `GetSpaceKeyFromURL` table test | URL formats must survive path changes |
| `parseConfluenceID` / `resolveURL` / `extractConfluenceBaseURL` | Discovery unaffected by DB swap |
| `fetchPageListAPI` with `httptest.Server` | Cloud + Server flavors, pagination, version parsing |
| `extractContent` / `stripChrome` / `extractTextFromHTML` on HTML fixtures | Content pipeline independent of DB |
| `savePageMetadata` with mock `Store` | Verifies scraper → DB boundary |

#### `internal/session`

| Test | Why |
|------|-----|
| Existing `session_test.go` — keep green | Session file path changes must not break crypto |
| `AsHeaders` cookie format | API scraping depends on this |
| `ValidateWithConfluence` with mock HTTP (200 JSON, 302, HTML) | SSO fix + dockerless session flow |

#### `internal/config` + paths (new)

| Test | Why |
|------|-----|
| `Load` defaults | Config shape stable across modes |
| `DatabaseConfig.DSN` vs `DATABASE_URL` | Docker env override still works |
| `ResolveDataDir` — env override, default, portable mode | **New code; test from day one** |
| `Resolve*` paths all under data dir | No stray `/app/` paths |

---

### Tier 2 — Add During Refactoring

#### `internal/scraper` — job manager

| Test | Why |
|------|-----|
| `CrawlJobManager` create/get/list/cancel/cleanup | In-memory; unaffected by DB driver |
| Incremental skip logic (version compare) with mock Store | Refactor must preserve skip behavior |

#### `internal/api` — HTTP contract tests

Use `httptest.NewRecorder` + mock `Store` / `session.Store`.

| Endpoint | Assertions |
|----------|------------|
| `GET /health` | `200 ok` |
| `POST /api/session` | valid/invalid body, missing encryption key |
| `GET /api/search?q=` | missing `q` → 400; valid → JSON shape `{query, count, results}` |
| `POST /api/crawl` | missing `space_url` → 400; creates job |
| `GET /api/crawl/status` | unknown job → 404 |
| `GET /api/spaces` | list shape stable |

**Purpose:** MCP clients and the extension depend on JSON shapes — catch breaking changes early.

#### `internal/mcp`

| Test | Why |
|------|-----|
| `processMessage` — `initialize`, `tools/list`, `ping` | MCP handshake stable |
| `handleToolsCall` — arg validation per tool | Tool schemas unchanged |
| `toolSearch` with mock Store | Search path through MCP |

#### `internal/cron`

| Test | Why |
|------|-----|
| `Manager` upsert/delete/list round-trip on temp file | Cron config path change |
| `ParseCronDuration` / `ParseMaxDuration` | Scheduler config |

#### `internal/storage`

| Test | Why |
|------|-----|
| `sanitizeFilename` | Saved path layout under new data dir |
| `Writer` round-trip (HTML, metadata) in `t.TempDir()` | Files on disk independent of DB |

---

### Tier 3 — Integration Tests (Tagged, Optional in CI)

Run with `go test -tags=integration ./...` — not required for every PR, but valuable before releases.

| Test | Setup | Validates |
|------|-------|-----------|
| `TestServerBoot_SQLite` | Temp data dir + in-process server | Full wiring: migrations, health, graceful shutdown |
| `TestCrawlFlow_APIOnly` | `httptest.Server` as fake Confluence + SQLite | Discovery → scrape → search round-trip without browser |
| `TestMCP_SearchRoundTrip` | Boot server + MCP `tools/call` | End-to-end MCP contract |
| `TestDockerCompose_Postgres` | Existing `tests/*.sh` or Go integration | Docker path not regressed |

**Fake Confluence server:** reusable `internal/testutil/confluence_mock.go` serving discovery + content JSON fixtures.

---

### Tier 4 — Launcher Tests (Phase 3)

| Test | Why |
|------|-----|
| `resolveChromiumBin` with `CHROMIUM_PATH` set | Env override |
| `resolveChromiumBin` with `LookPath` mock | System browser path |
| `LaunchBrowser` does not call download when API-only (no fallback) | Lazy launch |
| Skip test that actually downloads Chromium in CI | Use `t.Skip` unless `-tags=browser_download` |

---

### Tests Explicitly NOT Needed for This Epic

- Docker Compose startup tests (shell scripts suffice for manual/CI smoke)
- Extension TypeScript tests
- Real Confluence integration
- Real Chromium download in default CI
- pgvector / embedding search (unused)

---

### Suggested CI Layout

```yaml
# Every PR
- go test -race ./...

# Nightly or pre-release
- go test -race -tags=integration ./...
- docker compose up -d && ./tests/run_tests.sh
```

---

## Acceptance Criteria (Epic Complete)

- [ ] End user can install binary, run `init`, capture session, crawl, and search — no Docker
- [ ] All data lives under configurable single directory (default `~/.spacemosquito/`)
- [ ] SQLite is default database driver for new installs
- [ ] Search returns relevant results via FTS5 (parity with current behavior on sample corpus)
- [ ] API-first crawl works without Chromium installed
- [ ] Browser fallback auto-downloads Chromium on first need to `browser/` under data dir
- [ ] `docker compose up` still works for developers using Postgres
- [ ] `go test -race ./...` passes; contract tests run against SQLite
- [ ] README documents both install paths
- [ ] GitHub Release ships binaries for darwin/linux/windows amd64+arm64 (darwin)

---

## Open Questions

### Product & UX

Resolved:

1. **Single binary vs server+cli?** **Merge into one binary** (`spacemosquito`) with subcommands.
2. **Portable mode?** **Yes** — support portable mode as first-class (`--data-dir ./data` and equivalent env/config path).
3. **Migration path for existing Docker users?** **Add export/import tool** (in addition to recrawl option).
4. **First-run encryption key?** **Auto-generate on `init` and display once**.
5. **Extension distribution?** **Sideload only** for this epic (no store publishing).

### Technical

Resolved:

6. **SQLite driver:** use **`modernc.org/sqlite`** (pure Go).
7. **FTS5 tokenizer:** pick one "good enough" default now and **abstract tokenizer selection in code/config** so it can be replaced later with minimal changes.
8. **Migrations packaging:** **both** — embedded migrations for release binaries + file tree for dev/Docker workflows.
9. **Database interface location:** introduce **`internal/store`** package.
10. **Postgres support:** **keep Postgres for now** as optional mode.
11. **Browser download in CI:** **mock/skip in standard CI** (real browser-download coverage can run in optional nightly/pre-release checks).
12. **Windows specifics:** **follow-up** (not in v1 scope).
13. **Linux arm64:** **exclude from v1 release matrix**.

### Auth & Scraping

Resolved:

14. **API-only fallback policy:** retry browser fallback download/setup **2 extra times**, then return error and stop.
15. **SSO validation fix** (`DOCS/task-validation-sso-fix.md`): **block this epic** until that task is fixed.
16. **Incremental `detection: dom`:** keep supported; browser/Chromium is **lazy-loaded on first browser-required operation** and stored under the resolved local browser directory (data-dir relative, including portable mode).

### Process

17. **ADR before or during implementation?** Write `ADR-014` before Phase 1, or after spike?
18. **Epic sequencing vs unit tests epic?** `DOCS/task-go-unit-tests.md` Phase 0 is a hard prerequisite — confirm priority.
19. **Versioning / release cadence?** Tag `v1.0.0-dockerless` or incremental `v0.x` releases per phase?

---

## Related Documents

| Doc | Relationship |
|-----|--------------|
| `DOCS/task-dockerless-migrations.md` | SQLite/Postgres migration trees, embed, FTS5 |
| `DOCS/task-go-unit-tests.md` | Phase 0 prerequisite; full test inventory |
| `DOCS/task-validation-sso-fix.md` | Session validation must be reliable for dockerless |
| `DOCS/task-incremental-scraper.md` | Version-based skip logic must pass contract tests |
| `ADR-004` | Headless browser rationale — update for lazy/auto-download |
| `ADR-009` | Migrations — extend for SQLite |
| `README.md` | Dual quick-start after epic |

## Suggested ADR

**ADR-014: Dockerless Distribution with SQLite and Embedded Chromium**

- Status: Proposed
- Decision: Default install uses SQLite + rod auto-download; Docker/Postgres remains optional developer path
- Record after remaining open questions are resolved
