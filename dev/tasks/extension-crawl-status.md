# Extension — multi-space crawl status (inline on Spaces)

- **Task ID:** `extension-crawl-status`
- **Status:** done
- **Parent:** [`extension-redesign`](extension-redesign.md)
- **Blocked by:** [`extension-redesign-mockups`](extension-redesign-mockups.md), [`extension-spaces-cron`](extension-spaces-cron.md)

## Goal

**Inline** on Spaces rows: play → expand dark-red progress; stop cancels;
multiple rows may crawl in parallel — per wireframes.

## Shipped

- Play starts crawl and expands per-row progress; play → stop while running.
- Poll `GET /api/crawl` every 2s while popup open; concurrent jobs independent.
- Stop → `POST /api/crawl/cancel`; complete/fail collapses and refreshes space counts.
- Gating when backend down / session invalid.
- Firefox + Chrome; CHANGELOG `[Unreleased]`.

## Done when

- [x] C1–C6 satisfied
- [x] CHANGELOG note
