# Extensions: self-hosted cookie + UX polish

- **Task ID:** `self-hosted-extension-cookies`
- **Status:** done
- **Parent:** [`self-hosted-confluence`](self-hosted-confluence.md)

## Problem

Background URL detection largely works on custom hosts, but:

- Firefox cookie capture may still query **parent** domains (conflicts with
  hostname-only lock).
- Chrome cookie name filter lacks common Server/DC names (`jsessionid`,
  `seraph`, `crowd`, …).
- Popup placeholders still show `tenant.atlassian.net`.
- Content `space-detector.ts` is Cloud-only — leave it; content scripts stay
  **unregistered** (epic lock #8).

## Goal

Hostname-only cookie capture on both browsers; Server-friendly cookie filters;
neutral placeholders. Keep duplicated helpers (epic lock #3). Keep client
cookie peek in `auth.ts` (epic lock #7).

## Decisions

| # | Topic | Status | Decision |
|---|--------|--------|----------|
| 1 | Cookie domain | locked | Hostname only |
| 2 | Code sharing | locked | Duplicate across extensions |
| 3 | Content scripts | locked | Stay unregistered |
| 4 | auth peek | locked | Keep |

## Implementation

| File | Change |
|------|--------|
| `firefox-extension/lib/session.ts` | Capture cookies for tab hostname only (drop parent-domain scrape) |
| `chrome-extension/lib/session.ts` | Add Server/DC cookie name patterns to match Firefox |
| `firefox-extension/popup/popup.html` | Neutral placeholder (`wiki.example.com/...`) |
| `chrome-extension/popup/popup.html` | Same |
| `*/content/space-detector.ts` | **No change** unless registering content scripts later |

## Done when

- [x] Firefox does not call `cookies.getAll` for parent domains
- [x] Chrome filter includes Server/DC session cookie names
- [x] Placeholders are not Atlassian-only
- [x] Manual smoke: capture on custom-host tab still works (checklist)

### Manual smoke checklist

1. Open a Confluence tab on a custom host (e.g. `wiki.example.com`), log in.
2. Capture session from the extension popup — expect non-zero cookies, no parent-domain scrape in logs.
3. Validate session against the backend.
4. Add/crawl a space URL on that host (`/spaces/…` or `/display/…` / `/confluence/…`).

## Non-goals

- `shared/` package
- Registering `content_scripts`
- Backend context path / cron
