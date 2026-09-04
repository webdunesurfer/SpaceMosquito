# Extension — multi-space crawl status

- **Task ID:** `extension-crawl-status`
- **Status:** backlog
- **Parent:** [`extension-redesign`](extension-redesign.md)
- **Blocked by:** [`extension-redesign-mockups`](extension-redesign-mockups.md), [`extension-spaces-cron`](extension-spaces-cron.md)

## Problem

Crawl progress UI is oriented around one active job. Users may run (or cron may
trigger) **multiple** space crawls; the popup should show which spaces are
running and allow sensible cancel/cleanup.

## Goal

Display **all active (and recent) crawl jobs** — possibly several in parallel —
with status per space and cancel where supported by the API.

## Decisions

| # | Topic | Status | Decision |
|---|--------|--------|----------|
| 1 | Data source | open | Poll `/api/crawl/status` / list jobs; any API gaps → small backend follow-up |
| 2 | History | open | Running only vs include last completed/failed |
| 3 | Cancel | open | Per-job cancel only (preferred) |

## Implementation

1. Confirm backend can list concurrent jobs; extend API if missing (note in task).
2. Activity UI: list jobs with space key, %, pages, error, cancel.
3. Poll while popup open; stop when idle.
4. Firefox + Chrome.

## Done when

- [ ] Two concurrent crawls visible as two entries
- [ ] Cancel works per job when API allows
- [ ] Gating when backend down
- [ ] CHANGELOG if user-facing
