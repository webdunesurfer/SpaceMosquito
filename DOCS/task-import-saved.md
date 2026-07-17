# Task: `import_saved` Bootstrap (Rebuild SQLite from `saved/`)

## Objective

Implement **`import_saved`**: scan an on-disk `saved/` tree (from prior Docker/Postgres crawls or dockerless installs), reconstruct `spaces` and `pages` rows in SQLite, rebuild FTS, and leave the server ready for REST/MCP search without re-crawling Confluence.

This is the **fast offline migration path** for users who already have crawl artifacts when switching to dockerless SQLite. It does **not** read from PostgreSQL directly.

**Parent references:**

- `DOCS/epic-dockerless-mode.md` — migration path open question (closes with this task)
- `DOCS/task-dockerless-data-bootstrap-modes.md` — umbrella bootstrap modes (`recrawl` vs `import_saved`)
- `ADR/006-saved-page-format.md` — on-disk layout contract
- `DOCS/task-dockerless-migrations.md` — schema/migrations (prerequisite, done)

**Sibling mode (out of scope for this task):**

- **`recrawl`** — empty DB + crawl again. No importer code; only document as the default alternative.

**Out of scope (v1):**

- Direct Postgres → SQLite SQL/pg_dump transfer
- Re-downloading assets or validating asset checksums
- Reconstructing `parent_confluence_id` when not present in metadata (leave `NULL`)
- `page_embeddings` / vector data
- Cron config migration
- Browser extension changes
- Automatic run on `serve` (bootstrap is explicit CLI only)

---

## Current State

| Piece | Status |
|-------|--------|
| On-disk writer | `internal/storage/writer.go` writes `metadata.json`, `index.html`, `raw.html` per page |
| Scraper DB upsert | `savePageMetadata` in `internal/scraper/scraper.go` — reference mapping from files → `store.Page` |
| Text extraction | `extractTextFromHTML` in `internal/scraper/page.go` (unexported) |
| Confluence ID parse | `parseConfluenceID` in `internal/scraper/discovery.go` (method on `Scraper`, regex `/pages/(\d+)`) |
| Space key from URL | `session.GetSpaceKeyFromURL` |
| SQLite store + FTS | `internal/store/sqlite` — triggers maintain `pages_fts`; `IndexAllPageContents` rebuilds FTS |
| `spacemosquito init` | Runs migrations only; **no** `--bootstrap-mode` |
| Integration tests | `internal/app/server_integration_test.go` seeds DB in-process; no `saved/` fixture importer |

**Gap:** Users with a populated `saved/` directory from Docker must either recrawl or manually hack DB rows. No supported importer exists.

---

## User Story

> I ran SpaceMosquito in Docker for months. My Postgres volume and `saved/` bind-mount have thousands of pages. I want a local SQLite install without waiting for a full recrawl.

**Success:** After `spacemosquito bootstrap import-saved` (or `init --bootstrap-mode=import_saved`), `confluence_search`, `confluence_get_page`, and REST list/search endpoints return the same logical pages as before (by `space_key` + `confluence_id`), using content extracted from `index.html`.

---

## On-Disk Input Contract

Per `ADR-006`, each page is a directory:

```
saved/
  {space_key}/
    {sanitized-page-title}/
      metadata.json
      index.html          # clean HTML (primary content source)
      raw.html            # optional fallback
      assets/...          # not read by importer v1
```

### `metadata.json` shape (today)

From `internal/storage.Metadata`:

| Field | JSON key | Importer use |
|-------|----------|--------------|
| Title | `title` | `pages.title` |
| Confluence URL | `confluence_url` | Parse `confluence_id`; derive `spaces.url` root |
| Space key | `space_key` | `spaces.key`; fallback: parent path segment |
| Parent title | `parent_title` | informational only v1 (no `parent_confluence_id`) |
| Author | `author` | ignored v1 |
| Created / updated | `created_at`, `updated_at` | `pages.created_at`, `pages.updated_at` |
| Saved at | `saved_at` | ignored v1 |
| Images / attachments | `images`, `attachments` | ignored v1 |

**Important:** `confluence_id` is **not** stored in `metadata.json`. It must be parsed from `confluence_url` using the same `/pages/(\d+)` rule as the scraper.

**Path columns:** Store paths relative to data dir where possible, matching scraper output:

- `html_path` → `{page_dir}/index.html`
- `raw_html_path` → `{page_dir}/raw.html` (if file exists)
- `metadata_path` → `{page_dir}/metadata.json`
- `file_dir` → absolute or data-dir-relative page directory (match existing scraper convention)

---

## Target Behavior

### CLI surface (v1)

**Dedicated command (preferred for repeatability):**

```bash
spacemosquito bootstrap import-saved \
  --data-dir ~/.spacemosquito \
  --from ~/.spacemosquito/saved    # default: ResolveSaved()
  --force                          # required if DB already has pages
  --dry-run                        # scan + report only, no writes
```

**Optional init hook (convenience, same implementation):**

```bash
spacemosquito init --bootstrap-mode=import_saved [--from PATH] [--force]
```

