# Extension — spaces list & cron indicators

- **Task ID:** `extension-spaces-cron`
- **Status:** backlog
- **Parent:** [`extension-redesign`](extension-redesign.md)
- **Blocked by:** [`extension-redesign-mockups`](extension-redesign-mockups.md), [`extension-session-ux`](extension-session-ux.md)

## Problem

Spaces live under Settings; cron is a separate dense block. Hard to see at a
glance whether a space has cron enabled. Actions (crawl, manage cron, remove)
are cramped.

## Goal

A clear **spaces list** with typical actions and an obvious **cron on/off**
(or schedule summary) per space. Align with multi-wiki (space → site session).

## Decisions

| # | Topic | Status | Decision |
|---|--------|--------|----------|
| 1 | Cron display | open | Badge “cron on/off”; next-run hint; or interval string |
| 2 | Actions | open | Crawl, cancel?, cron edit, remove — confirm set from mockups |
| 3 | Placement | open | Own tab/section vs under Overview (from IA) |

## Implementation

1. Rebuild list UI per mockups; show cron enablement from existing cron config API.
2. Gate actions on backend + session for that space’s host.
3. Keep cron edit flow usable (inline or sheet) without the current clutter.
4. Firefox + Chrome.

## Done when

- [ ] Each listed space shows whether cron is enabled
- [ ] Core actions work and respect gating
- [ ] Both browsers; CHANGELOG if user-facing
