# Task: Dockerless Database Migrations (SQLite + Embedded)

## Objective

Introduce a migration system that works for **dockerless end-user installs** (embedded SQLite, migrations bundled in the binary) while preserving the existing **Docker/PostgreSQL** path.

Users should never need a `migrations/` folder on disk or a separate database service. Running `spacemosquito init` or `spacemosquito serve` must create or upgrade `~/.spacemosquito/spacemosquito.db` automatically.

**Parent epic:** `DOCS/epic-dockerless-mode.md` (Phase 1 + path resolution overlap)

**Out of scope:**

- Export/import from Postgres → SQLite (separate task)
- `page_embeddings` / pgvector on SQLite (deferred)
- Changing migration version numbers of already-deployed Postgres installs

---

## Current State

| Item | Today |
|------|-------|
| Tool | `golang-migrate/migrate/v4` (ADR-009) |
| Location | `space-mosquito/migrations/*.sql` (flat, Postgres-only) |
| Driver | Postgres only (`file://` source + postgres DSN) |
| Resolution | `$(cwd)/migrations` — breaks when binary runs outside repo |
| Auto-run | `server` startup + `cli init` |
| Latest version | `007_page_version` |

**Postgres-specific SQL in use:**

- `CREATE EXTENSION vector`
- `gen_random_uuid()`
- `tsvector` generated column + GIN index (`004_fts`)
- `plainto_tsquery` / `ts_rank` in application queries (`internal/db/models.go`)

---

## Target State

```
space-mosquito/migrations/
  postgres/
    001_initial.up.sql
    001_initial.down.sql
    ...
    007_page_version.up.sql
  sqlite/
    001_initial.up.sql
    001_initial.down.sql
    ...
    007_page_version.up.sql
```

| Mode | Migration source | Database |
|------|------------------|----------|
| **Release binary** | `//go:embed` → `iofs` source | SQLite file in data dir |
| **Local dev (`go run`)** | `file://./migrations/sqlite` (or embed) | SQLite or Postgres |
| **Docker Compose** | `file://` in image (`/app/migrations/postgres`) | Postgres |

**Version numbers stay aligned:** migration `00N` in both trees represents the same logical schema change.

---

## Design Decisions

### 1. Keep golang-migrate

Do not replace the tool. Extend it with:

- `github.com/golang-migrate/migrate/v4/database/sqlite` (with `modernc.org/sqlite` — pure Go, no CGO)
- `github.com/golang-migrate/migrate/v4/source/iofs` for embedded SQL

### 2. Two SQL trees, not conditional SQL in one file

Postgres and SQLite syntax diverges too much for a single migration file. Duplicate version numbers with driver-specific SQL.

### 3. Embed SQLite migrations; file-mount Postgres in Docker

- **SQLite (dockerless):** embed at build time — users have no `migrations/` directory
- **Postgres (Docker):** copy `migrations/postgres/` into image (same as today, after restructure)

Dev override: env `SPACEMOSQUITO_MIGRATIONS_DIR` points at `file://` tree for iterating on SQL without rebuild.

### 4. Auto-migrate on `init` and `serve`

No manual migration step for end users. `migrate.ErrNoChange` is success.

### 5. FTS5 replaces Postgres `tsvector` on SQLite

Search index lives in SQLite FTS5 (migration `004`). Application `SearchPages` branches on driver or hides behind `Store` interface.

---

## SQLite Schema Mapping

### `001_initial` (SQLite)

| Postgres | SQLite |
|----------|--------|
| `CREATE EXTENSION vector` | omit |
| `UUID DEFAULT gen_random_uuid()` | `TEXT PRIMARY KEY` — UUID generated in Go on insert |
| `TIMESTAMP WITH TIME ZONE` | `TEXT` (RFC3339) or `INTEGER` (Unix ms) — pick one, document in ADR |
| `page_embeddings` table + ivfflat index | omit in v1 |
| `idx_pages_title` GIN tsvector | omit — replaced by FTS5 in `004` |
| FK `pages.space_id → spaces.id` | keep (`REFERENCES spaces(id) ON DELETE CASCADE`) |

Suggested SQLite `001_initial.up.sql` sketch:

```sql
CREATE TABLE spaces (
    id TEXT PRIMARY KEY,
    key TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    last_crawled TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE pages (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    confluence_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    parent_confluence_id INTEGER,
    content TEXT,
    html_path TEXT,
    raw_html_path TEXT,
    metadata_path TEXT,
    file_dir TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(space_id, confluence_id)
);

CREATE INDEX idx_pages_space ON pages(space_id);
CREATE INDEX idx_pages_parent ON pages(parent_confluence_id);
```

