# Extension redesign — mockups & IA

- **Task ID:** `extension-redesign-mockups`
- **Status:** done
- **Parent:** [`extension-redesign`](extension-redesign.md)

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
| 1 | Artifact format | locked | **Markdown wireframes + PNG** under [`extension-redesign-mockups/`](extension-redesign-mockups/) |
| 2 | Primary surfaces | locked | **Page · Spaces · Settings**; header **Session** disc only. Popup only. |
| 3 | Multi-session / context | locked | Header Session disc = **current tab’s host**; Page tab shows that page; Spaces lists current space first |
| 4 | Catalog in v1 | locked | **Omit** — [`extension-catalog-ui`](extension-catalog-ui.md) cancelled |
| 5 | Crawl progress | locked | **Inline on Spaces** (expand row); no Activity tab; dark-red progress; play/stop |
| 6 | Cron UI | locked | Expand on space row; **autosave**; independent of crawl expand |
| 7 | Page meta row | locked | Live + Stored version/date + **sync** refresh icon on **one line** |
| 8 | Backend UX | locked | No header Backend chip; banner when down; **poll every 2s while popup open**, no poll when closed; no click-to-recheck |

## Artifacts

**[`extension-redesign-mockups/WIREFRAMES.md`](extension-redesign-mockups/WIREFRAMES.md)** — ASCII + PNGs 01–07 (revision 3).

## Done when

- [x] IA written and accepted
- [x] Key screens mock’d and linked from this doc
- [x] Parent epic open decisions updated (locked or explicitly deferred)
- [x] Implementation children can start without guessing layout

## Notes

Revision 3: compact Page meta + sync icon; Session-only header; backend banner + 2s popup-only poll.
