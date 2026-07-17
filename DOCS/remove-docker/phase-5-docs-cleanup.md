# Phase 5 — Documentation Cleanup

**Parent:** [`DOCS/task-remove-docker-mode.md`](../task-remove-docker-mode.md)  
**Priority:** P0  
**Size:** M  
**Status:** Done  
**Depends on:** Phases 1–4 ideally done so docs match reality.  
**Decision:** Merge into **or** fully replace `README.md` from `README-dockerless.md`, then **delete** `README-dockerless.md`. Release cutover: ignore.

## Objective

One install story: **SQLite / local binary only**. No Colima/Compose primary path. Scrub dual-mode wording from active docs.

## Work items

| Action | Doc |
|--------|-----|
| **Rewrite / replace** | `README.md` — dockerless-only quick start |
| **Delete** | `README-dockerless.md` after content is in `README.md` |
| **Update** | `ARCHITECTURE.md`, `DEVELOPMENT.md`, `space-mosquito/config.yaml.example` |
| **Update / archive** | `DOCS/epic-dockerless-mode.md` — dual-mode epic completed; point to this removal overview |
| **Mark historical** | `phase-01`, `phase-08`, `phase-10` — banner: "historical; Docker removed" |
| **Scrub** | Key taskdocs with "both Postgres and SQLite" / "Docker remains" (`task-improve-search`, `task-get-page-by-confluence-id`, `task-import-saved`, `task-mcp-search-excerpts`, …) |
| **Migration note** | Short "Coming from Docker?" — link Phase 1 cleanup script + `import_saved` (see overview migration path) |
| **Link** | Phase 1 cleanup script from README |

## Acceptance criteria

- [x] `README.md` is dockerless-only; no Compose quick start
- [x] `README-dockerless.md` deleted
- [x] Architecture / development docs match SQLite-only
- [x] Historical phase docs bannered (not mass-deleted)
- [x] Overview + cleanup script discoverable from README

## Out of scope

- ADR create/delete list (Phase 6)
- Mass-deleting all `DOCS/phase-*` files
