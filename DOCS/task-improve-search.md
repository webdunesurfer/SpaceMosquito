# Task: Improve Search Reliability (FTS Sync + Query Semantics)

## Objective

Fix search so pages that **exist in the catalog and are retrievable via `get-page`** are **findable by title terms** — including single-word queries copied from the title.

Reported failure mode (confirmed by user):

- `get-page` returns the page with the expected title (exact words, no punctuation)
- `search` with the full title → no hit (in default top 10)
- `search` with **individual title words** → still no hit (same limit)
- `reindex` already run → no improvement
- **`COUNT(pages) = COUNT(pages_fts)`** → row-count desync ruled out
- **`pages.title` = `pages_fts.title`** for repro page → stale FTS text ruled out
- **Direct FTS `MATCH` with all 3 title words finds the page at rank ~30** → **confirmed root cause: ranking + default `LIMIT 10`**

The page is **indexed and matchable** but **buried below the default result cap**. Not a missing-index bug.

**Parent references:**

- `DOCS/task-mcp-search-excerpts.md` — excerpt quality (orthogonal; do after reliability)
- `DOCS/task-dockerless-migrations.md` — SQLite FTS5 design
- `DOCS/task-get-page-by-confluence-id.md` — `get-page` works; search must catch up

**Out of scope (v1):**

- Vector / semantic search
- Live Confluence API search
- Search across `saved/` HTML files on disk (DB only)
- UI changes in browser extension

---

## Current Architecture

```
REST /api/search?q=...
MCP  confluence_search { query, space?, limit? }
CLI  spacemosquito search <query> [space]
        ↓
store.SearchPages(query, spaceKey, limit)   # default limit 10
        ↓
Driver-specific FTS
```

| Driver | Index | Query | Match semantics |
|--------|-------|-------|-----------------|
| **SQLite** (dockerless) | FTS5 `pages_fts(title, content)` Porter stemmer | `buildFTSQuery()` → `"w1" OR "w2" OR ...` | Any token matches |
| **Postgres** (Docker) | `tsvector` on title (weight A) + content (B) | `plainto_tsquery('english', q)` | All tokens (minus stop words) |

**Indexed source:** `pages.title` + `pages.content` (text extracted at crawl/import). **Not** live Confluence.

### SQLite FTS maintenance today

Triggers on `pages` INSERT/UPDATE keep `pages_fts` in sync **when they fire**:

```sql
-- migrations/sqlite/004_fts.up.sql
INSERT INTO pages_fts(page_id, title, content) VALUES (new.id, new.title, coalesce(new.content, ''));
```

`IndexAllPageContents` / `spacemosquito reindex` today:

```go
INSERT INTO pages_fts(pages_fts) VALUES('rebuild')
```

**`rebuild` only reorganizes existing FTS rows.** It does **not** read from `pages` and backfill missing `pages_fts` rows.

---

## Problem Analysis

### Confirmed root cause (user repro)

```
3-word title query
  → buildFTSQuery: "w1" OR "w2" OR "w3"
  → FTS matches many pages (any single word)
  → bm25 ranks by term frequency in title+body (title not boosted)
  → target page ranks ~30
  → CLI / default API return LIMIT 10
  → user sees "no results" / page missing
```

**Why OR hurts:** A page whose title contains all three words competes with hundreds of pages whose **body** mentions one common word. Without title weighting, body-heavy pages win.

**Why AND fixes it:** `"w1" AND "w2" AND "w3"` requires all terms — the exact-title page should rank near the top.

### Symptom matrix

| Check | User observation | Implication |
|-------|------------------|-------------|
| `get-page` | Works; title correct | `pages` row is correct |
| Search full title | No hit in CLI/default API | Buried below rank 10 |
| Search single title word | No hit in CLI/default API | Common word → many matches, same cap |
| `reindex` | No help | Ranking issue, not index maintenance |
| `COUNT(pages) = COUNT(pages_fts)` | Confirmed | FTS rows present |
| `pages.title` = `pages_fts.title` | Confirmed | FTS text is current |
| FTS `MATCH` all 3 words | Hit at rank ~30 | Index works; ranking/limit broken |

### Contributing factors (ordered)

| # | Factor | Impact |
|---|--------|--------|
| 1 | **`buildFTSQuery` uses OR** | Inflates result set; weak matches outrank strong title matches |
| 2 | **No title boost in bm25** | Body term frequency beats title-only or title-sparse pages |
| 3 | **Default `limit=10`** | CLI hardcoded; page at rank 30 never returned |
| 4 | **Porter + common words** | Single-word queries match huge swaths of corpus |

### Ruled out

