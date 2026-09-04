# Extension redesign — mockups & IA

- **Task ID:** `extension-redesign-mockups`
- **Status:** ready
- **Parent:** [`extension-redesign`](extension-redesign.md)
- **Blocked by:** — (can start before multi-wiki code; design **for** multi-session)

## Problem

The popup grew organically (Session / Crawl / Settings tabs, separate capture vs
validate, cron tucked under settings, weak backend/session gating). Jumping
straight into code risks another incremental patch without a clear IA.

## Goal

Agree **information architecture** and **key screens** before implementation
children ship UI. Output is reviewable mockups the agent can build against.

## Decisions

| # | Topic | Status | Decision |
|---|--------|--------|----------|
| 1 | Artifact format | open | Markdown wireframes + PNG; static HTML mock; or Figma/Penpot export into this task folder |
| 2 | Primary surfaces | open | Which tabs/sections: e.g. Overview, Sessions, Spaces, Activity, Catalog, Settings |
| 3 | Multi-session presentation | open | List of hosts with status dots vs “current tab’s site” emphasis + expandable others |
| 4 | Catalog in v1 | open | Include in mockups as phase-2 / grayed “later”, or omit |

## Implementation (design work)

1. Inventory current popup affordances (capture, validate, delete, crawl, spaces, cron, settings).
2. Propose IA that covers epic wishes (session, backend gate, spaces+cron, multi-crawl, optional catalog).
3. Produce mockups for at least:
   - Backend down / session invalid (disabled states)
   - Healthy: sessions list + capture+validate primary action
   - Spaces list with cron on/off + actions
   - Concurrent crawl activity
   - Settings (backend URL, auto-renew checkbox)
4. Lock epic open decisions that mockups settle (#4–#8 in parent) or record remaining opens.
5. Store artifacts under `dev/tasks/extension-redesign-mockups/` **or** embed in this file; link from parent epic.

## Done when

- [ ] IA written and accepted
- [ ] Key screens mock’d and linked from this doc
- [ ] Parent epic open decisions updated (locked or explicitly deferred)
- [ ] Implementation children can start without guessing layout

## Notes

Popup is ~narrow; prefer vertical stacks over dense dashboards. Match existing
brand loosely; redesign is allowed but keep extension constraints (no huge
assets, works in Firefox + Chrome popup).
