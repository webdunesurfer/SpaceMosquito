# Extension — spaces list & cron indicators

- **Task ID:** `extension-spaces-cron`
- **Status:** done
- **Parent:** [`extension-redesign`](extension-redesign.md)
- **Blocked by:** [`extension-redesign-mockups`](extension-redesign-mockups.md), [`extension-session-ux`](extension-session-ux.md)

## Goal

**Spaces** tab: current space first, then A–Z; compact rows with crawl/cron;
expandable cron panel with **autosave**; crawl-all at bottom — per
[`WIREFRAMES.md`](extension-redesign-mockups/WIREFRAMES.md).
Inline crawl progress is owned by [`extension-crawl-status`](extension-crawl-status.md).

## Shipped

- Compact rows: name (URL tooltip), `crawled/total` via `pages_crawled` + `pages_total`, last crawl date, play, clock.
- Current tab space first (synthetic empty row if not in catalog); rest A–Z.
- Cron expand: Full / Incremental / Detection / Enabled (both flags); autosave + debounced `POST /api/cron/reload`.
- `pages_total` column + `UpdateSpacePagesTotal` after crawl discovery.
- Gating with backend + session; Firefox + Chrome.

## Done when

- [x] Z1–Z6 done
- [x] Both browsers; CHANGELOG note
