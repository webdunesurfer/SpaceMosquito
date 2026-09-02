# Cron: use stored page browse URL (remove tenant hardcode)

- **Task ID:** `self-hosted-cron-webui`
- **Status:** done
- **Parent:** [`self-hosted-confluence`](self-hosted-confluence.md)

## Problem

Incremental cron builds page URLs with a hardcoded Cloud tenant:

```go
// internal/cron/scheduler.go
pageURL := fmt.Sprintf("https://teamnetconomy.atlassian.net/wiki/spaces/%s/pages/%d", ...)
```

That breaks self-hosted and any non-that-tenant Cloud for DOM change checks.
`ScrapePageAPI` already receives a correct `spaceURL`; only the reconstructed
`pageURL` is wrong.

## Goal

Cron uses each page’s **real browse URL** (webui / `confluence_url`), never a
hardcoded host. Cloud and Server both work from stored data.

## Decisions

| # | Topic | Status | Decision |
|---|--------|--------|----------|
| 1 | URL source | locked | Prefer stored **webui / browse URL** (epic lock #5) |
| 2 | Persistence | locked | **A)** Read `metadata.json` → `confluence_url` via `file_dir` / `metadata_path` — **no** DB migration / `pages.url` column in v1. |

## Implementation

1. Remove `teamnetconomy.atlassian.net` from `internal/cron/scheduler.go`.
2. Resolve browse URL per page from on-disk `metadata.json` (`confluence_url`)
   using `file_dir` / `metadata_path`. Fallback only if missing: derive from
   space `url` + page id (document as degraded).
3. Use that URL for DOM change detection and any scrape path that needs a page URL.
4. Tests: mock/list pages with custom-host `confluence_url` in metadata; assert
   no hardcode host.

## Done when

- [x] No hardcoded tenant hostname in cron
- [x] Incremental path uses stored browse URL when present
- [x] Unit test covers custom-domain page URL
- [x] CHANGELOG `[Unreleased]` notes the fix

## Non-goals

- Full context-path auto-detect (see [`self-hosted-context-path`](self-hosted-context-path.md))
- Extension cookie changes