### `002_placeholder` / `003_placeholder`

Keep as no-op placeholders in SQLite tree (same version alignment) or collapse into comments. Do not skip version numbers.

### `004_fts` (SQLite) — FTS5

Postgres uses a stored generated `tsvector` column. SQLite uses an FTS5 virtual table + triggers:

```sql
CREATE VIRTUAL TABLE pages_fts USING fts5(
    title,
    content,
    content='pages',
    content_rowid='rowid',
    tokenize='porter unicode61'
);

-- Triggers: pages_ai, pages_ad, pages_au
-- (insert / delete / update sync pages_fts with pages)
```

**Tokenizer:** `porter unicode61` approximates Postgres `english` config. Document that ranking will not be identical.

**Alternative (simpler, worse incremental):** no triggers; `IndexPageContent` / `IndexAllPageContents` rebuild FTS rows in Go. Matches existing reindex CLI. Prefer triggers for automatic sync on `UpsertPage`.

### `006_list_pages`

Postgres adds `idx_pages_space_id`. SQLite `001` already has `idx_pages_space`. SQLite `006` can be no-op with comment, or add redundant index if name parity matters for ops.

### `007_page_version`

Same in both drivers:

```sql
ALTER TABLE pages ADD COLUMN version INTEGER NOT NULL DEFAULT 0;
```

SQLite supports `ADD COLUMN` (no `IF NOT EXISTS` on column — use golang-migrate version tracking to run once).

---

## Implementation Sequence

### 1. Restructure existing migrations

- Move current `migrations/*.sql` → `migrations/postgres/`
- Update Dockerfile: `COPY ... /app/migrations/postgres`
- Update `docker compose exec app /app/cli init` docs if path changes
- Verify existing Postgres installs: version numbers unchanged → `migrate up` is no-op

### 2. Add SQLite migration files

- Create `migrations/sqlite/001` through `007` per mapping above
- Add `008` only when both drivers need a new change going forward

### 3. Config changes

**File:** `internal/config/config.go`

```yaml
database:
  driver: sqlite          # sqlite | postgres
  path: spacemosquito.db  # sqlite only; resolved under data dir
  host: localhost         # postgres only (existing fields)
  port: 5432
  ...
```

- Default `driver: sqlite` for new `config.yaml` from `init`
- Docker `config.yaml` sets `driver: postgres`

### 4. Migration resolver package

**New file:** `internal/db/migrate.go` (or `internal/migrate/`)

```go
func RunUp(ctx context.Context, cfg *config.Config, dataDir string, log logging.Sugar) error
func RunDown(ctx context.Context, cfg *config.Config, dataDir string, log logging.Sugar) error
func Status(cfg *config.Config, dataDir string) (version uint, dirty bool, err error)
```

**Resolution logic:**

```
if SPACEMOSQUITO_MIGRATIONS_DIR set → file:// that path (dev)
else if driver == sqlite            → iofs from embed.FS
else if driver == postgres          → file:// adjacent to binary or /app/migrations/postgres in Docker
```

**DSN construction:**

| Driver | DSN |
|--------|-----|
| sqlite | `sqlite://` + absolute path to `{dataDir}/spacemosquito.db` |
| postgres | existing `postgres://...` (unchanged) |

Enable SQLite pragmas after open (in `db.New`, not migration SQL):

```sql
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;
```

### 5. Embed migrations

**File:** `internal/db/embed.go`

```go
//go:embed migrations/sqlite/*.sql
var sqliteMigrations embed.FS
```

Build tag optional: always embed sqlite tree in release binary.

### 6. Wire callers

| Caller | Change |
|--------|--------|
| `internal/app/server.go` | Replace CWD `migrations` with `db.RunUp(cfg, dataDir, log)` |
| `cmd/cli/main.go` `init` | Create data dir → write config → `RunUp` |
| `cmd/cli/main.go` `migrate-down` | `RunDown` (dev only; warn in docs) |
| New `migrate status` subcommand | Print version + dirty flag |

### 7. Update search layer

**File:** `internal/db/models.go` (or `sqlite.go` / `postgres.go` split)

- Postgres `SearchPages`: keep `plainto_tsquery` / `ts_rank`
- SQLite `SearchPages`: FTS5 `JOIN pages_fts ... WHERE pages_fts MATCH ? ORDER BY bm25(pages_fts)`

Also update:

- `IndexPageContent` / `IndexAllPageContents` — Postgres updates `content_vector`; SQLite re-inserts FTS row or no-op if triggers handle it

### 8. UUID generation

SQLite has no `gen_random_uuid()`. Generate in Go before `INSERT`:

```go
id := uuid.New().String()
```

