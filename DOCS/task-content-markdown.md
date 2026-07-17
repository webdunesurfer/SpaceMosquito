# Task: Structured Page Content (Markdown Extraction)

## Objective

Replace flat `doc.Text()` page content with **structured Markdown** so `pages.content` is readable, searchable, and useful for MCP/REST consumers — without requiring perfect Confluence fidelity.

**Reported failure mode:**

```
orders are defined in acquisition definitionNo Articles are given
```

Words merge across block boundaries (`definition` + `No` → `definitionNo`). Search tokens and agent-readable content both suffer.

**Target:** “Good enough” Markdown detecting:

- Headers (`#` … `######`)
- Bold / italic
- Tables
- Links (`[text](url)`)

**Not required:** pixel-perfect Confluence rendering, macro replay, layout columns, emoji, task lists, code syntax highlighting.

**Parent references:**

- `ADR/015-saved-page-format-with-markdown.md` — **supersedes ADR-006**; adds `content.md` to on-disk layout
- `ADR/006-saved-page-format.md` — historical; superseded
- `DOCS/task-api-migration.md` — API `body.storage` vs browser HTML
- `DOCS/task-import-saved.md` — re-extract content from `saved/` on import
- `DOCS/task-mcp-search-excerpts.md` — better excerpts once content has real word boundaries
- `DOCS/task-improve-search.md` — search ranking (orthogonal; benefits from cleaner tokens)

**Out of scope (v1):**

- Replacing `index.html` (keep for offline browser viewing)
- Live Confluence rendering or macro execution
- PDF / DOC export
- Vector embeddings on Markdown
- UI / browser extension changes
- Full Confluence storage-format macro catalog (panels, Jira, draw.io, etc.) — stub or skip unknown macros

---

## Current State

### Pipeline (API and browser share the tail)

```
API:  GET /rest/api/content/{id}?expand=body.storage
        → body.storage XHTML (Confluence storage format)
Browser fallback: go-rod page.HTML()
        → full rendered DOM

Both:
  extractContent()        # strip chrome, scripts, assets
    → clean HTML          # saved/index.html
  extractTextFromHTML()   # goquery doc.Text() + strings.Fields
    → pages.content       # FTS-indexed flat string
```

| Step | File | What it does |
|------|------|--------------|
| `extractContent` | `internal/scraper/page.go` | Chrome removal, asset download, link rewrite |
| `extractTextFromHTML` | `internal/scraper/page.go` | **`doc.Text()`** — concatenates all text nodes, no block separators |
| `savePageMetadata` | `internal/scraper/scraper.go` | `Content: extractTextFromHTML(pg.CleanHTML)` |
| `import_saved` | `internal/bootstrap/import_saved.go` | Duplicate `extractTextFromHTML` on `index.html` / `raw.html` |

### What is **not** done today

- No Confluence storage-format parsing (`ac:structured-macro`, `ac:parameter`, `ri:page`, `ac:layout`)
- No main-content scoping (`.wiki-content`, `#main-content`) on browser HTML
- No block-boundary spacing (`</p><p>` → merged words)
- No structure preservation in DB `content`

### On-disk artifacts today

```
saved/{space}/{page-title}/
  metadata.json
  index.html      # clean HTML
  raw.html        # storage XHTML or rendered page
  assets/...
```

**Gap:** No `content.md`. DB holds only flattened text.

---

## Problem Analysis

### Root cause 1: `doc.Text()` flattening (primary)

`goquery.Document.Text()` walks the DOM and concatenates text from siblings **without inserting separators**.

| HTML | Flat text output |
|------|------------------|
| `<p>definition</p><p>No Incentives</p>` | `definitionNo Incentives` |
| `<td>A</td><td>B</td>` | `AB` |
| `</li><li>` adjacent list items | `item1item2` |

`strings.Join(strings.Fields(text), " ")` only collapses **existing** whitespace — it cannot split `definitionNo`.

### Root cause 2: Confluence storage format noise (API path)

`body.storage` is XHTML-based XML with custom namespaces:

- `ac:structured-macro` + `ac:parameter` — config strings (JQL, titles, sizes) extracted as plain text
- `ac:rich-text-body` — actual macro content mixed with parameters in document order
- `ri:page`, `ri:attachment` — resource metadata as text
- `ac:layout` / `ac:layout-cell` — column text concatenated without gaps