Default bootstrap mode for `init` remains **`recrawl`** (no import).

Env override (optional): `SPACEMOSQUITO_BOOTSTRAP_MODE=import_saved`

### Execution flow

```
Resolve data dir + config (sqlite)
       ↓
datastore.MigrateUp
       ↓
Open store.Store
       ↓
Safety gate: if pages exist && !--force → exit 1 with message
       ↓
Walk --from tree for metadata.json
       ↓
Parse + validate each page → ImportRecord
       ↓
Dedupe by (space_key, confluence_id) → keep newest updated_at
       ↓
Batch upsert spaces (CreateSpace or skip if exists)
       ↓
Batch UpsertPage per record
       ↓
store.IndexAllPageContents (FTS rebuild)
       ↓
Write report JSON + print summary to stdout
```

`serve` must **never** trigger import implicitly.

### Safety gates

| Condition | Behavior |
|-----------|----------|
| DB has ≥1 page, no `--force` | Refuse with clear error |
| `--force` | `DELETE FROM pages` (+ FTS rebuild via reindex); keep `spaces` rows unless `--replace-spaces` (defer v2) |
| Missing `metadata.json` in dir | Skip dir (not an error) |
| Unparseable `confluence_id` | Skip row; count in report |
| `space_key` empty and not inferable from path | Skip row |
| Duplicate `(space_key, confluence_id)` | Keep row with latest `updated_at` (tie-break: latest `saved_at`, then path mtime) |

### Report output

Machine-readable file:

```text
{dataDir}/reports/bootstrap-import-{timestamp}.json
```

Suggested fields:

```json
{
  "mode": "import_saved",
  "from": "/path/to/saved",
  "started_at": "...",
  "finished_at": "...",
  "scanned_dirs": 1200,
  "imported_pages": 1180,
  "imported_spaces": 4,
  "deduplicated": 12,
  "skipped": [
    {"path": "...", "reason": "missing confluence_id in url"}
  ],
  "errors": [
    {"path": "...", "error": "upsert failed: ..."}
  ]
}
```

Human summary on stdout (counts + report path).

---

## DB Mapping

### Space row

| DB column | Source |
|-----------|--------|
| `key` | `metadata.space_key` or `saved/{key}/` segment |
| `name` | `key` if unknown (same as scraper auto-create) |
| `url` | `extractConfluenceBaseURL(metadata.confluence_url)` + `/spaces/{key}` or stored root if parseable |
| `last_crawled` | max `metadata.updated_at` among imported pages in space (optional v1) |

Use `store.CreateSpace` when space missing; no update if space already exists (v1).

### Page row

| DB column | Source |
|-----------|--------|
| `confluence_id` | parse `metadata.confluence_url` |
| `title` | `metadata.title` |
| `version` | `0` (not in metadata today) |
| `parent_confluence_id` | `NULL` v1 |
| `content` | `extractTextFromHTML(index.html)`; if empty, try `raw.html` |
| `html_path`, `raw_html_path`, `metadata_path`, `file_dir` | from walk paths |
| `created_at`, `updated_at` | from metadata; fallback `saved_at` or file mtime |

Use existing `store.UpsertPage` (same as scraper).

### FTS

After all upserts:

```go
db.IndexAllPageContents(ctx)
```

For SQLite this runs FTS rebuild (`INSERT INTO pages_fts(pages_fts) VALUES('rebuild')`). Triggers should also have fired per upsert; rebuild is the consistency guarantee.

---

## Implementation Plan

### Phase 1 — Shared parsers (small refactor)

Avoid duplicating scraper logic.

| Change | Location |
|--------|----------|
| Export `ParseConfluenceID(url string) int` | `internal/scraper` or new `internal/confluence/ids.go` |
| Export `ExtractTextFromHTML(html string) string` | `internal/scraper/page.go` (rename to exported) |
| Reuse `session.GetSpaceKeyFromURL`, `extractConfluenceBaseURL` | existing |

Keep scraper call sites working via thin wrappers if needed.

### Phase 2 — `internal/bootstrap` package

```
internal/bootstrap/
  mode.go          # ModeImportSaved, validation
  import_saved.go  # orchestration
  scanner.go       # walk saved/, yield candidate dirs
  parse.go         # metadata.json → ImportRecord
  dedupe.go        # (space_key, confluence_id) merge
  apply.go         # store writes in batches
  report.go        # Report struct + JSON write
  safety.go        # existing DB checks, --force wipe
```

Core API:

```go
type Options struct {
    DataDir     string
    FromDir     string
    Force       bool
    DryRun      bool
    ReportDir   string // default {dataDir}/reports
}

type Report struct { /* fields above */ }

func ImportSaved(ctx context.Context, db store.Store, opts Options, log logging.Sugar) (Report, error)
```

### Phase 3 — CLI wiring

| File | Change |
|------|--------|
| `internal/cliapp/run.go` | Add `bootstrap` subcommand with `import-saved` |
| `internal/cliapp/run.go` | `init --bootstrap-mode=import_saved` calls same `bootstrap.ImportSaved` after migrate |
| `internal/config/config.go` | Optional `bootstrap.mode` field (informational only) |
| `internal/paths/paths.go` | `ResolveReports()` → `{dataDir}/reports` |

