# Extension — session UX

- **Task ID:** `extension-session-ux`
- **Status:** backlog
- **Parent:** [`extension-redesign`](extension-redesign.md)
- **Blocked by:** — (multi-wiki sessions shipped in 0.3.4); start after [`extension-redesign-mockups`](extension-redesign-mockups.md); preferably after [`extension-backend-status`](extension-backend-status.md)

## Problem

Capture and Validate are separate buttons; session info is thin; invalid session
does not consistently disable crawl/spaces actions. No auto-renew. Multi-wiki
will require listing multiple sites — current UI assumes one session.

## Goal

Simple session management: **capture+validate in one action**, show status/info
per site, delete, optional auto-renew, and **gray-out** actions that need a
valid session (coordinated with backend gating).

## Decisions

| # | Topic | Status | Decision |
|---|--------|--------|----------|
| 1 | One-button flow | open | Capture cookies from tab → save (upsert) → validate against Confluence; surface combined success/failure |
| 2 | Validity refresh | open | Inherit epic decision (poll / on open / hybrid) |
| 3 | Auto-renew | open | Settings checkbox; only renew when matching Confluence tab is open and cookies readable |
| 4 | Gating rules | open | Which controls require valid session for **current tab’s site** vs any site |

## Non-goals

- Changing cookie capture filters (already hostname-only)
- Backend multi-session storage (parent dependency)

## Implementation

Per locked mockups + multi-wiki API:

1. Primary CTA: capture+validate (replace two buttons).
2. Session panel: per-site status, cookie count / captured-at / last validated / flavor if known; delete per site.
3. Apply disabled/gray styles when session for relevant site is missing/invalid.
4. Optional auto-renew via extension alarm / idle callback when checkbox on.
5. Duplicate changes in Firefox and Chrome.

## Done when

- [ ] One primary capture+validate action works end-to-end
- [ ] Multi-site session info + delete
- [ ] Dependent UI disabled when session invalid (per gating rules)
- [ ] Auto-renew checkbox behaves per locked decision (or explicitly deferred)
- [ ] Both browsers updated; CHANGELOG note