`doc.Text()` treats macro parameters and body copy equally → junk phrases and more boundary merges.

### Root cause 3: Browser path noise (fallback)

Rendered HTML adds wrappers, duplicated titles, ARIA strings. `stripChrome` removes known selectors but does not isolate the wiki body widget. Same flattening applies.

### Downstream impact

| Consumer | Effect |
|----------|--------|
| FTS (`pages_fts.content`) | Merged tokens (`definitionNo`) — search misses individual words |
| MCP `confluence_get_page` | Wall of text — agents cannot skim structure |
| REST `GET /api/pages/{id}` | Same |
| Future match-centered excerpts | Poor anchors in flat text |

### Ruled out

- Search ranking / AND semantics (addressed in `task-improve-search.md`)
- FTS desync / missing rows
- Porter tokenizer (keep `porter`; problem is upstream extraction)

---

## Target Behavior

After this task:

1. **Every crawled/imported page** has `content.md` on disk next to `index.html`.
2. **`pages.content` stores Markdown** (same field, new semantics) — FTS indexes Markdown text.
3. **Block boundaries preserved** — no `word1word2` merges across paragraphs, headings, table cells, list items.
4. **Structure detectable** — headers, bold/italic, tables, links visible in Markdown.
5. **Macro noise reduced** — API path skips or summarizes `ac:parameter`; keeps `ac:rich-text-body` prose.
6. **`import_saved` and re-crawl** both produce Markdown via the same converter.
7. **Backward compatible viewing** — `index.html` unchanged for browser offline use.

### Example (illustrative)

**Before (`pages.content`):**

```
orders are defined in acquisition definitionNo Articles are given
```

**After (`content.md` / `pages.content`):**

```markdown
orders are defined in acquisition definition.

No Articles are given.

| Column A | Column B |
|----------|----------|
| Foo      | Bar      |

See [Related page](https://example.atlassian.net/wiki/spaces/X/pages/123).
```

Exact wording may differ; **word boundaries and structure** are the win.

---

## Design: Two Input Paths, One Markdown Output

```
                    ┌─────────────────────────────────────┐
                    │         Markdown converter          │
                    │  (shared output: content.md + DB)   │
                    └─────────────────────────────────────┘
                           ▲                    ▲
                           │                    │
              ┌────────────┴──────┐   ┌────────┴────────────┐
              │ Storage converter │   │  HTML → Markdown    │
              │ (API body.storage)│   │  (clean index.html) │
              └────────────┬──────┘   └────────┬────────────┘
                           │                    │
                    ScrapePageAPI          ScrapePage (browser)
                           │                    │
                           └────────┬───────────┘
                                    │
                           extractContent() → index.html
```

| Path | Input | Converter | Notes |
|------|-------|-----------|-------|
| **API** (primary) | `body.storage` before or after `extractContent` | **Storage-aware** — map `p`, `h1`–`h6`, `strong`/`em`, `table`, `ac:link`/`a`; strip/skip macro params | Best signal-to-noise |
| **Browser** | `index.html` (clean HTML) | **Generic HTML→MD** | Turndown-style rules on `.wiki-content` if present, else whole clean body |
| **import_saved** | `index.html` (prefer), `raw.html` | Same HTML→MD as browser | No API available offline |

**v1 pragmatic choice:** Implement **HTML→Markdown on `index.html`** first (fixes both paths after `extractContent`). Add **storage-aware pass** in v2 if macro noise remains high on API crawls.

---

## Markdown “Good Enough” Rules

### Block elements → newlines

| HTML | Markdown |
|------|----------|
| `h1`–`h6` | `#` … `######` + blank line |
| `p` | paragraph + blank line |
| `br` | newline |
| `li` | `- item` |
| `tr` / `td` / `th` | pipe table (header row + separator) |
| `pre` / `code` | fenced `` ``` `` or `` `inline` `` |

### Inline

| HTML | Markdown |
|------|----------|
| `strong`, `b` | `**text**` |
| `em`, `i` | `*text*` |
| `a[href]` | `[text](href)` — keep absolute URLs; local `#` links optional |

### Skip or stub (v1)

| Element | Behavior |
|---------|----------|
| `script`, `style` | Skip (already stripped in `extractContent`) |
| `ac:parameter` | **Skip** (macro config noise) |
| `ac:structured-macro` without rich body | `[macro: name]` one-line stub or omit |
| `ac:rich-text-body` | Recurse into children |
| `ri:page` inside `ac:link` | `[content-title](original-url)` if URL known from metadata |
| Images | `![alt](local-path)` when `src` is local asset path |