- Missing `pages_fts` rows (count parity)
- Stale `pages_fts.title` (parity query matched)
- Tokenization failure (FTS MATCH finds the page)
- Space filter (user SQL had no space filter issue)

### Diagnostic queries (run on repro page)

```sql
-- Page exists?
SELECT p.id, p.confluence_id, p.title, length(p.content), s.key
FROM pages p JOIN spaces s ON s.id = p.space_id
WHERE p.confluence_id = ?;

-- FTS row + title parity (critical when counts match)
SELECT p.title AS pages_title, f.title AS fts_title, f.content AS fts_content
FROM pages p
JOIN pages_fts f ON f.page_id = p.id
WHERE p.confluence_id = ?;

-- Count parity (user: equal)
SELECT (SELECT COUNT(*) FROM pages) AS pages,
       (SELECT COUNT(*) FROM pages_fts) AS fts_rows;

-- Does FTS match this page_id for one title word?
SELECT page_id, title FROM pages_fts
WHERE page_id = '<uuid>' AND pages_fts MATCH '"exactword"';

-- Global FTS match (no JOIN) — is token indexed at all?
SELECT page_id, title FROM pages_fts WHERE pages_fts MATCH '"exactword"';

-- Same query through search JOIN (detect space/JOIN exclusion)
SELECT p.confluence_id, s.key, p.title
FROM pages_fts
JOIN pages p ON p.id = pages_fts.page_id
JOIN spaces s ON s.id = p.space_id
WHERE pages_fts MATCH '"exactword"';
```

---

## Target Behavior

After this task:

1. **Multi-word queries use AND by default** — all terms must match; repro page rises from ~30 to top results.
2. **Title matches rank higher** than body-only matches (SQLite `bm25` column weights).
3. **Exact-title pages surface in top 10** for queries copied from the title.
4. **CLI supports `--limit`** (REST/MCP already do).
5. **Title fallback** optional safety net when FTS returns 0 rows (lower priority now).
6. **Regression test**: 3-word title page ranks in top 3 after multi-word search.

---

## Implementation Plan

### Phase 1 — Query AND semantics (P0)

**File:** `internal/store/sqlite/sqlite.go` — `buildFTSQuery`

| Today | Target |
|-------|--------|
| `"w1" OR "w2" OR "w3"` | `"w1" AND "w2" AND "w3"` |

This alone should move the repro page from rank ~30 into the default top 10.

### Phase 2 — Title-boosted ranking (P0)

**SQLite** — weight title column higher in BM25:

```sql
bm25(pages_fts, 0.0, 10.0, 1.0) AS similarity
-- weights: page_id UNINDEXED (0), title (10), content (1)
```

Helps single-word queries where title match should beat body-only mentions.

### Phase 3 — Configurable limit on CLI (P1)

**File:** `internal/cliapp/run.go`

```sh
spacemosquito search "foo bar baz" --limit 50
spacemosquito search "foo bar baz" SPACE --limit 50
```

REST already supports `?limit=50`. MCP supports `limit`. CLI is hardcoded to 10 today.

### Phase 4 — Force FTS text sync on reindex (P2)

Still useful for edge cases (pre-migration data, trigger gaps), but **not the user repro**. Repopulate `pages_fts` from `pages` on `reindex`, then `rebuild`.

### Phase 5 — Query normalization (P2)

Strip punctuation per token; optional NFC normalize. Single-word queries unchanged.

### Phase 6 — Title fallback (P3)

**Decision:** FTS first; run title `LIKE` fallback **only when FTS returns 0 rows**.

Case-insensitive title substring match requiring all query tokens in title. Merge not needed — return fallback results directly when FTS is empty.

### Phase 7 — Observability (P2)

Optional `search --debug` showing `fts_query`, total FTS hits before limit, rank of best title match.

### Phase 8 — Tests

| Test | Package |
|------|---------|
| `TestBuildFTSQuery_AND` | `internal/store/sqlite` |
| `TestSearchPages_titleRanksInTopN` | sqlite — 3-word title page in top 3 with noisy corpus |
| `TestSearchPages_titleBoost` | sqlite — title match beats body-only |
| Integration: REST search `?limit=10` finds title page | `internal/app` (integration tag) |

Fixture: page title `Alpha Beta Gamma`, many decoy pages each mentioning one word in body — full title query must return target in top 3.

### Phase 9 — Docs

| Doc | Update |
|-----|--------|
| `README-dockerless.md` | AND semantics; `?limit=` / MCP `limit` |
| `DEVELOPMENT.md` | Ranking behavior; diagnostic SQL |

---

## API / MCP Changes

**Default behavior change** (breaking, acceptable per project rules):

- Multi-word search becomes **AND** (SQLite); fewer but more relevant results.
- Title-boosted bm25 surfaces title matches above body-only hits.