Apply in `CreateSpace`, `UpsertPage`, and any other insert paths.

---

## When Migrations Run

| Command | Behavior |
|---------|----------|
| `spacemosquito init` | Create `~/.spacemosquito/`, default config, `RunUp` |
| `spacemosquito serve` | `RunUp` before HTTP listener (current behavior) |
| `spacemosquito migrate status` | Show version; exit 1 if dirty |
| `spacemosquito migrate-down` | Roll back one step — **document as dev-only** |

**Dirty state:** if migration fails mid-flight, golang-migrate sets `dirty=1`. `migrate status` reports it; document manual fix (`migrate force VERSION` or delete DB for end users).

---

## Docker Path (No Regression)

| Item | Value |
|------|-------|
| Image | `COPY migrations/postgres /app/migrations/postgres` |
| Env | `database.driver: postgres` in mounted `config.yaml` |
| Init | `docker compose exec app /app/cli init` |
| DSN | `DATABASE_URL` override still works |

Postgres migration files remain the source of truth for Docker until both trees are updated in lockstep for new versions.

---

## Tests

Add before or alongside implementation (see `DOCS/task-go-unit-tests.md` Tier 1).

| Test | File | Asserts |
|------|------|---------|
| `TestMigrateUp_SQLite_Fresh` | `internal/db/migrate_test.go` | Empty dir → `RunUp` → `schema_migrations.version == 7` |
| `TestMigrateUp_SQLite_Idempotent` | same | Second `RunUp` → no error (`ErrNoChange`) |
| `TestMigrateUp_SQLite_AllTablesExist` | same | `spaces`, `pages`, `pages_fts` present |
| `TestMigrateUp_SQLite_FTS` | same | Insert page → searchable via `SearchPages` |
| `TestMigrateUp_SQLite_007_VersionColumn` | same | `pages.version` exists, default 0 |
| `TestStore_Contract_SQLite` | `internal/db/contract_test.go` | Full store contract after migrate |
| `TestMigrateUp_Postgres` | tag `integration` | CI Postgres service; same version target |

**Fixtures:** use `t.TempDir()` + file source pointing at `migrations/sqlite/` (no embed required in tests).

---

## Acceptance Criteria

- [ ] `migrations/postgres/` contains all existing migrations (renamed path only; versions unchanged)
- [ ] `migrations/sqlite/` exists with versions `001`–`007` aligned to Postgres
- [ ] `spacemosquito init` creates SQLite DB and applies all migrations
- [ ] `spacemosquito serve` auto-migrates on startup
- [ ] Release binary runs migrations without `migrations/` on disk (embed)
- [ ] `go run` from `space-mosquito/` can use file source via env or default dev path
- [ ] Docker `cli init` + `serve` still work against Postgres
- [ ] `SearchPages` works on SQLite after crawl fixture data
- [ ] `migrate status` reports current version
- [ ] Migration tests pass in CI (`go test ./internal/db/...`)

---

## Open Questions

1. **Timestamp type in SQLite** — `TEXT` (ISO8601) vs `INTEGER` (Unix)? Affects sorting and `DeleteStalePages` comparisons.
2. **FTS5 sync** — triggers in SQL vs explicit reindex in Go on `UpsertPage`? Triggers are less code in app; reindex matches existing `reindex` CLI mental model.
3. **Tokenizer** — `porter unicode61` vs `unicode61` only? Need a small corpus test for search parity.
4. **`006` on SQLite** — no-op migration vs skip version number? Prefer no-op to keep version integers aligned.
5. **Dirty migration recovery for end users** — document "delete `spacemosquito.db` and re-crawl" vs ship `migrate force`?
6. **Single embed for both drivers?** — embed both `postgres/` and `sqlite/` trees, or only sqlite (Postgres uses files in Docker)?
7. **Future `008+` changes** — require both `postgres/008_*.sql` and `sqlite/008_*.sql` in same PR (CI check)?

---

## Related Documents

| Doc | Relationship |
|-----|--------------|
| `DOCS/epic-dockerless-mode.md` | Parent epic; Phase 1 storage abstraction |
| `DOCS/task-go-unit-tests.md` | Contract + migration tests |
| `ADR-009` | Update: dual-driver migration trees + embed |
| `ADR-014` | Dockerless distribution decision — **Accepted** |

## Suggested ADR Update (ADR-009 addendum)

- Migration files split by driver under `migrations/{postgres,sqlite}/`
- SQLite migrations embedded in binary via `go:embed` + `iofs`
- Version numbers synchronized across drivers; SQL may differ
- FTS: Postgres `tsvector` / SQLite FTS5 — application abstracts search
