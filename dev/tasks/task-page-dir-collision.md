# Task: Make per-page directories collision-proof (prefix Confluence ID)

## Problem

On-disk page directories are named from the **sanitized, truncated title**
([writer.go `MakePageDir`](../../spacemosquito/internal/storage/writer.go#L46)):

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

Name the directory `{confluenceID}-{sanitized-title}`, truncating the **title
part** after the ID so the ID is never cut:

```
{basePath}/{spaceKey}/{confluenceID}-{sanitizeFilename(title, cap=100)}
```

`confluence_id` is unique per space → directories are collision-free, and the
title suffix keeps them recognizable. Example:
`saved/SPACE/542576204-Test Report Long Title…`.

Alternatives considered:
- **ID-only dir** (`{confluenceID}`) — simplest and safe, but loses
  browsability. *Rejected for readability.*
- **Collision suffix on demand** (`title`, then `title-2`, …) — non-deterministic
  across crawls (a page's dir could change if crawl order changes). *Rejected.*
- **`{title}-{id}`** — also unique; rejected in favor of ID-first for stable
  sort/prefix by Confluence ID.

### Code changes

- **`MakePageDir(spaceKey, pageTitle string, confluenceID int)`**
  ([writer.go:46](../../spacemosquito/internal/storage/writer.go#L46)) — new param;
  build `{id}-{title}`. Truncate title to 100; ID is always the full prefix.
- **`sanitizeFilename`** — keep as-is (still folds unsafe chars); the ID prefix
  is added by `MakePageDir`.
- **Callers:**
  - `savePageMetadata` ([scraper.go:393](../../spacemosquito/internal/scraper/scraper.go#L393))
    — pass `pg.ConfluenceID`. (Primary path.)
  - `runSave` ([run.go:362](../../spacemosquito/internal/cliapp/run.go#L362)) — a
    stub with placeholder title/space and no ID; pass `0` or leave clearly
    marked as a dev stub.
- **`GetSavedPath`** ([writer.go:148](../../spacemosquito/internal/storage/writer.go#L148))
  — **delete** (prod-dead) + its test.

### Not affected

- **DB schema** — `file_dir` already stores the full path per row; no migration
  of the schema needed.
- **`bootstrap import-saved`** — derives `fileDir` by walking existing
  `metadata.json` files (`filepath.Dir(metaPath)`,
  [import_saved.go:195](../../spacemosquito/internal/bootstrap/import_saved.go#L195)),
  so it reads whatever dir names exist (old or new). No change.
- **`reindex --content`** — resolves via the stored `file_dir`, so existing
  pages keep working without re-crawl.

## Migration concern (existing catalogs)

Existing installs have DB rows + on-disk dirs under the **old title-only**
scheme. After the change:

- **Existing pages still work** — `reindex`/`get-page` use the stored
  `file_dir`, which still points at the old dir.
- **Re-crawling a page** now creates a **new** `{id}-{title}` dir and updates the
  row's `file_dir` (upsert sets `file_dir=excluded.file_dir`); the **old
  title-only dir is orphaned** on disk (harmless cruft, but duplicated bytes).
- **Already-corrupted pages** (two pages that previously shared a dir) can't be
  recovered by any migration — the overwritten files are gone. Only a **re-crawl**
  restores both. Call this out in the changelog.

Options (locked — see Decisions):
1. **No auto-migration.** New scheme applies going forward; document that a
   fresh crawl re-lays-out under `{id}-{title}` and that stale old-scheme dirs
   are left on disk (no cleanup command in v1).
2. ~~Optional `migrate-page-dirs` helper~~ — deferred / out of scope for v1.

## Testing

- `MakePageDir` returns `…/{id}-{title}` and is **unique** for two pages with
  identical (or truncation-colliding) titles but different IDs → two distinct
  dirs, no overwrite.
- Title > 100 chars: full ID prefix + truncated title (ID never cut).
- Char-folding titles (`A/B` vs `A:B`) with different IDs → distinct dirs.
- Existing scraper/storage tests updated for the new signature; `reindex` test
  (stored `file_dir`) stays green.

## Non-goals

- Recovering data already overwritten by past collisions (requires re-crawl).
- Changing the DB schema or the `file_dir` contract (still a full path string).
- Renaming the space-level directory layout.

## Decisions

| # | Topic | Status | Decision |
|---|--------|--------|----------|
| 1 | Dir scheme | locked | `{id}-{title}` — `{basePath}/{spaceKey}/{confluenceID}-{sanitizeFilename(title, cap=100)}` |
| 2 | Migration | locked | **Document-only** — no `migrate-page-dirs` helper in v1; fresh crawl re-lays-out under the new scheme. |
| 3 | Orphaned old dirs | locked | **Leave them** — no cleanup step/command in v1. |
| 4 | `GetSavedPath` | locked | **Delete** (prod-dead) + its test. |

## Open questions

_None — all locked above._
