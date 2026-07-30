# Task: Make per-page directories collision-proof (append Confluence ID)

## Problem

On-disk page directories are named from the **sanitized, truncated title**
([writer.go `MakePageDir`](../spacemosquito/internal/storage/writer.go#L46)):

```
{basePath}/{spaceKey}/{sanitizeFilename(title)}
```

`sanitizeFilename` folds `/ \ :` → `-` and **hard-truncates to 100 chars**.
Confluence guarantees titles are unique *within a space*, but this naming is
lossy, so two genuinely-different pages can map to the **same directory**:

- **Truncation** — titles sharing their first 100 chars (common with long
  "Test Report …" titles) collide.
- **Character folding** — titles differing only by `/` vs `\` vs `:` vs `-`
  collide.

`MakePageDir` uses `os.MkdirAll` (no-op if the dir exists), so collisions are
**silent**. Consequences:

| Layer | Effect |
|---|---|
| DB rows | Fine — keyed `UNIQUE(space_id, confluence_id)`, both rows stored |
| `pages.content` / search / MCP | Fine — content is per-row in the DB |
| On-disk `index.html`/`raw.html`/`content.md`/`metadata.json`/`assets/` | **Overwritten** — last-writer-wins; both DB rows' `file_dir` point at one dir |
| `reindex --content` | **Cross-contamination** — both rows resolve to the same `file_dir`, read the same `raw.html`, so both get identical content |

## Goal

Give every page a **unique directory regardless of title**, eliminating the
collision class. Keep directories human-readable/browsable.

## Design

Name the directory `{sanitized-title}-{confluenceID}`, truncating the **title
part** before appending the ID so the ID is never cut:

```
{basePath}/{spaceKey}/{sanitizeFilename(title, cap=100)}-{confluenceID}
```

`confluence_id` is unique per space → directories are collision-free, and the
title prefix keeps them recognizable. Example:
`saved/SPACE/Test Report Long Title…-542576204`.

Alternatives considered:
- **ID-only dir** (`{confluenceID}`) — simplest and safe, but loses
  browsability. *Rejected for readability.*
- **Collision suffix on demand** (`title`, then `title-2`, …) — non-deterministic
  across crawls (a page's dir could change if crawl order changes). *Rejected.*

### Code changes

- **`MakePageDir(spaceKey, pageTitle string, confluenceID int)`**
  ([writer.go:46](../spacemosquito/internal/storage/writer.go#L46)) — new param;
  build the suffixed name. Truncate title to 100, then append `-{id}`.
- **`sanitizeFilename`** — keep as-is (still folds unsafe chars); the ID suffix
  is added by `MakePageDir`.
- **Callers:**
  - `savePageMetadata` ([scraper.go:393](../spacemosquito/internal/scraper/scraper.go#L393))
    — pass `pg.ConfluenceID`. (Primary path.)
  - `runSave` ([run.go:362](../spacemosquito/internal/cliapp/run.go#L362)) — a
    stub with placeholder title/space and no ID; pass `0` or leave clearly
    marked as a dev stub.
- **`GetSavedPath`** ([writer.go:148](../spacemosquito/internal/storage/writer.go#L148))
  — production-dead (only referenced by a test). Either update it to the new
  scheme (needs the ID) or delete it + its test. *Recommend delete.*

### Not affected

- **DB schema** — `file_dir` already stores the full path per row; no migration
  of the schema needed.
- **`bootstrap import-saved`** — derives `fileDir` by walking existing
  `metadata.json` files (`filepath.Dir(metaPath)`,
  [import_saved.go:195](../spacemosquito/internal/bootstrap/import_saved.go#L195)),
  so it reads whatever dir names exist (old or new). No change.
- **`reindex --content`** — resolves via the stored `file_dir`, so existing
  pages keep working without re-crawl.

## Migration concern (existing catalogs)

Existing installs have DB rows + on-disk dirs under the **old title-only**
scheme. After the change:

- **Existing pages still work** — `reindex`/`get-page` use the stored
  `file_dir`, which still points at the old dir.
- **Re-crawling a page** now creates a **new** `{title}-{id}` dir and updates the
  row's `file_dir` (upsert sets `file_dir=excluded.file_dir`); the **old
  title-only dir is orphaned** on disk (harmless cruft, but duplicated bytes).
- **Already-corrupted pages** (two pages that previously shared a dir) can't be
  recovered by any migration — the overwritten files are gone. Only a **re-crawl**
  restores both. Call this out in the changelog.

Options:
1. **No auto-migration (recommended for v1).** New scheme applies going forward;
   document that a fresh crawl re-lays-out under `{title}-{id}` and that stale
   old-scheme dirs can be deleted once a space is recrawled.
2. **Optional one-time `migrate-page-dirs` helper.** For each DB row whose
   `file_dir` is old-scheme, rename dir → `{title}-{id}` and update `file_dir`.
   Skip/flag rows whose target already exists (the collision victims) for manual
   re-crawl. More work; do only if orphaned dirs are a real concern.

## Testing

- `MakePageDir` returns `…/{title}-{id}` and is **unique** for two pages with
  identical (or truncation-colliding) titles but different IDs → two distinct
  dirs, no overwrite.
- Title > 100 chars: truncated title + full ID suffix (ID never cut).
- Char-folding titles (`A/B` vs `A:B`) with different IDs → distinct dirs.
- Existing scraper/storage tests updated for the new signature; `reindex` test
  (stored `file_dir`) stays green.

## Non-goals

- Recovering data already overwritten by past collisions (requires re-crawl).
- Changing the DB schema or the `file_dir` contract (still a full path string).
- Renaming the space-level directory layout.

## Open questions

1. **Dir scheme** — `{title}-{id}` (recommended) vs `{id}`-only?
2. **Migration** — ship the optional `migrate-page-dirs` helper, or document
   manual recrawl only? *Recommend document-only for v1.*
3. **Orphaned old dirs** — leave them, or add a cleanup step/command?
4. **`GetSavedPath`** — delete (recommended, prod-dead) or keep and update?
