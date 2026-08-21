# Status

Last updated: 2026-08-21

## Current focus

Landed [`task-firefox-persistent-install`](tasks/task-firefox-persistent-install.md) (AMO-signed Firefox XPI in release CI). Ensure repo secrets `AMO_JWT_ISSUER` / `AMO_JWT_SECRET` are set before the next `v*` tag.

## Active task

_None._

## Blockers

_None._ (Release will fail without AMO secrets.)

## Next action

1. Confirm GitHub repository secrets for AMO JWT are configured.
2. Pick up next backlog item (e.g. [`task-page-dir-collision`](tasks/task-page-dir-collision.md) or [`task-cme-markdown-gap-analysis`](tasks/task-cme-markdown-gap-analysis.md)).