Print usage in `printUsage()`.

### Phase 4 — Docs

| Doc | Update |
|-----|--------|
| `README.md` | Coming from Docker / `import_saved` |
| `DOCS/task-dockerless-data-bootstrap-modes.md` | Link to this task; mark `import_saved` spec as canonical |
| `DEVELOPMENT.md` | How to run importer locally against fixture tree |

### Phase 5 — Verification

Manual smoke:

```bash
# Copy saved/ from Docker volume into ~/.spacemosquito/saved
spacemosquito bootstrap import-saved --data-dir ~/.spacemosquito
spacemosquito serve
# curl /api/search?q=... and MCP confluence_search
```

Optional: extend `internal/app` integration test with a miniature `testdata/saved/` fixture imported before HTTP assertions (follow-up; not required for v1 acceptance).

---

## Tests

All in `internal/bootstrap/` (no `integration` tag required).

| Test | Asserts |
|------|---------|
| `TestParseMetadata_happyPath` | metadata + paths → `ImportRecord` |
| `TestParseMetadata_missingConfluenceID` | skip reason |
| `TestParseMetadata_spaceKeyFromPath` | fallback when `space_key` empty |
| `TestDedupe_keepsNewestUpdatedAt` | duplicate merge |
| `TestImportSaved_happyPath` | temp `saved/` fixture → pages in SQLite |
| `TestImportSaved_searchWorks` | after import, `SearchPages` returns seeded term |
| `TestImportSaved_existingDB_requiresForce` | safety gate |
| `TestImportSaved_forceReplacesPages` | `--force` clears and reimports |
| `TestImportSaved_dryRun_noWrites` | report only, zero pages |
| `TestImportSaved_missingIndexHTML` | falls back to raw or skips with reason |

**Fixtures:** `internal/bootstrap/testdata/saved/{SPACE}/{Title}/` with minimal `metadata.json` + `index.html` (2–3 pages, one duplicate dir for dedupe test).

Reuse patterns from `internal/testutil/seed.go` for search assertions.

---

## Acceptance Criteria

- [ ] `spacemosquito bootstrap import-saved` scans `saved/`, upserts spaces/pages, rebuilds FTS
- [ ] Default `--from` is `paths.ResolveSaved()` for configured data dir
- [ ] Import refuses non-empty DB without `--force`
- [ ] `--dry-run` produces report without DB writes
- [ ] Report JSON written under `{dataDir}/reports/`
- [ ] After import, REST `/api/search` and MCP `confluence_search` / `confluence_get_page` work for imported pages
- [ ] `init --bootstrap-mode=import_saved` delegates to same code path
- [ ] `go test -race ./...` passes (bootstrap package tests included)
- [ ] README/dockerless docs describe Docker → SQLite migration via `saved/`

---

## Design Decisions (resolved defaults)

| Question | Decision |
|----------|----------|
| Content source | **`index.html`** via `ExtractTextFromHTML`; fallback **`raw.html`** if index empty/missing |
| Duplicate rows | Keep newest **`metadata.updated_at`**; tie-break **`saved_at`**, then dir mtime |
| Missing space in DB | **Auto-create** space (same as scraper) |
| `parent_confluence_id` | **NULL** v1 (`parent_title` in metadata is not reliable) |
| `version` | **0** when unknown |
| Wipe scope for `--force` | **Pages only** (+ FTS rebuild); spaces preserved v1 |
| Postgres data | **Not read**; user copies `saved/` mount only |

---

## Open Questions (defer unless blocking)

1. Should importer update `spaces.last_crawled` from max page `updated_at`? **Recommend yes** — helps list-spaces UI.
2. Should paths in DB be absolute or relative to data dir? **Match scraper output today** (audit `savePageMetadata` and stay consistent).
3. Add `bootstrap import-saved --space TST` filter for partial import? **Defer v2.**
4. Progress bar for large trees (10k+ pages)? **Defer**; log every N pages in v1.
5. Should `init` run import automatically when `saved/` non-empty? **No** — explicit mode flag only.

---

## Related Files (implementation touch list)

| File | Role |
|------|------|
| `internal/storage/writer.go` | `Metadata` struct — importer input schema |
| `internal/scraper/scraper.go` | `savePageMetadata` — reference DB mapping |
| `internal/scraper/page.go` | `extractTextFromHTML` — export for reuse |
| `internal/scraper/discovery.go` | `parseConfluenceID` — extract to shared helper |
| `internal/session/session.go` | `GetSpaceKeyFromURL` |
| `internal/store/store.go` | `CreateSpace`, `UpsertPage`, `IndexAllPageContents` |
| `internal/cliapp/run.go` | CLI commands |
| `internal/paths/paths.go` | `ResolveSaved`, reports dir |

---

## Estimated Effort

| Phase | Size |
|-------|------|
| Parsers refactor | S |
| `internal/bootstrap` core | M |
| CLI + init flag | S |
| Tests + fixtures | M |
| Docs | S |

**Total:** ~1–2 days for a focused implementation.