### Normalization (after conversion)

- Collapse 3+ newlines → 2
- Trim trailing spaces per line
- Truncate to **50k chars** (match current `extractTextFromHTML` cap)
- Do **not** run `strings.Fields` on the whole document (destroys Markdown newlines)

---

## Implementation Plan

### Phase 1 — HTML → Markdown core (P0)

**New package:** `internal/contentmd/` (or `internal/scraper/markdown.go`)

- `HTMLToMarkdown(html string) (string, error)` — generic converter
- Use a maintained Go library if suitable (evaluate: `github.com/JohannesKaufmann/html-to-markdown`, `goldmark` reverse, or custom walker for minimal rules)
- Unit tests with fixtures:
  - Adjacent paragraphs (`definition` / `No`)
  - Table 2×2
  - `h2` + bold + link
  - Nested `strong` in `p`

**Wire into scraper:**

```go
// savePageMetadata
md, err := contentmd.HTMLToMarkdown(pg.CleanHTML)
pg.ContentMD = md  // or write directly to pages.content
```

**Wire into `import_saved`:** replace duplicate `extractTextFromHTML` with shared `HTMLToMarkdown`.

### Phase 2 — Persist `content.md` (P0)

**`internal/storage/writer.go`:**

```go
func (w *Writer) SaveMarkdown(dir, markdown string) error
// writes {dir}/content.md
```

**`store.Page`:** optional `MarkdownPath string` or derive as `{file_dir}/content.md` (no migration if path is deterministic).

**`savePageMetadata`:** save `content.md` after `index.html`.

### Phase 3 — Content region scoping (P1)

Before HTML→MD on browser-sourced clean HTML:

1. Try `#main-content`, `.wiki-content`, `[data-testid="page-content"]`
2. Fall back to `<main>`, then full document

Reduces chrome leakage in browser fallback path.

### Phase 4 — Storage-format-aware converter (P2)

**New:** `StorageToMarkdown(storageXHTML string) (string, error)`

