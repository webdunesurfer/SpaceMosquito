# Extension — session UX (+ Page tab)

- **Task ID:** `extension-session-ux`
- **Status:** done
- **Parent:** [`extension-redesign`](extension-redesign.md)
- **Blocked by:** — (multi-wiki sessions shipped in 0.3.4); start after [`extension-redesign-mockups`](extension-redesign-mockups.md); preferably after [`extension-backend-status`](extension-backend-status.md)

## Problem

Capture and Validate are separate buttons; session info is thin; invalid session
does not consistently disable crawl/spaces actions. No auto-renew. Page context
(live vs stored, one-page refresh) is missing from the popup.

## Goal

Header **Session** disc = capture+validate for the current tab’s host; gray-out
actions that need a valid session; Settings auto-renew; **Page** tab shows host,
title, live vs stored version/date, and one-page refresh — per
[`WIREFRAMES.md`](extension-redesign-mockups/WIREFRAMES.md).

## Decisions

| # | Topic | Status | Decision |
|---|--------|--------|----------|
| 1 | One-button flow | locked | Session disc click → capture cookies → `POST /api/session` → `POST /api/session/validate`; disc green/red from `GET /api/session/status?url=` |
| 2 | Validity refresh | locked | On popup open + after capture; no background Confluence poll in v1 (cheap `status` only) |
| 3 | Auto-renew | locked | Settings checkbox; renew only when a matching Confluence tab is open and cookies readable |
| 4 | Gating | locked | Refresh page + space crawls require valid session for **current tab’s host**; Session disc still clickable when red |
| 5 | Delete | locked | No delete in header (v1); Settings may add later |
| 6 | Page meta | locked | One line: Live · Stored · sync icon ([wireframes](extension-redesign-mockups/WIREFRAMES.md)) |

## Scenarios

(See history below for GWT text.)

## Shipped

- Popup shell: **Page · Spaces · Settings**; Session chip (capture+validate via `capture-and-validate`).
- Session + backend gating for crawl / refresh / cron mutate.
- Page tab: host, title, `GET /api/pages/{id}/compare`, sync → `POST /api/pages/{id}/refresh`.
- Settings: auto-renew checkbox; background renews open Confluence hosts every 30s when enabled.
- Firefox + Chrome; CHANGELOG `[Unreleased]`.

## Done when

- [x] S1–S7 done (S6/S7 via new compare + refresh APIs)
- [x] Both browsers; CHANGELOG note
