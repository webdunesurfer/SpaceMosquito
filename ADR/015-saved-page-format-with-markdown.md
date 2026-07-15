# ADR-015: Saved Page Format — HTML + Markdown Content

- **Status**: Accepted
- **Date**: 2026-07-15
- **Supersedes**: [ADR-006](006-saved-page-format.md)
- **Context**: ADR-006 defined on-disk page artifacts as clean HTML, raw HTML, assets, and metadata. Flat text extraction (`doc.Text()` on HTML) merges words across block boundaries (`definitionNo`), pollutes search with macro noise, and produces unreadable `pages.content` for MCP/REST. ADR-006 rejected “Markdown only” because it is not directly browsable in a browser. We still need offline HTML viewing **and** structured text for search and agents.

- **Decision**: Extend the per-page directory layout with **`content.md`** — structured Markdown derived from clean HTML (v1) or Confluence storage format (v2). Keep all ADR-006 artifacts unchanged.

  Each page directory contains:

  - `index.html` — clean extracted content with rewritten URLs and downloaded assets (unchanged)
  - `raw.html` — original Confluence storage XHTML or rendered HTML (unchanged)
  - **`content.md`** — searchable/readable Markdown (headers, bold/italic, tables, links; “good enough” fidelity)
  - `assets/images/` and `assets/attachments/` — downloaded media (unchanged)
  - `metadata.json` — page metadata (unchanged)

  **Database:** `pages.content` stores the same Markdown text as `content.md` (single source for FTS indexing). Path to `content.md` is derivable as `{file_dir}/content.md`; no separate API field in v1.

  **Regeneration:** `spacemosquito reindex --content` re-walks saved HTML and updates `content.md` + `pages.content` + FTS for existing installs.

- **Rationale**:
  - **Hybrid model** keeps ADR-006’s offline browser story (`index.html`) while fixing tokenization and readability.
  - Markdown enforces block boundaries (paragraphs, headings, table cells) that flat `doc.Text()` destroys.
  - One string field (`pages.content`) avoids schema churn; FTS triggers stay unchanged.
  - `content.md` on disk enables debugging, diffing, and tooling without parsing HTML.
  - HTML→Markdown library approach (approved) is faster to ship than a full Confluence storage parser; storage-aware conversion can follow in v2.

- **Alternatives considered**:
  - **Keep flat text only** — rejected; root cause of merged words and poor MCP content.
  - **Markdown only (replace HTML)** — rejected; loses direct offline browsing without a Markdown viewer.
  - **Separate `content_md` DB column** — deferred; `pages.content` semantics change is sufficient v1.
  - **Expose `markdown_path` in REST/MCP** — deferred; convention under `file_dir` is enough v1.

- **Consequences**:
  - Implementation in **`DOCS/task-content-markdown.md`** (converter package, scraper/import wiring, tests).
  - `import_saved` and new crawls must write `content.md` and upsert Markdown into `pages.content`.
  - Existing catalogs need one-time `reindex --content` after upgrade.
  - ADR-006 remains in the repo for history but is **superseded** by this ADR for current behavior.
  - Search and MCP consumers should treat `content` as Markdown (plain text with lightweight markup), not HTML.

- **Related**: `DOCS/task-content-markdown.md`, `DOCS/task-import-saved.md`, ADR-006 (superseded), ADR-010
