# Remove legacy single-session `session.enc` compatibility

- **Task ID:** `session-enc-legacy-cleanup`
- **Status:** backlog
- **Parent:** `multi-wiki-sessions` (shipped in 0.3.4; see ADR-002)
- **Blocked by:** — (schedule after users have had at least one release that writes the v2 multi-session blob)

## Problem

[`multi-wiki-sessions`](../CHANGELOG.md) (0.3.4) kept compatibility shims so existing
single-session `session.enc` files still load:

- Decrypt → unmarshal as bare `Session` → wrap into hostname map
- `Store.Save` alias → `Upsert`
- `Store.Load` “exactly one session” helper for callers without a URL
- API status/validate falling back to `Load` when no URL is provided
- Tests that still exercise the legacy on-disk shape

After a few releases, most installs will only have the v2 blob. The shims become
dead weight and hide “must pass a host/URL” mistakes.

## Goal

Drop legacy single-session format support and thin aliases once migration window
is over; store/API speak **only** multi-session blob + `GetForURL` / `Upsert`.

## Decisions

| # | Topic | Status | Decision |
|---|--------|--------|----------|
| 1 | When | open | After N releases / changelog note that old single-session files need one capture under a multi-session build first (document in release notes). |
| 2 | Behavior on old file | open | Fail with a clear “re-capture session” error vs one-shot migrate-and-rewrite on read (prefer fail-closed once compatibility ends). |

## Non-goals

- Changing encryption or filename (`session.enc` path can stay)
- Redesigning extension session UI (see [`extension-redesign`](extension-redesign.md))

## Implementation

Likely removals / tightenings (re-audit at start):

1. `loadBlob` legacy `Session` unmarshal path in `internal/session/store.go`
2. `Save` / `Load` if unused — force `Upsert` / `GetForURL` / `GetForHostname`
3. API `SessionStatus` / `ValidateSession` paths that call `Load` without URL
4. Legacy migration unit tests; replace with “reject unknown blob” test
5. ADR-002 / README note that pre-multi-wiki files are unsupported

## Done when

- [ ] No code path unmarshals a bare single `Session` as the on-disk file
- [ ] Call sites use hostname/URL selection; no “sole session” Load fallback in production API
- [ ] Tests updated; CHANGELOG notes breaking compatibility if applicable
- [ ] ADR-002 consequences updated

## Notes

Do **not** rush this into the same release as multi-wiki-sessions. Prefer after
operators have run a version that upserts the v2 format at least once.