- Parse with `encoding/xml` or HTML parser tolerating `ac:` tags
- Explicit rules for Confluence elements (see [storage format docs](https://confluence.atlassian.com/doc/confluence-storage-format-790796544.html))
- Use in `ScrapePageAPI` **instead of** HTML→MD on storage HTML, when `body.storage` is available
- Fall back to HTML→MD on `index.html` if storage parse fails

### Phase 5 — Re-index existing catalog (P1)

**CLI:** extend existing `reindex` command with `--content` flag:

```sh
spacemosquito reindex              # FTS rebuild only (current behavior)
spacemosquito reindex --content    # regenerate content.md + pages.content from saved HTML, then FTS sync
```

- Walk `pages.html_path` (or `file_dir` + `index.html`)
- Regenerate `content.md` via `HTMLToMarkdown` + update `pages.content`
- FTS updates via existing page upsert triggers (or explicit reindex tail)
- Idempotent; report counts (updated / skipped / errors)

**Document:** users with existing installs run `reindex --content` once after upgrade.

### Phase 6 — API / MCP surface (P1)

No response shape break if `content` field semantics change from flat text → Markdown (still a string).

| Surface | Change |
|---------|--------|
| `GET /api/pages/{id}` | `content` is Markdown |
| MCP `confluence_get_page` | same |
| `confluence_list_space?include_content=true` | same |

Optional follow-up: `content_format: "markdown"` field in JSON (defer v1).

### Phase 7 — Tests

| Test | Package |
|------|---------|
| `TestHTMLToMarkdown_adjacentParagraphs` | `internal/contentmd` — no merged words |
| `TestHTMLToMarkdown_table` | pipe table output |
| `TestHTMLToMarkdown_headersBoldLinks` | structure |
| `TestScrapePageAPI_contentQuality` | fixture storage XHTML |
| `TestImportSaved_producesMarkdown` | bootstrap |
| Integration: search finds `No` and `definition` separately on repro fixture | `internal/app` (integration tag) |

**Fixture:** HTML reproducing user’s `definitionNo` case — assert Markdown contains `definition\n\nNo` or separate paragraphs.

### Phase 8 — Docs

| Doc | Update |
|-----|--------|
| `ADR/015-saved-page-format-with-markdown.md` | Saved page layout with `content.md` (supersedes ADR-006) |
| `ADR/006-saved-page-format.md` | Marked superseded |
| `DEVELOPMENT.md` | Content pipeline diagram |
| `README.md` | Mention Markdown content for search/MCP |
| `DOCS/task-import-saved.md` | Point text extraction to `contentmd` package |

---

## Schema / Migration

**v1 — no DB migration required** if:

- `pages.content` continues to hold the searchable text blob (now Markdown)
- `content.md` path is derivable: `{file_dir}/content.md`

**Optional v2:** `pages.content_format TEXT DEFAULT 'markdown'` for forward compatibility.

FTS triggers unchanged — they index `pages.content` on upsert.

---

## Acceptance Criteria

- [ ] Repro case: `definition` and `No` are separate tokens in `pages.content` (not `definitionNo`)
- [ ] `content.md` written for every new crawl save
- [ ] `import_saved` produces Markdown from `index.html`
- [ ] Headers, bold, tables, links present in fixture output (smoke assertions)
- [ ] `index.html` and `raw.html` behavior unchanged
- [ ] `go test -race ./...` passes
- [ ] `reindex --content` documented for existing installs

---

## Design Decisions (defaults)

| Question | Decision |
|----------|----------|
| Replace or supplement `index.html`? | **Supplement** — add `content.md`; keep HTML for browsing |
| Store MD where? | **`pages.content` + `content.md` on disk** |
| API vs browser converter | **v1:** HTML→MD on clean HTML; **v2:** storage-aware for API |
| Macro parameters | **Skip** in v1 |
| Unknown macros | One-line `[macro: name]` stub or omit |
| Max length | **50k chars** (parity with today) |
| Porter / FTS | **Unchanged** — better input fixes tokenization |
| `doc.Text()` | **Remove** from content path (keep only if needed for debug) |
| HTML→MD implementation | **Approved library** (Turndown-style); custom walker only if `ac:` tags break parser |
| Re-extract CLI | **`reindex --content`** |
| `content.md` path in API | **Defer** v1 |
| ADR | **[ADR-015](ADR/015-saved-page-format-with-markdown.md) supersedes ADR-006** |

---

## Resolved Questions

### 1. Library vs custom walker

**Decision:** **Approved** — use a maintained HTML→Markdown library (e.g. Turndown-style rules in Go). Spike in Phase 1; fall back to a thin custom DOM walker only if Confluence `ac:` tags break the library parser.

### 2. Re-extract CLI

**Decision:** **`spacemosquito reindex --content`** — extend the existing `reindex` command rather than a new bootstrap subcommand.

- `reindex` alone — FTS maintenance only (current behavior)
- `reindex --content` — regenerate Markdown from saved `index.html`, update DB + `content.md`, then refresh FTS

### 3. Expose `content.md` path in API

**Decision:** **Defer.** Consumers use `pages.content` (Markdown string). On-disk path is `{file_dir}/content.md` by convention.

### 4. ADR amendment

**Decision:** **New ADR-015 supersedes ADR-006.** ADR-006 status set to Superseded; hybrid HTML + Markdown model documented in ADR-015.

---

## Estimated Effort

| Phase | Size |
|-------|------|
| P1 HTML→MD + wire scraper/import | M |
| P2 content.md on disk | S |
| P3 content region scoping | S |
| P5 re-extract command | M |
| P4 storage-aware converter | L (defer v2) |
| Tests + docs | M |

**v1 total:** ~2–3 days (Phases 1–3, 5, 7–8).  
**v2:** +2–3 days for storage-format converter.

---

## Related Files

| File | Role |
|------|------|
| `internal/scraper/page.go` | `extractTextFromHTML` — replace/supersede |
| `internal/scraper/scraper.go` | `savePageMetadata`, `ScrapePageAPI`, `ScrapePage` |
| `internal/bootstrap/import_saved.go` | duplicate text extraction |
| `internal/storage/writer.go` | add `SaveMarkdown` |
| `internal/store/store.go` | `Page` struct |
| `migrations/sqlite/004_fts.up.sql` | indexes `pages.content` (unchanged) |
| `ADR/015-saved-page-format-with-markdown.md` | on-disk contract (supersedes ADR-006) |
| `internal/cliapp/run.go` | `reindex --content` flag |