**Workaround until fixed:**

```sh
# REST
curl 'http://localhost:PORT/api/search?q=word1+word2+word3&limit=50'

# MCP confluence_search
{ "query": "word1 word2 word3", "limit": 50 }
```

CLI has no `--limit` today — hardcoded to 10.

**Optional follow-ups (defer):**

- `?match=any|all` on REST search (default `all`)
- MCP `confluence_search` arg `match_mode`
- Phrase search with quotes

No change to response shape (`SearchHit`).

---

## Acceptance Criteria

- [ ] User repro: 3-word title query returns page in **top 10** (default limit)
- [ ] `buildFTSQuery` uses **AND** for multi-token queries
- [ ] Title-weighted BM25 on SQLite (title ranks above body-only matches)
- [ ] CLI `search --limit N` flag
- [ ] Regression test: title page in top 3 with noisy OR-style corpus
- [ ] `go test -race ./...` passes

---

## Design Decisions (resolved defaults)

| Question | Decision |
|----------|----------|
| Root cause | **OR + no title boost + limit 10** (confirmed rank ~30) |
| Multi-word match | **AND** (all terms) |
| Title boost | **10x title, 1x content** in bm25 |
| Title fallback | **FTS first; title LIKE only on 0 FTS hits** (v1) |
| Tokenizer | **Keep `porter`** for v1; see comparison below |
| `pages_without_fts` in MCP stats | **Defer** |
| `reindex` repopulate | **P2** — not needed for this repro |
| Default limit | Keep 10; add CLI `--limit`; document REST/MCP workaround |

---

## Resolved Questions

### 1. Title fallback: FTS first, fallback on 0 hits

**Decision:** Run FTS search first. Only if FTS returns **zero rows**, run a case-insensitive title substring fallback (`LIKE` with all query tokens required in title). Do **not** union fallback with FTS results in v1.

**Rationale:** User repro has FTS hits (rank ~30) — union would add noise without fixing ranking. Fallback covers true index gaps (stale FTS, tokenizer edge cases) without diluting ranked FTS results.

### 2. Porter vs `unicode61` tokenizer

**Decision:** **Keep `porter` for v1.** Revisit only if AND + title boost still leaves gaps.

| | **`porter`** (current) | **`unicode61`** |
|---|------------------------|-----------------|
| **What it does** | Splits on non-alphanumerics, then **stems** English words (`running` → `run`, `mice` → `mice`) | Splits on Unicode rules; **no stemming** — tokens are literal substrings |
| **English recall** | Higher — query `run` matches indexed `running` | Lower — `run` does not match `running` unless both forms indexed |
| **English precision** | Lower — `run` also matches `runner`, `running` via stem overlap | Higher — exact token match only |
| **Non-English / mixed** | Stemming is English-only; CJK and most non-Latin tokenized as single-char or byte sequences (same as unicode61 for non-ASCII) | Better punctuation/Unicode handling; still no CJK word segmentation |
| **Hyphenated tokens** | `foo-bar` → often one token `foo-bar` or split depending on FTS version | `foo-bar` typically splits to `foo`, `bar` |
| **Numbers / acronyms** | Usually kept; short tokens may behave oddly | Kept literally (`v2` stays `v2`) |
| **Migration cost** | None (already deployed) | New migration: recreate `pages_fts`, reindex all pages; **changes all existing search behavior** |
| **Fit for this bug** | Not the root cause (FTS found page at rank 30) | Unlikely to fix ranking; might help hyphenated/acronym titles |

**When to switch:** If after AND + title boost, specific pages still fail single-word search because Porter over-stems or merges tokens differently than the user's query — test that page title on both tokenizers before migrating.

### 3. `pages_without_fts` in MCP stats

**Decision:** **Defer.** Not needed for current repro; add to stats API later if operators need FTS health visibility.

---

## Estimated Effort

| Phase | Size |
|-------|------|
| P1 AND query | S |
| P2 title boost | S |
| P3 CLI --limit | S |
| Tests + docs | S |

**Total:** ~0.5–1 day.

---

## Related Files

| File | Role |
|------|------|
| `internal/store/sqlite/sqlite.go` | `SearchPages`, `buildFTSQuery`, `IndexAllPageContents` |
| `internal/db/models.go` | Postgres `SearchPages`, `IndexAllPageContents` |
| `migrations/sqlite/004_fts.up.sql` | FTS5 schema + triggers |
| `internal/bootstrap/import_saved.go` | Post-import reindex call |
| `internal/api/search.go` | REST handler |
| `internal/mcp/server.go` | `confluence_search` tool |
| `internal/cliapp/run.go` | `search`, `reindex` commands |
