# Session — stale-green indicator vs real auth failure

- **Task ID:** `session-stale-indicator`
- **Status:** done
- **Parent:** —
- **Blocked by:** —

## Problem

Session disc stayed **green** while Confluence auth was actually dead.
Crawl appeared stuck (~30 min, same progress). Manual click of the header
session chip did **not** recover. Recovery that worked:

1. Reload the wiki page in the browser (re-pass AD / issue fresh cookies)
2. Recapture session in the extension
3. Restart the crawl

## Shipped

| # | Fix | What landed |
|---|-----|-------------|
| A | Invalidate on auth failure | `session.ErrUnauthorized` + `ClearValidationForURL`; compare / refresh / crawl / cron clear `ValidatedAt` on 401/403 |
| B | TTL status | Keep cheap `GET /api/session/status`; **TTL = 60 min** (`session.ValidateTTL`) |
| C | Failed validate | `POST /api/session/validate` clears + persists `ValidatedAt` on failure so status cannot flip green |
| D | Crawl fail-fast | Job / `CrawlSpace` / cron abort with `session expired — recapture` (no pause state); no browser fallback on auth errors |
| E | Soft revalidate | Postponed (unchanged) |

Logging: session status decision + age; validate start/end; `session_unauthorized` / `crawl_aborted_session`; extension `capture-and-validate` + status flips.

Customer evidence file `tmplogs.jsonl` **deleted** after ship.

## Decisions (locked)

| # | Topic | Decision |
|---|--------|----------|
| 2 | Cheap status | Keep TTL status (no live probe on poll) |
| 3 | Validate TTL | **60 minutes** |
| 4 | Crawl on 401 | **Fail job** (no pause) |
| 5 | Soft revalidate | Postponed |
| 6 | tmplogs | Deleted after implement |

## Done when

- [x] A–D + logging
- [x] Tests (TTL, validate clears, unauthorized fetch)
- [x] Delete `tmplogs.jsonl`
- [x] CHANGELOG
