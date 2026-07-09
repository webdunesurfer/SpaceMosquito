# Task: Dockerless Data Bootstrap Modes (`recrawl` vs `import_saved`)

## Objective

After initial dockerless implementation lands (SQLite + embedded migrations), support **two bootstrap paths** for existing users moving from Docker/Postgres:

1. **`recrawl`** — start with an empty SQLite DB and crawl spaces again.
2. **`import_saved`** — rebuild SQLite from local `saved/` files, then reindex.

This gives users a safe default (`recrawl`) plus a faster offline-ish path (`import_saved`) when historical crawl artifacts already exist.

**Parent docs:**

- `DOCS/epic-dockerless-mode.md`
- `DOCS/task-dockerless-migrations.md`

**Out of scope:**

- Direct Postgres-to-SQLite SQL transfer tool
- Browser extension changes
- Semantic/vector migration (`page_embeddings`)

---

## Problem Summary

Current dockerless planning leaves migration path unresolved:

- `epic-dockerless-mode.md` calls out migration as an open question.
- `task-dockerless-migrations.md` explicitly excludes export/import.

Users currently have two imperfect options:

| Option | Downside |
|--------|----------|
| Fresh recrawl | Slow, requires Confluence access/session, may hit rate limits |
| Manual import ad-hoc | Not standardized, risk of malformed catalog/index |

We need a **supported, deterministic bootstrap flow** for both.

---

## Target Behavior

### UX

Bootstrap mode is chosen at initialization/migration time:

```bash
spacemosquito init --bootstrap-mode=recrawl
spacemosquito init --bootstrap-mode=import_saved
```

Defaults:

- `bootstrap-mode=recrawl` (safe default)

Optional config persistence (for audit/visibility):

```yaml
bootstrap:
  mode: recrawl # recrawl | import_saved
```

`serve` should **not** re-run bootstrap implicitly. Bootstrap is one-time (or explicit command).

### `recrawl` mode

- Apply migrations
- Leave DB empty (except schema)
- User triggers crawl jobs (manual or cron)

### `import_saved` mode

- Apply migrations
- Scan `saved/` tree
- Reconstruct `spaces` and `pages` rows
- Rebuild FTS index (`IndexAllPageContents` or SQLite FTS trigger-based path)
- Print import report: imported/skipped/errors

---

## Data Mapping for `import_saved`

Use file artifacts as source:

- `saved/{space_key}/{page_dir}/metadata.json`
- `saved/{space_key}/{page_dir}/index.html` and/or extracted text

### Required fields

| DB field | Source |
|----------|--------|
| `space.key` | `metadata.space_key` (fallback: path segment) |
| `space.name` | derive from URL or set to key if unknown |
| `space.url` | derive from `confluence_url` root and space key |
| `pages.confluence_id` | parse from `metadata.confluence_url` (`/pages/{id}`) |
| `pages.title` | `metadata.title` |
| `pages.content` | extracted text from HTML or stored text field |
| `pages.parent_confluence_id` | optional parse from metadata/URL if available |
| `pages.created_at` | metadata `created_at` (fallback now) |
| `pages.updated_at` | metadata `updated_at` (fallback now) |
| `pages.version` | default `0` (unless discoverable) |
| file path columns | built from actual file paths |

### Validation rules

- Skip rows without parseable `confluence_id`.
- Skip rows missing `space_key` and no inferable path key.
- De-duplicate by `(space_key, confluence_id)`; keep latest `updated_at`.
- Record every skip/error in report output.

---

## Design Decisions

### 1. Mode switch location

Prefer **CLI flag/env** for one-time operation:

- `--bootstrap-mode` (primary)
- `SPACEMOSQUITO_BOOTSTRAP_MODE` (optional)

Keep runtime config minimal; writing chosen mode into config is informational only.

### 2. Explicit command boundaries

- `init` may run bootstrap once.
- Add optional dedicated command for repeatable operation:

