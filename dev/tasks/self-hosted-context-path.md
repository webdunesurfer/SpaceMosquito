# Backend: auto-detect Confluence context path

- **Task ID:** `self-hosted-context-path`
- **Status:** done
- **Parent:** [`self-hosted-confluence`](self-hosted-confluence.md)
- **Blocked by:** optional — can land before or after cron webui

## Problem

`confluence.BaseURL` / session root helpers return only `scheme://host`. Installs
like `https://wiki.mycompany.com/confluence/display/PROJ` need API base
`https://wiki.mycompany.com/confluence`.

Also, space auto-create still falls back to
`https://example.atlassian.net/wiki/spaces/{key}` when `spaceURL` is empty.

## Goal

Auto-detect context path from the Confluence URL (epic lock #1). Fix space
URL fallback to derive from the crawl/session URL, not a fake Cloud host.

## Decisions

| # | Topic | Status | Decision |
|---|--------|--------|----------|
| 1 | Detection | locked | Auto-detect path segments before `/display/`, `/spaces/`, `/wiki/` |
| 2 | Config override | locked | **No** required `confluence.base_path` in v1 |
| 3 | Cloud custom domain | locked | API on same hostname (epic lock #6) |

## Implementation

1. Enhance shared Go base-URL helper (`internal/confluence` and/or session root)
   so context path is preserved.
2. Point session validation + scraper discovery/API calls at that base.
3. Replace `example.atlassian.net` auto-create fallback with derivation from
   `spaceURL` / session confluence URL.
4. Tests: `/confluence/display/KEY` → base includes `/confluence`; root `/display/KEY`
   → host only; Cloud `/wiki/spaces/KEY` unchanged.

## Done when

- [x] Context path auto-detected in unit tests
- [x] No `example.atlassian.net` fallback for space create
- [x] Cloud regression tests still pass
- [x] CHANGELOG / README mention `/confluence/` installs if user-facing

## Non-goals

- Extension changes
- Cron hardcode (sibling task) — but cron benefits once space URLs are correct
