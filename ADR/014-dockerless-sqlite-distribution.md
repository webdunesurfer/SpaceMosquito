# ADR-014: Dockerless Local Distribution

- **Status**: Accepted
- **Date**: 2026-07-09
- **Context**: The current Docker + PostgreSQL setup is appropriate for development but too heavy for end users who want a local install with crawl, search, MCP, and the existing browser extension. We need a simpler default install without giving up the current Docker-based workflow.

- **Decision**:
  - Add a **dockerless mode** as the default for new local installs: one application, one data location, embedded database, no separate database service.
  - Keep **Docker + PostgreSQL** as a supported option for developers and existing deployments.
  - Use **API-first scraping**; headless browser remains a **fallback**, loaded only when needed.
  - Support **portable installs** (all user data in one relocatable directory).
  - Provide **migration paths** for existing users (re-crawl, import from saved files, export/import tool) — details in task docs, not this ADR.

- **Rationale**:
  - Lowers setup friction for the primary use case (single user, local machine).
  - Preserves current behavior for teams already on Docker.
  - Avoids requiring a browser engine when the Confluence API is enough.
  - Structured storage and search still need a database layer; files alone are insufficient at scale.

- **Alternatives considered**:
  - **Postgres-only** — rejected; too much ops overhead for end users.
  - **Files-only (no database)** — rejected; poor fit for search, listing, and incremental crawl at scale.
  - **Replace Docker entirely** — rejected; dockerless is additive.

- **Consequences**:
  - Implementation details (drivers, packages, paths, migration layout, CI, platform matrix) live in **`DOCS/epic-dockerless-mode.md`** and linked task docs; they may change without amending this ADR.
  - Search ranking may differ slightly between database backends; acceptable if results remain useful.
  - Browser extensions stay sideload-only; no store publishing in this effort.
  - SSO/session validation must be reliable before dockerless is considered production-ready.

- **Related**: `DOCS/epic-dockerless-mode.md`, ADR-004, ADR-009
