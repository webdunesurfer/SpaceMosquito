# Extension — Spaces CSF / HTML tooltip

- **Task ID:** `extension-spaces-body-format-tooltip`
- **Status:** done
- **Parent:** —
- **Blocked by:** —

## Goal

On the Spaces row counts (`X / Y`), tooltip: `Stored: N CSF · M HTML`.

## Shipped

- Migration `010_pages_body_format`: `pages.body_format` (`storage` / `rendered`).
- Set on crawl/save and import; startup backfill via `DetectBodyFormat` for empty rows.
- `GET /api/spaces`: `pages_storage` + `pages_rendered` (unknown → rendered).
- Popup tooltip on `.space-row-counts` when `pages_crawled > 0` (always both sides).

## Decisions (locked)

| # | Decision |
|---|----------|
| 4 | Unknown → HTML |
| 5 | List API counts |
| 6 | Persist column + SQL aggregate; no per-list FS scan |
| 8 | Always show both CSF and HTML counts when crawled > 0 |

## Done when

- [x] Hovering `X / Y` shows `Stored: N CSF · M HTML`
- [x] Labels match Page tab
- [x] Both browsers; CHANGELOG
