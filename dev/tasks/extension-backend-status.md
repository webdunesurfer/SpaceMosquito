# Extension — live backend status

- **Task ID:** `extension-backend-status`
- **Status:** backlog
- **Parent:** [`extension-redesign`](extension-redesign.md)
- **Blocked by:** [`extension-redesign-mockups`](extension-redesign-mockups.md)

## Problem

Backend availability indication is unreliable (“currently buggy”): false
negatives/positives, unclear when the popup should disable actions. Session and
crawl UIs depend on a reachable local API.

## Goal

**Live**, trustworthy backend status (available / unavailable), and disable
actions that need the API when it is down. Shared gating with session UX.

## Decisions

| # | Topic | Status | Decision |
|---|--------|--------|----------|
| 1 | Probe | open | Which endpoint (e.g. health/status/session)? Timeout? |
| 2 | Refresh | open | On popup open + interval while popup open; optional background |
| 3 | Visual | open | Header dot + short text; toast only on transition |

## Implementation

1. Reproduce current bugs; fix race/error handling in popup/API client.
2. Central `backendState`: unknown / up / down.
3. Disable capture (if it needs API), crawl, spaces CRUD, cron save, catalog when down; allow editing backend URL.
4. Same behavior Firefox + Chrome.

## Done when

- [ ] Status matches reality under kill/restart of `spacemosquito serve`
- [ ] Dependent controls disabled when unavailable
- [ ] No flaky flicker on normal open (debounce/hysteresis if needed)
- [ ] CHANGELOG note if user-facing
