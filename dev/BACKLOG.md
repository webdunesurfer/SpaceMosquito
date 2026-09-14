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

## extension-toolbar-detect-icon
- status: done
- blocked_by: extension-themes
- goal: Red toolbar icon on Confluence tabs, gray otherwise (no popup open needed).
- done_when: Per-tab setIcon on activate/navigate; PNG red+gray; FF+Chrome; CHANGELOG.
- notes: Done — active `#9b4d4d` / inactive `#8a93a0`; [`tasks/extension-toolbar-detect-icon.md`](tasks/extension-toolbar-detect-icon.md).

## extension-themes
- status: done
- goal: Light/dark popup themes auto-following browser scheme; single muted-red mosquito icon; keep red crawl progress.
- done_when: Two themes + prefers-color-scheme (fallback light); no manual switch; one theme-agnostic icon; both browsers; CHANGELOG.
- notes: Done — CSS tokens light/dark; icon `#9b4d4d`; progress `--progress-fill`. [`tasks/extension-themes.md`](tasks/extension-themes.md).

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
- status: done
- goal: Epic — redesign Firefox/Chrome extensions (session, backend gate, spaces/cron, crawl status, optional catalog).
- done_when: Mockups locked; children shipped (catalog may defer); gating + multi-wiki aware; both browsers; CHANGELOG.
- notes: Done — children + smoke polish (homepageId, pages_total on cancel, session hint/Refresh, revalidate flicker, cron checkbox/icon). Task docs kept until cleanup/release. [`tasks/extension-redesign.md`](tasks/extension-redesign.md).

## extension-redesign-mockups
- status: done
- parent: extension-redesign
- goal: Lock IA and key popup screens before UI implementation.
- done_when: Accepted wireframes/mockups; epic open decisions updated; children unblocked on layout.
- notes: Done (rev 3) — [`tasks/extension-redesign-mockups/WIREFRAMES.md`](tasks/extension-redesign-mockups/WIREFRAMES.md). IA: Page/Spaces/Settings; Session disc only; backend banner + 2s popup-only poll; catalog omitted.

## extension-session-ux
- status: done
- parent: extension-redesign
- goal: One-button capture+validate; session info/delete; validity gating; optional auto-renew.
- done_when: Combined capture+validate; multi-site session UI; gray-out when invalid; both browsers.
- notes: Done — GWT S1–S7; compare+refresh APIs; CHANGELOG [Unreleased]. [`tasks/extension-session-ux.md`](tasks/extension-session-ux.md).

## extension-backend-status
- status: done
- parent: extension-redesign
- goal: Live trustworthy backend available/unavailable; disable API-dependent UI when down.
- done_when: Status matches serve up/down; gating works; flicker fixed.
- notes: Done — GWT B1–B5; CHANGELOG [Unreleased]. Detail in [`tasks/extension-backend-status.md`](tasks/extension-backend-status.md).

## extension-spaces-cron
- status: done
- parent: extension-redesign
- goal: Spaces list with cron on/off and clearer actions.
- done_when: Cron enablement visible per space; actions + gating; both browsers.
- notes: Done — GWT Z1–Z6; pages_total; clock on/off colors; CHANGELOG [Unreleased]. [`tasks/extension-spaces-cron.md`](tasks/extension-spaces-cron.md).

## extension-crawl-status
- status: done
- parent: extension-redesign
- goal: Show concurrent crawl jobs / which spaces are crawling.
- done_when: Multiple active jobs visible; per-job cancel if API allows.
- notes: Done — GWT C1–C6 inline on Spaces; CHANGELOG [Unreleased]. [`tasks/extension-crawl-status.md`](tasks/extension-crawl-status.md).

## extension-catalog-ui
- status: cancelled
- parent: extension-redesign
- goal: Optional in-extension search / get page / page list (or cancel if deferred).
- done_when: Catalog flows shipped or task cancelled after mockups.
- notes: Cancelled for v1 (mockups omit catalog). Detail in [`tasks/extension-catalog-ui.md`](tasks/extension-catalog-ui.md).
