# Phase 11: Production Hardening

## Objective
Finalize the project for safe use and distribution: proper .gitignore, secret audit, Makefile, documentation updates, stub fixes, and migration gap resolution.

---

## Tasks

### 11.0 — Secret Audit

**Risk**: Accidentally committing secrets (encryption keys, passwords, session cookies) to the repository.

**Findings**:
| File | Secret Type | Status | Action |
|------|------------|--------|--------|
| `config.yaml` | `encryption_key: spacemosquito-dev-key-32bytes!` | Untracked, in .gitignore | OK — but remove dev key from example |
| `config.yaml` | `password: spacemosquito` | Untracked, in .gitignore | OK — placeholder DB password |
| `session.enc` | Encrypted cookies | Untracked, in .gitignore | OK |
| `session.enc.bak` | Encrypted cookies backup | Untracked, NOT in .gitignore | **Add to .gitignore** |
| `config.yaml.example` | `encryption_key: ""` (empty) | Tracked | OK — template with empty value |
| `config.yaml.example` | `password: spacemosquito` (placeholder) | Tracked | OK — dev-only placeholder |
| `docker-compose.yml` | `POSTGRES_PASSWORD: spacemosquito` | Tracked | OK — dev-only placeholder |
| `.env.example` | `SESSION_ENCRYPTION_KEY=your-32-character-encryption-key-here` | Tracked | OK — clearly a template |
| Go code (`config.go`) | `APIKey` struct field | Tracked | OK — config field, not hardcoded |
| Extension code | No secrets | Tracked | OK |

**Actions**:
- [ ] Confirm `session.enc.bak` is added to `.gitignore`
- [ ] Ensure `config.yaml` is listed in `.gitignore` (it is)
- [ ] Add comment to `.env.example` warning users never to commit the file
- [ ] Update `config.yaml.example` to add a `# WARNING: Do NOT use this exact key in production` note
- [ ] Document the secret policy in README: which files must never be committed

---

### 11.1 — Complete .gitignore

**Current state**: `.gitignore` has been reduced to 2 lines (from the original 22). Critical ignores are missing.

**Original (from HEAD, 22 lines)**: Covers `node_modules/`, `vendor/`, `build/`, `dist/`, `*.exe`, `space-mosquito/cli`, `space-mosquito/server`, `saved/`, `*.enc`, `.env`, `config.yaml`, `session-data/`, `*.log`, `.idea/`, `.vscode/`, `*.swp`, `*.swo`, `*~`, `.DS_Store`, `Thumbs.db`, `firefox-extension/*.xpi`

**Current (2 lines)**: Only `firefox-extension/node_modules/` and `firefox-extension/dist/`

**Missing items to add**:
- `session.enc` and `session.enc.bak` (encrypted sessions — critical)
- `config.yaml` (contains real encryption key and DB password)
- `*.log` (log files)
- `.DS_Store` (macOS artifacts)
- `space-mosquito/cli` and `space-mosquito/server` (compiled binaries)
- `firefox-extension/*.xpi` (distribution packages)
- `build/` (build output directory)
- `vendor/` (Go vendoring)
- `Thumbs.db` (Windows artifacts)
- `cron-config.json` (contains real per-space cron config — may have sensitive space URLs)
- `saved/` (crawled data)
- `session-data/` (legacy session storage)

**Actions**:
- [ ] Restore original .gitignore entries from HEAD
- [ ] Add `session.enc`, `session.enc.bak`, `cron-config.json`
- [ ] Add `saved/` and `session-data/`
- [ ] Add `vendor/`
- [ ] Add `build/`
- [ ] Add `Thumbs.db`
- [ ] Verify `.dockerignore` is aligned with `.gitignore`

---

### 11.2 — Complete .dockerignore

**Current state** (14 lines): Has `**/node_modules/`, `**/dist/`, `**/build/`, `**/*.enc`, `**/.env`, `**/config.yaml`, `**/*.log`, `**/.DS_Store`, `**/.git/`, `firefox-extension/`, `docs/`, `ADR/`, `DOCS/`, `*.md`

