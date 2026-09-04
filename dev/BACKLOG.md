# Backlog

Flat task list. Status values: `backlog` | `ready` | `in_progress` | `blocked` | `done` | `cancelled`.

On cleanup, move shipped work to [`CHANGELOG.md`](../CHANGELOG.md) and clean the backlog — see [`CLEANUP.md`](CLEANUP.md). On version cut, see [`RELEASE.md`](RELEASE.md).

Reorder freely. Cancel by setting status to `cancelled`. Unexpected work becomes a new task with its own ID.

## Format

Each task:

```markdown
## <task-id>
- status: backlog
- parent: <optional-related-task-id>
- blocked_by: <optional-task-id>
- goal: One sentence
- done_when: Observable acceptance criteria
- notes: Optional one-liner
```

Detail lives in `dev/tasks/<task-id>.md` when needed (no `task-` filename prefix).

---

## cme-markdown-gap-analysis
- status: ready
- goal: Extract confluence-markdown-exporter conversion behaviors (tests + Converter) and gap-analyze vs our CSF→Markdown rules; recommend follow-ups only.
- done_when: CME capability inventory, ours crosswalk, severity-rated gap matrix, and ≤10 ranked implement/defer/ignore recommendations; no port in this task.
- notes: Detail in [`tasks/cme-markdown-gap-analysis.md`](tasks/cme-markdown-gap-analysis.md).

## self-hosted-confluence
- status: done
- goal: Epic — self-hosted / custom-domain Confluence end-to-end (not only `*.atlassian.net`).
- done_when: Children shipped; capture/validate/crawl on `wiki.example.com`; no hardcoded tenant in cron; Cloud unchanged.
- notes: Done; detail in [`tasks/self-hosted-confluence.md`](tasks/self-hosted-confluence.md). Optional follow-up ADR still suggested there.

## self-hosted-cron-webui
- status: done
- parent: self-hosted-confluence
- goal: Remove cron tenant hardcode; use stored page browse/webui URL for incremental checks.
- done_when: No `teamnetconomy` (or any fixed host) in scheduler; custom-domain URL covered by test; CHANGELOG note.
- notes: Shipped in tree; detail in [`tasks/self-hosted-cron-webui.md`](tasks/self-hosted-cron-webui.md).

## self-hosted-context-path
- status: done
- parent: self-hosted-confluence
- goal: Auto-detect Confluence context path in base URL; fix space auto-create fallback host.
- done_when: `/confluence/display/KEY` base includes context path; no `example.atlassian.net` fallback; Cloud regression OK.
- notes: Shipped in tree; detail in [`tasks/self-hosted-context-path.md`](tasks/self-hosted-context-path.md).

## self-hosted-extension-cookies
- status: done
- parent: self-hosted-confluence
- goal: Hostname-only Firefox cookie capture; Chrome Server cookie filter; neutral popup placeholders.
- done_when: No parent-domain cookie scrape; Chrome filter includes Server names; placeholders not Atlassian-only.
- notes: Shipped in tree; detail in [`tasks/self-hosted-extension-cookies.md`](tasks/self-hosted-extension-cookies.md). Content scripts stay unregistered.
