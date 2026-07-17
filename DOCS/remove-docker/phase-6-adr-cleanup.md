# Phase 6 — ADR Cleanup

**Parent:** [`DOCS/task-remove-docker-mode.md`](../task-remove-docker-mode.md)  
**Priority:** P0  
**Size:** S  
**Depends on:** Can ship with Phase 5; decision: hard-delete obsolete ADRs (no long tombstones required).

## Objective

Record **SQLite-only** distribution as the accepted architecture. Remove ADRs that locked in "keep Docker" or obsolete Docker-primary decisions. Lightly amend keepers that still mention containers.

## New ADR

**Add** `ADR/016-sqlite-only-distribution.md`:

- SQLite + FTS5 only; Docker / Postgres removed from the product
- Supersedes ADR-014
- Points to `DOCS/task-remove-docker-mode.md` and this phase folder

## Delete

| ADR | Why |
|-----|-----|
| `ADR/014-dockerless-sqlite-distribution.md` | "Keep Docker" reversed; replaced by ADR-016 |
| `ADR/012-chromedp-over-playwright.md` | Already superseded by ADR-013; Docker sandbox narrative only |
| `ADR/003-embedding-model-selection.md` | Embeddings / ONNX not shipped; Docker sizing obsolete |
| `ADR/006-saved-page-format.md` | Optional: already superseded by ADR-015 — delete if still present as tombstone-only |

## Keep but amend (one-line historical Docker note where needed)

| ADR | Note |
|-----|------|
| `ADR/001` extension | Current |
| `ADR/002` hybrid auth | Drop "in a Docker container" |
| `ADR/004` headless browser | Chromium on host / rod download, not image |
| `ADR/005` session file | Data-dir path, not volume |
| `ADR/007` MCP SSE | "Remote Docker" → "remote host" |
| `ADR/008` YAML config | Current |
| `ADR/009` golang-migrate | Entrypoint → `spacemosquito init` / embed |
| `ADR/010` HTML extraction | Current |
| `ADR/011` Go language | "Ideal for Docker" → "single binary" |
| `ADR/013` go-rod | Docker EPERM → historical note |
| `ADR/015` saved format + markdown | Current |

Amend in the same PR as ADR-016 or a small docs follow-up.

## Acceptance criteria

- [ ] ADR-016 accepted and linked from overview
- [ ] ADR-014, 012, 003 deleted (006 if applicable)
- [ ] Keepers amended or explicitly deferred to a follow-up with a tracked list

## Out of scope

- Rewriting README (Phase 5)
- Code changes (Phase 3)
