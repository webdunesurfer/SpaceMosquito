# Extension — catalog UI (search / pages)

- **Task ID:** `extension-catalog-ui`
- **Status:** backlog
- **Parent:** [`extension-redesign`](extension-redesign.md)
- **Blocked by:** [`extension-redesign-mockups`](extension-redesign-mockups.md), [`extension-spaces-cron`](extension-spaces-cron.md)
- **Notes:** Optional phase — cancel if mockups defer catalog out of the extension

## Problem

Search, get-page, and page listing exist via REST/MCP/CLI but not in the
extension. Users may want a quick lookup without leaving the browser.

## Goal

If in scope: integrated **search**, **get page by id**, and/or **page list** for
a selected space inside the extension (or a clear link-out). If out of scope:
cancel this child after mockups.

## Decisions

| # | Topic | Status | Decision |
|---|--------|--------|----------|
| 1 | In popup vs link-out | open | Full UI vs “Open local UI” / docs only |
| 2 | Scope | open | Search only; or search + page detail; or list pages |

## Non-goals

- Editing Confluence content
- Replacing MCP for agents

## Implementation

Depends on mockups. Reuse existing REST: `/api/search`, `/api/pages/…`, space pages list.

## Done when

- [ ] Either: catalog flows work in both browsers with gating, or
- [ ] Task **cancelled** with parent epic noting deferral