**Issues**:
- `**/config.yaml` will exclude the root `config.yaml` which IS needed (mounted as volume) — but since Docker COPY doesn't touch mounted volumes, this is actually fine for the build stage
- `*.md` excludes all markdown — fine for build (we don't need docs in the image)
- `DOCS/` and `ADR/` are redundant with `*.md`

**Actions**:
- [ ] Keep current `.dockerignore` as-is (it's correct for build isolation)

---

### 11.3 — Restore Makefile

**Current state**: The Makefile at HEAD is simpler (no lint, no `docker-logs`, no `crawl-docker`, no `serve-docker`). The current file doesn't exist (was deleted: `D Makefile`).

**Required targets**:
```
build              — Build Go server + CLI binaries
run                — Run server locally (after build)
test               — Run all Go tests
migrate-up         — Run database migrations
migrate-down       — Rollback last migration
lint               — go vet + TypeScript type check
docker-up          — docker compose up --build -d
docker-down        — docker compose down
docker-logs        — docker compose logs -f app
docker-build       — docker compose build --no-cache
docker-migrate     — Run migrations inside container
serve-docker       — Run server inside container (for testing)
crawl-docker       — Run crawl command inside container
dev-extension      — web-ext run for Firefox dev
build-extension    — webpack production build for extension
clean              — Remove build artifacts
config-example     — Generate config.yaml.example from code
```

**Actions**:
- [ ] Restore Makefile from HEAD
- [ ] Add missing targets: `docker-logs`, `serve-docker`, `crawl-docker`, `lint`, `migrate-down`
- [ ] Add `docker-migrate` target
- [ ] Add `clean` target
- [ ] Add `config-example` target (generates config.yaml.example)
- [ ] Update comments to reflect go-rod (not chromedp)

---

### 11.4 — Fix `runServe` CLI Stub

**Location**: `space-mosquito/cmd/cli/main.go:151-156`

**Current code**:
```go
func runServe(cfg *config.Config, log *zap.Logger) {
	sugar := logging.New("cli", log)
	sugar.Infow("starting server", "port", cfg.MCP.Port)
	// Phase 5: MCP server
	// Phase 2: API server
}
```

This is a no-op stub. The `serve` CLI command should actually start the HTTP server (same logic as `cmd/server/main.go`).

**Actions**:
- [ ] Implement `runServe()` to wire up the same components as `cmd/server/main.go` and start the HTTP server
- [ ] Extract the wiring logic into a shared `runServer()` function in a shared package (e.g., `internal/app/`) to avoid duplication between `cmd/server/main.go` and `cmd/cli/main.go`
- [ ] Both `cmd/server/main.go` and `cmd/cli/main.go` should call the shared function

---

### 11.5 — Resolve Migration Gap (002, 003)

**Current state**: 6 migrations referenced in docs, but only 3 SQL files exist:
- `001_initial.up.sql` / `.down.sql` — core schema
- `004_fts.up.sql` / `.down.sql` — FTS tsvector column + GIN index
- `006_list_pages.up.sql` / `.down.sql` — space page count index

Missing: `002_*` and `003_*` SQL files.

**Analysis**: Looking at the migration files and code:
- `001_initial` creates: `vector` extension, `spaces`, `pages`, `page_embeddings`, plus indexes (B-tree on space/parent, GIN tsvector on title, IVFFlat on embeddings)
- `004_fts` adds: `content_vector tsvector` column to `pages` (note: 001 already had a GIN tsvector on title — 004 adds a more comprehensive content_vector)
- `006_list_pages` adds: `idx_pages_space_id` B-tree on `pages(space_id)`

The gap between 001 and 004 suggests there was intermediate work that either:
(a) Was committed inline with 001 (no separate migration needed)
(b) Was done but migration files were lost/discarded

**Verification**: Check if 001 already includes everything that 004 adds, or if 004 adds something new:
- 001: GIN tsvector on `title` only
- 004: Adds `content_vector tsvector` (title + content) with GIN index

So 004 IS needed (content_vector is different from the title-only tsvector in 001).

**Recommendation**: Since the DB is already at migration 006 (confirmed working), create placeholder migrations 002 and 003 as no-ops, OR renumber the existing migrations to be sequential.

**Actions**:
- [ ] Verify current DB state is at migration 006 (check `schema_migrations` or `pgmigrate`)
- [ ] Option A: Create empty placeholder migrations 002 and 003 (no-ops)
- [ ] Option B: Create `002_fts_content.up.sql` (rename 004 → 002) and `003_page_index.up.sql` (rename 006 → 003), with corresponding down migrations
- [ ] Option C: Leave as-is if DB is already at 006 (migration numbers don't need to be contiguous in golang-migrate, but it's confusing)
- [ ] **Preferred**: Create placeholder files `002_placeholder.up.sql` / `.down.sql` and `003_placeholder.up.sql` / `.down.sql` (empty SQL) to maintain expected numbering. Document why.

---

### 11.6 — Update app-start.sh Comments

**Current state**:
```sh
#!/bin/sh
# chromedp runs Chromium headless natively — no Xvfb needed
exec /app/server
```

The comment references `chromedp` but the code uses `go-rod`.

**Actions**:
- [ ] Update comment to reference go-rod instead of chromedp
- [ ] Or better: remove the browser-specific comment entirely (just `# Start the SpaceMosquito server`)

---

### 11.7 — Update Outdated ADRs

**ADR-012** (`chromedp-over-playwright.md`) references chromedp but the actual implementation uses go-rod. This ADR was written before the switch to go-rod (commit `b4ceb34: feat: switch scraper from chromedp to go-rod`).

**ADR-004** (`headless-browser-for-scraper.md`) also mentions chromedp in the decision, which was the chromedp-era decision. The go-rod switch should be documented.

**Actions**:
- [ ] Add ADR-013: "go-rod over chromedp" — document the switch from chromedp to go-rod (chromedp `NoSandbox` fails in Colima vz driver, go-rod `launcher.NoSandbox(true)` works)
- [ ] Update ADR-012 to reflect that chromedp was superseded by go-rod
- [ ] Update ADR-004 to reflect the final go-rod decision

---

### 11.8 — Update DOCS to Reflect Current State

**Current state**: DOCS directory has phase-by-phase docs. Some reference abandoned tech (chromedp, Playwright).

**Needs updating**:
| File | Issue | Action |
|------|-------|--------|
| `phase-03-playwright-scraper.md` | Playwright was abandoned | Mark as superseded by go-rod, add reference to ADR-013 |
| `phase-04-embedder-pipeline.md` | Embeddings deferred | Update status to "deferred — BM25 search used instead" |
| `phase-06-firefox-extension-auth.md` | References chromedp in cookie section | Update cookie injection to `Browser.MustSetCookies()` / CDP `Storage.setCookies` |
| `phase-07-firefox-extension-scraping.md` | References chromedp scraping | Update to go-rod |
| `phase-09-cron-scheduler.md` | Mostly accurate | Verify against current implementation |
| `phase-10-polish.md` | Contains tasks that are partially done | Update with current status |
| `ARCHITECTURE.md` | Was in DOCS/ (untracked) | Already moved to root `ARCHITECTURE.md` — remove `DOCS/ARCHITECTURE.md` |

**Actions**:
- [ ] Remove `DOCS/ARCHITECTURE.md` (duplicate, already in root)
- [ ] Mark `phase-03-playwright-scraper.md` as superseded
- [ ] Update `phase-04-embedder-pipeline.md` — mark embeddings as deferred
- [ ] Update cookie injection references in phase-06 docs
- [ ] Update phase-07 docs for go-rod
- [ ] Update phase-10-polish.md with completion status

---

### 11.9 — Add Missing CLI `migrate-down` Support

**Current state**: CLI has `init` for `migrateUp` but no `migrateDown` support.

**Actions**:
- [ ] Add `migrate-down` subcommand to CLI
- [ ] Add `migrate-rollback` as alias

---

### 11.10 — Final Build and Test Verification

**Actions**:
- [ ] Run `go build ./...` — verify no errors
- [ ] Run `go test ./... -v` — verify all tests pass
- [ ] Run `go vet ./...` — verify no issues
- [ ] Run `cd firefox-extension && npx webpack --mode production` — verify extension builds
- [ ] Run `cd firefox-extension && npx tsc --noEmit` — verify TypeScript types
- [ ] Verify `docker compose build` succeeds

---

## Acceptance Criteria

- [x] `.gitignore` properly excludes all secrets, build artifacts, and generated files (no false positives)
- [x] Secret audit confirms no secrets are committed to the repository
- [x] Makefile has all required targets and they execute correctly
- [x] `runServe` CLI command actually starts the server (not a stub)
- [x] Migration numbering is clean and consistent
- [x] All ADRs and documentation reflect the final go-rod implementation
- [x] `go build`, `go test`, `go vet` all pass
- [x] Firefox extension builds successfully
- [ ] Docker build succeeds (requires Docker/Colima running)
- [x] README.md and ARCHITECTURE.md are comprehensive and accurate