```bash
spacemosquito bootstrap --mode=import_saved --from=/path/to/saved
```

This avoids accidental destructive behavior on normal `serve`.

### 3. Safety on existing DB

If DB already has pages:

- default: refuse and ask for `--force` (or `--replace`)
- with `--replace`: clear `pages` + FTS tables, keep spaces unless requested

### 4. Deterministic import over best-effort magic

Importer should be strict and verbose; no silent assumptions.

---

## Implementation Plan

### Phase 1 — CLI/Config plumbing

| Area | Change |
|------|--------|
| `cmd/cli/main.go` | Add `--bootstrap-mode` to `init` |
| `internal/config` | Optional `bootstrap.mode` field |
| Validation | Mode enum: `recrawl`, `import_saved` |

### Phase 2 — Bootstrap service

Add `internal/bootstrap` package:

```go
type Mode string
const (
  ModeRecrawl Mode = "recrawl"
  ModeImportSaved Mode = "import_saved"
)

func Run(ctx context.Context, mode Mode, opts Options) (Report, error)
```

Responsibilities:

- Ensure migrations up
- Dispatch by mode
- Return structured report

### Phase 3 — Saved importer

Add importer modules:

- `scanner.go` (walk files)
- `parse_metadata.go` (decode and validate metadata)
- `mapper.go` (map to DB models)
- `apply.go` (upsert with transactions/batches)
- `report.go`

### Phase 4 — Reindex/FTS integration

- Trigger `IndexAllPageContents` post-import
- For SQLite trigger-based FTS, verify index consistency
- Emit final stats (spaces/pages/indexed)

### Phase 5 — Docs

Update:

- `DOCS/epic-dockerless-mode.md` (close open question)
- `README.md` dockerless migration section
- new troubleshooting section for import skips

---

## Failure & Recovery Behavior

### `recrawl`

- Fail fast only on migration/DB init issues.

### `import_saved`

- Partial file errors do not abort full run by default.
- Abort only on critical DB write failures.
- Emit machine-readable report file:

```text
{dataDir}/reports/bootstrap-import-<timestamp>.json
```

Report fields:

- scanned files
- imported rows
- deduplicated rows
- skipped rows (with reason)
- hard errors

---

## Tests

| Test | Scope |
|------|-------|
| `TestBootstrapModeValidation` | invalid mode rejected |
| `TestBootstrapRecrawl_NoDataInserted` | migrations run, zero pages |
| `TestImportSaved_HappyPath` | fixtures -> spaces/pages imported |
| `TestImportSaved_DedupByConfluenceID` | latest row kept |
| `TestImportSaved_MissingConfluenceID_Skipped` | skip reason recorded |
| `TestImportSaved_ReindexCalled` | search works post-import |
| `TestImportSaved_ExistingDB_RequiresForce` | safety gate works |

Use fixture tree under `internal/bootstrap/testdata/saved/...`.

---

## Acceptance Criteria

- [ ] `init` supports `--bootstrap-mode=recrawl|import_saved`
- [ ] Default mode is `recrawl`
- [ ] `import_saved` can rebuild SQLite catalog from `saved/`
- [ ] Imported DB supports `confluence_search`, `confluence_get_page`, list-space endpoints
- [ ] FTS is rebuilt/available after import
- [ ] Import emits summary + detailed report file
- [ ] Existing-populated DB requires explicit override (`--force`/`--replace`)
- [ ] `go test -race ./...` passes

---

## Open Questions

1. Should `import_saved` read `index.html` and extract text, or trust a text field if present in metadata?
2. For duplicate `(space_key, confluence_id)` rows: newest `updated_at` vs newest file mtime?
3. Should import create missing spaces automatically if metadata has only page URLs?
4. Do we need `bootstrap --dry-run` in v1?
5. Should `--replace` wipe only page tables or full DB?

---

## Recommended Default

- Ship both modes.
- Keep **`recrawl` default**.
- Position `import_saved` as advanced migration accelerator for users with substantial local archives.

