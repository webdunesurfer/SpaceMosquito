# Backlog

Flat task list. Status values: `backlog` | `ready` | `in_progress` | `blocked` | `done` | `cancelled`.

On release, move shipped work to [`CHANGELOG.md`](../CHANGELOG.md) and clean the backlog — see [`RELEASE.md`](RELEASE.md).

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

Detail lives in `dev/tasks/<task-id>.md` when needed.

---

## task-cme-markdown-gap-analysis
- status: ready
- goal: Extract confluence-markdown-exporter conversion behaviors (tests + Converter) and gap-analyze vs our CSF→Markdown rules; recommend follow-ups only.
- done_when: CME capability inventory, ours crosswalk, severity-rated gap matrix, and ≤10 ranked implement/defer/ignore recommendations; no port in this task.
- notes: Detail in [`tasks/task-cme-markdown-gap-analysis.md`](tasks/task-cme-markdown-gap-analysis.md). CME is HTML+markdownify (MIT); ours is CSF — map intent, not drop-in rules.

## task-page-dir-collision
- status: ready
- goal: Make per-page `saved/` dirs unique by appending Confluence ID (`{title}-{id}`).
- done_when: Truncation/char-fold collisions no longer overwrite on-disk files; existing `file_dir` rows still work; tests cover colliding titles.
- notes: Detail in [`tasks/task-page-dir-collision.md`](tasks/task-page-dir-collision.md). No auto-migration of old dirs in v1.

## task-self-hosted-confluence
- status: backlog
- goal: End-to-end support for self-hosted / custom-domain Confluence (not only `*.atlassian.net`).
- done_when: Firefox+Chrome capture/validate/crawl work on `wiki.example.com`; no hardcoded tenant URLs in extension/cron; Cloud path unchanged.
- notes: Detail in [`tasks/task-self-hosted-confluence.md`](tasks/task-self-hosted-confluence.md). Backend partial; Firefox + cron still block.
