# Overview: Remove Docker Mode Completely (SQLite-Only)

## Objective

Make **SQLite / dockerless the only supported runtime**. Delete Docker Compose, the PostgreSQL driver path, container scripts, and dual-mode complexity so the project has one database, one install story, and one test surface.

**Today:** ADR-014 and `DOCS/epic-dockerless-mode.md` treat Docker + Postgres as a **kept** option. This work **reverses** that.

**Out of scope:** semantic/vector search, scrape/API/MCP behavior beyond dropping Postgres, extension store publishing, binary auto-update, release-process cutover (none in place).

**Related:**

- `DOCS/epic-dockerless-mode.md` — historical dual-mode epic
- `DOCS/task-import-saved.md` — migrate catalogs from old Docker `saved/` trees
- Phase docs under [`DOCS/remove-docker/`](./remove-docker/)

---

## Phases

| Phase | Doc | Size | Notes |
|-------|-----|------|-------|
| 1 | [`phase-1-cleanup-script.md`](./remove-docker/phase-1-cleanup-script.md) | S | **Done** — `scripts/cleanup-docker-legacy.sh`, [`DOCS/guides/cleanup-docker-legacy.md`](./guides/cleanup-docker-legacy.md) |
| 2 | [`phase-2-delete-packaging.md`](./remove-docker/phase-2-delete-packaging.md) | S | **Done** — Dockerfile/Compose/scripts deleted; Makefile cleaned |
| 3 | [`phase-3-code-cleanup.md`](./remove-docker/phase-3-code-cleanup.md) | M | **Done** — SQLite-only store/config; `pgx` removed |
| 4 | [`phase-4-test-cleanup.md`](./remove-docker/phase-4-test-cleanup.md) | S–M | **Done** — Docker smoke scripts removed; SQLite tests only |
| 5 | [`phase-5-docs-cleanup.md`](./remove-docker/phase-5-docs-cleanup.md) | M | README merge/replace; scrub dual-mode docs |
| 6 | [`phase-6-adr-cleanup.md`](./remove-docker/phase-6-adr-cleanup.md) | S | ADR-016; delete 014/012/003 |

**Total:** ~1–2 days. Implementation detail lives **only** in the phase docs — do not duplicate here.

---

## Target Architecture

```
User machine
  spacemosquito (binary)
    ~/.spacemosquito/   (or --data-dir)
      config.yaml         # sqlite only
      spacemosquito.db
      session.enc
      saved/
      browser/            # optional rod Chromium
  Extension → localhost:8081
  MCP → localhost:8081/mcp
```

| Area | After removal |
|------|---------------|
| Database | SQLite + FTS5 only |
| Install | Source build or release binary |
| Config | Default driver `sqlite`; never postgres |
| Migrations | `migrations/sqlite/` only |
| Docker | Gone from repo |
| Old Docker users | `saved/` + `import_saved` + `reindex --content` (no Postgres dump) |

High-level dual-mode inventory (packaging, code, docs, tests) is listed per phase — start from the phase tables.

---

## Migration Path for Existing Docker Users

1. `./scripts/cleanup-docker-legacy.sh` (Phase 1) — stop containers, remove `pgdata`.
2. `spacemosquito init`.
3. Copy Compose bind-mount `saved-data/` (or `./saved`) → `~/.spacemosquito/saved`.
4. `spacemosquito bootstrap import-saved`.
5. `spacemosquito reindex --content`.
6. Extension → `http://localhost:8081`; `spacemosquito serve`.

---

## Design Decisions

| Question | Decision |
|----------|----------|
| Keep Docker optional? | **No** |
| Migrate Postgres data? | **No** — `saved/` + `import_saved` |
| Default DB driver | **sqlite** |
| Cleanup script | **Yes** — dry-run; volumes yes; bind mounts opt-in |
| Packaging removal | **Hard delete** (no `archive/docker/`) |
| `README-dockerless.md` | **Merge into or replace `README.md`**, then delete |
| ADR-014 | **Delete**; replace with ADR-016 |
| Historical `DOCS/phase-*` | **Banner**, do not mass-delete |
| `cmd/server` + `cmd/cli` | **Keep** as aliases |
| Release / CI cutover | **Ignore** |

---

## Overall Acceptance Criteria

- [ ] Phase 1–6 acceptance criteria met (see each phase doc)
- [ ] No Docker packaging; no Postgres store/migrations; SQLite default
- [ ] Tests green without Docker/Postgres
- [ ] Single README install story; ADR-016 in place
