# Extension — live backend status

- **Task ID:** `extension-backend-status`
- **Status:** done
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
| 1 | Probe | locked | `GET /health` (expect body/`ok`); short timeout (e.g. 1–2s). Failures → down. |
| 2 | Refresh | locked | Poll every **2s while popup is open**; **no** polling when popup closed (popup `setInterval` dies with the popup) |
| 3 | Visual | locked | **No** header Backend chip; show unreachable **banner** with backend URL. Healthy = no status chrome. No click-to-recheck. |

## Scenarios

Convention: **Given / When / Then**. Tag `[needs-api]` only when a **new or changed backend** route/behavior is required before the Then can be true. None below need new backend — `/health` exists.

### B1 — Banner when unreachable

```gherkin
Given the popup is open and spacemosquito serve is stopped
When the probe runs (on open or next 2s tick)
Then GET /health fails (timeout/network/non-ok)
  and a content banner shows "Backend unreachable" with the configured URL
  and there is no Backend status chip in the header
```

### B2 — Healthy = no chrome

```gherkin
Given the popup is open and serve is running
When GET /health succeeds
Then no backend banner is shown
  and no Backend chip appears in the header
```

### B3 — Poll while open only

```gherkin
Given the popup is open and backend is down (banner visible)
When the user starts serve again and ~2s elapses with the popup still open
Then GET /health succeeds
  and the banner clears and API-dependent controls re-enable
```

```gherkin
Given the popup was open (polling) and the user dismisses the popup
When serve is stopped or restarted
Then no further GET /health calls are made from the closed popup
  (interval is cleared on unload; no background alarm for backend probe)
```

### B4 — Gate API actions when down

```gherkin
Given the popup shows the backend-unreachable banner
When the user views Page or Spaces
Then refresh/sync, Session disc capture, crawl play, cron edits, and crawl-all are disabled
  and Settings still allows editing/saving the backend URL (browser.storage)
```

### B5 — No flicker on normal open

```gherkin
Given serve is healthy
When the user opens the popup
Then controls are not briefly disabled then enabled in a visible flash
  (unknown→up may stay gated until first successful probe, or debounce; no red banner flash)
```

## Implementation

1. Reproduce current bugs; fix race/error handling in popup/API client.
2. Central `backendState`: unknown / up / down; banner when down (no header chip).
3. While popup open: poll `GET /health` every 2s; clear interval on unload.
4. Disable capture, crawl, spaces/cron mutations when down; allow backend URL edit.
5. Same behavior Firefox + Chrome.

## Done when

- [x] Scenarios B1–B5 satisfied
- [x] Status matches reality under kill/restart of `spacemosquito serve`
- [x] CHANGELOG note if user-facing

Shipped in extension popup (Firefox + Chrome): `ApiClient.checkHealth`, banner, 2s poll while open, API gating; backend URL remains editable when down.
