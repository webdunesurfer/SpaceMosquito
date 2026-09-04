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

## session-enc-legacy-cleanup
- status: backlog
- parent: multi-wiki-sessions
- goal: After a release window, remove legacy single-session session.enc migration and Save/Load shims.
- done_when: v2-only blob; no bare Session file unmarshal; API requires host/URL; tests + CHANGELOG/ADR.
- notes: Detail in [`tasks/session-enc-legacy-cleanup.md`](tasks/session-enc-legacy-cleanup.md). Defer until multi-session builds have been in the wild. Parent task shipped in 0.3.4 (ADR-002).

## extension-redesign
- status: backlog
- goal: Epic — redesign Firefox/Chrome extensions (session, backend gate, spaces/cron, crawl status, optional catalog).
- done_when: Mockups locked; children shipped (catalog may defer); gating + multi-wiki aware; both browsers; CHANGELOG.
- notes: Detail in [`tasks/extension-redesign.md`](tasks/extension-redesign.md). Multi-wiki sessions shipped in 0.3.4. Start with [`extension-redesign-mockups`](tasks/extension-redesign-mockups.md).

## extension-redesign-mockups
- status: ready
- parent: extension-redesign
- goal: Lock IA and key popup screens before UI implementation.
- done_when: Accepted wireframes/mockups; epic open decisions updated; children unblocked on layout.
- notes: Detail in [`tasks/extension-redesign-mockups.md`](tasks/extension-redesign-mockups.md).

## extension-session-ux
- status: backlog
- parent: extension-redesign
- goal: One-button capture+validate; session info/delete; validity gating; optional auto-renew.
- done_when: Combined capture+validate; multi-site session UI; gray-out when invalid; both browsers.
- notes: Detail in [`tasks/extension-session-ux.md`](tasks/extension-session-ux.md).

## extension-backend-status
- status: backlog
- parent: extension-redesign
- goal: Live trustworthy backend available/unavailable; disable API-dependent UI when down.
- done_when: Status matches serve up/down; gating works; flicker fixed.
- notes: Detail in [`tasks/extension-backend-status.md`](tasks/extension-backend-status.md).

## extension-spaces-cron
- status: backlog
- parent: extension-redesign
- goal: Spaces list with cron on/off and clearer actions.
- done_when: Cron enablement visible per space; actions + gating; both browsers.
- notes: Detail in [`tasks/extension-spaces-cron.md`](tasks/extension-spaces-cron.md).

## extension-crawl-status
- status: backlog
- parent: extension-redesign
- goal: Show concurrent crawl jobs / which spaces are crawling.
- done_when: Multiple active jobs visible; per-job cancel if API allows.
- notes: Detail in [`tasks/extension-crawl-status.md`](tasks/extension-crawl-status.md).

## extension-catalog-ui
- status: backlog
- parent: extension-redesign
- goal: Optional in-extension search / get page / page list (or cancel if deferred).
- done_when: Catalog flows shipped or task cancelled after mockups.
- notes: Detail in [`tasks/extension-catalog-ui.md`](tasks/extension-catalog-ui.md).
