# Backlog

Flat task list. Status values: `backlog` | `ready` | `in_progress` | `blocked` | `cancelled`.

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

## task-asset-skip-and-force
- status: ready
- goal: Skip already-downloaded crawl assets before the HTTP GET; `crawl --force` re-scrapes all pages and re-downloads all assets.
- done_when: Existing non-empty assets are not re-fetched (no GET / rate-limit wait); `--force` bypasses page version skip and asset skip; unit tests cover skip/force/zero-byte/page-force.
- notes: **Implemented** (awaiting release). Detail in [`tasks/task-asset-skip-and-force.md`](tasks/task-asset-skip-and-force.md).

## task-page-dir-collision
- status: ready
- goal: Make per-page `saved/` dirs unique by appending Confluence ID (`{title}-{id}`).
- done_when: Truncation/char-fold collisions no longer overwrite on-disk files; existing `file_dir` rows still work; tests cover colliding titles.
- notes: Detail in [`tasks/task-page-dir-collision.md`](tasks/task-page-dir-collision.md). Post-0.1.0; no auto-migration of old dirs in v1.

## task-validation-sso-fix
- status: backlog
- goal: Stop false-positive “authenticated” when Confluence/SSO returns HTML via redirect.
- done_when: `ValidateWithConfluence` fails closed on redirects, non-JSON bodies, and JSON decode errors.
- notes: Detail in [`tasks/task-validation-sso-fix.md`](tasks/task-validation-sso-fix.md).

## task-self-hosted-confluence
- status: backlog
- goal: End-to-end support for self-hosted / custom-domain Confluence (not only `*.atlassian.net`).
- done_when: Firefox+Chrome capture/validate/crawl work on `wiki.example.com`; no hardcoded tenant URLs in extension/cron; Cloud path unchanged.
- notes: Detail in [`tasks/task-self-hosted-confluence.md`](tasks/task-self-hosted-confluence.md). Backend partial; Firefox + cron still block.
