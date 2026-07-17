# Task: Search Excerpts Centered on Matched Terms

> **Note:** Postgres / Docker excerpts path is removed. Implementation uses SQLite FTS + `internal/search` normalization.

## Objective

Fix search snippets so **`excerpt` in `confluence_search` / `GET /api/search` shows text around the query match**, not always the first ~200 characters of the page. Agents must be able to see *why* a page matched and skim relevant sections without calling `confluence_get_page` for every hit.

**Source:** `DOCS/issues/mcp-issues.md` issue #2 (critical).

**Out of scope for this task:**

- `confluence_list_space` pagination or titles-only mode (issues #3–#4)
- `confluence_get_page_by_title` (issue #5)
- Vector / embedding search excerpts (`SearchEmbeddings`) — FTS lexical path only
- SQLite / dockerless FTS snippets (`DOCS/task-dockerless-migrations.md` — separate design)
- Docker Compose e2e or live Confluence tests

---

## Problem Summary

Today `SearchPages` builds the excerpt with:

```sql
LEFT(p.content, 200) AS excerpt
```

Ranking uses `ts_rank` on `content_vector` and the query filter `@@ plainto_tsquery('english', $1)`, but **excerpt generation ignores the query**. Every hit shows the page start (often title noise or intro boilerplate), regardless of where terms matched.

| Symptom | Cause |
|---------|--------|
| Excerpt ~200 chars | Hard `LEFT(..., 200)` |
| Always page beginning | No match-aware slicing |
| Poor MCP usefulness | Agents cannot triage long pages from search alone |

**Consumers affected (all use `db.SearchPages`):**

- MCP `confluence_search` → `SearchHit.excerpt`
- REST `GET /api/search` → `results[].excerpt`
- CLI `search` (also truncates excerpt to 150 chars locally — fix in this task)

---

## Target Behavior

After this task:

1. **Excerpt is match-centered** — surrounds the first (or best) occurrence of query terms in page text.
2. **Length ~300–500 characters** (configurable constant; default **400**).
3. **Same `plainto_tsquery('english', query)`** as ranking/filter — stemming and stop-word behavior stay consistent.
4. **Title matches visible** — if the hit is title-only or match is in the title, excerpt should include that context (not only `content` body).
5. **No schema migration** — SQL-only change in `SearchPages` (and shared excerpt helper if extracted).

Example (illustrative):

```json
{
  "confluence_id": 12345,
  "space_key": "PROJ",
  "title": "Invoice Filtering Logics",
  "excerpt": "...when <b>invoice</b> balance drops below threshold the filter applies <b>logic</b> from section 3.2...",
  "similarity": 0.42
}
```

Highlight markers optional — see [Resolved decisions](#resolved-decisions).

---

## Recommended Approach: PostgreSQL `ts_headline`

Use Postgres built-in headline generation on the same document text and `tsquery` as search:

```sql
ts_headline(
  'english',
  coalesce(p.title, '') || E'\n\n' || coalesce(p.content, ''),
  plainto_tsquery('english', $1),
  'MaxFragments=1, MaxWords=60, MinWords=20, ShortWord=3'
) AS excerpt
```

Replace `LEFT(p.content, 200)`.

**Why `ts_headline`:**

- Native to existing Postgres + FTS stack (`004_fts.up.sql`)
- Match-aware fragments aligned with `@@` / `ts_rank`
- No second pass over full text in Go for the hot path

**Why include `title` in headline source:**

- `content_vector` weights title (`A`) and body (`B`); a title-only match should not produce an excerpt from unrelated body start.

### Post-processing (Go, optional thin layer)

| Step | Purpose |
|------|---------|
| Strip `ts_headline` `<b>...</b>` tags | Plain text for LLM clients (default) |
| Or keep tags / map to `...` | If we want visible emphasis — decide in implementation |
| `strings.TrimSpace` | Normalize whitespace |
| Hard cap e.g. 500 runes | Safety net if `MaxWords` overshoots |

Extract `normalizeExcerpt(raw string, maxLen int) string` in `internal/search/` for unit testing without Postgres.

---

## Alternative Approaches (not recommended for v1)

| Approach | Pros | Cons |
|----------|------|------|
| **Go: find substring of query terms in `content`** | Unit-testable without DB | No stemming; misses `tsquery` semantics |
| **`substring(content from position(...))` in SQL** | Simple | Fragile for multi-term queries |
| **Store offsets at index time** | Fast read | Schema + indexer change |

Stick with **`ts_headline`** unless integration tests reveal bad snippets on real data.

---

## Implementation Plan

### Phase 1 — SQL (`internal/db/models.go`)

| Change | Detail |
|--------|--------|
| `SearchPages` | Replace `LEFT(p.content, 200)` with `ts_headline(...)` expression |
| Bind query once | Reuse same `$1` for `plainto_tsquery`, `@@`, `ts_rank`, and `ts_headline` |
| Constants | `excerptMaxWords`, `excerptMaxRunes` in Go or SQL options string |

No migration — `content` and `content_vector` unchanged.

### Phase 2 — Excerpt normalization (`internal/search/excerpt.go`)

| Function | Responsibility |
|----------|----------------|
| `NormalizeExcerpt(raw string, maxRunes int) string` | Strip HTML-like headline tags, collapse space, truncate |

Apply in `ToSearchHits` **or** in `SearchPages` before return — prefer **one place** (`ToSearchHits`) so MCP/REST/CLI share behavior.

### Phase 3 — CLI

Remove or raise the **150-char client-side truncation** in `cmd/cli/main.go` `runSearch` — it defeats longer DB excerpts.

### Phase 4 — Docs

| File | Update |
|------|--------|
| `DOCS/issues/mcp-issues.md` | Mark issue #2 fixed when done |
| `DEVELOPMENT.md` | Note excerpt behavior briefly (optional) |

---

## Testing Strategy

### Without Docker (unit tests)

| Test | File | Cases |
|------|------|-------|
| `NormalizeExcerpt` | `internal/search/excerpt_test.go` | Strip `<b>` tags; truncate; empty input; whitespace |
| `ToSearchHits` | existing `dto_test.go` | Pass-through of normalized excerpt (if wired there) |

### ~~With Postgres (integration)~~ — superseded

Extend `DOCS/task-db-integration-tests.md` Phase 1:

| Test | Setup | Assert |
|------|-------|--------|
| `TestSearchPages_excerptContainsQueryTerm` | Page with match only in middle of long `content` | `excerpt` contains query term; does **not** equal `LEFT(content, 200)` |
| `TestSearchPages_excerptTitleMatch` | Short body, distinctive title | Excerpt reflects title term |
| `TestSearchPages_multiWordQuery` | `content` matches second term only | Excerpt includes that term |

Use: `go test -race -tags=integration ./internal/app/...` (SQLite in-process).

**Default CI** stays `go test ./...` without integration tag — no Docker required for merge if unit tests + manual smoke suffice; integration tests strongly recommended before closing task.

### Manual smoke

```bash
# After crawl/index, search for a term known to appear mid-page only
curl "http://localhost:8081/api/search?q=unique_term&space_key=PROJ"
# Inspect results[].excerpt — should surround unique_term, not page intro
```

---

## Resolved Decisions

| # | Question | Decision |
|---|----------|----------|
| 1 | Highlight markup in excerpt | **Strip `<b>`/`</b>`** from `ts_headline` for plain-text MCP/LLM consumption (default) |
| 2 | Target length | **~400 runes** cap after normalization; `MaxWords=60` in `ts_headline` |
| 3 | Headline document | **`title + newline + content`** |
| 4 | Configurable length | **Constants in Go** for v1; no `config.yaml` field unless needed |
| 5 | `SearchEmbeddings` | **Out of scope** — still uses raw content slice today |
| 6 | Multiple fragments | **`MaxFragments=1`** |
| 7 | Field name | **Keep `excerpt`** |
| 8 | Highlight syntax | **None** — strip `ts_headline` `<b>` tags |

---

## Open Questions

_None — resolved for this task._

---

## Acceptance Criteria (implementation)

- [x] `SearchPages` uses `ts_headline` instead of `LEFT(p.content, 200)`
- [x] `NormalizeExcerpt` strips tags and caps at ~400 runes
- [x] MCP, REST, CLI share excerpts via `ToSearchHits`
- [x] CLI no longer truncates to 150 chars
- [x] Unit tests for `NormalizeExcerpt`
- [x] `go test -race ./...` passes without Postgres
- [x] `DOCS/issues/mcp-issues.md` issue #2 marked fixed

---

## Related Documents

| Doc | Relationship |
|-----|--------------|
| `DOCS/issues/mcp-issues.md` | Source issue #2 |
| `DOCS/task-mcp-confluence-id.md` | Search hit shape (`SearchHit.excerpt`) — already aligned |
| `DOCS/task-db-integration-tests.md` | Superseded — Postgres tests not planned |
| `DOCS/task-dockerless-migrations.md` | Future: FTS5 `snippet()` for SQLite |
| `space-mosquito/migrations/004_fts.up.sql` | Existing `content_vector` definition |
