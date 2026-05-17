# Phase 1: Go Backend Core

## Objective
Scaffold the Go backend with configuration, database setup, file storage layer, and a working CLI for saving pages locally.

## Deliverables
- Go module initialized with all required dependencies
- YAML config loader
- PostgreSQL connection with golang-migrate integration
- Initial database schema (spaces, pages, embeddings)
- File storage layer for saving HTML + metadata
- CLI entrypoint: `space-mosquito save <url>` saves a single page
- Docker Compose for local PostgreSQL

## Tasks

### 1.1 — Go Module Scaffolding
- Initialize Go module: `go mod init github.com/vkh/spacemosquito`
- Create directory structure:
  ```
  space-mosquito/
  ├── cmd/server/main.go
  ├── cmd/cli/main.go
  ├── internal/config/config.go
  ├── internal/db/postgres.go
  ├── internal/db/models.go
  ├── internal/storage/writer.go
  ├── internal/storage/asset.go
  ├── migrations/
  ├── go.mod
  └── go.sum
  ```
- Add dependencies:
  - `github.com/golang-migrate/migrate/v4`
  - `github.com/lib/pq`
  - `gopkg.in/yaml.v3`
  - `github.com/google/uuid`
  - `go.uber.org/zap`
  - `github.com/andybalholm/cascadia`

### 1.2 — YAML Configuration
- Define config struct in `internal/config/config.go`:
  ```yaml
  database:
    host: localhost
    port: 5432
    user: spacemosquito
    password: ""
    dbname: spacemosquito
    sslmode: disable

  storage:
    base_path: "./saved"

  session:
    encryption_key: ""
    file_path: "~/.config/spacemosquito/session.enc"

  embedder:
    model: nomic-embed-text
    # openai:
    #   api_key: ""
    #   model: text-embedding-3-small
  ```
- Config loading from `~/.config/spacemosquito/config.yaml`
- Environment variable override for sensitive values

### 1.3 — Database Schema & Migrations
- Create `migrations/001_initial.up.sql`:
  ```sql
  CREATE EXTENSION IF NOT EXISTS vector;

  CREATE TABLE spaces (
      id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      key VARCHAR(10) NOT NULL UNIQUE,
      name TEXT NOT NULL,
      url TEXT NOT NULL,
      last_crawled TIMESTAMP WITH TIME ZONE,
      created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
  );

  CREATE TABLE pages (
      id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      space_id UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
      confluence_id INTEGER NOT NULL,
      title TEXT NOT NULL,
      parent_confluence_id INTEGER,
      content TEXT,
      html_path TEXT,
      raw_html_path TEXT,
      metadata_path TEXT,
      file_dir TEXT,
      created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
      updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
      UNIQUE(space_id, confluence_id)
  );

  CREATE TABLE page_embeddings (
      id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      page_id UUID NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
      embedding vector(768) NOT NULL,
      created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
  );

  CREATE INDEX idx_pages_space ON pages(space_id);
  CREATE INDEX idx_pages_parent ON pages(parent_confluence_id);
  CREATE INDEX idx_embeddings_vector ON page_embeddings USING ivfflat (embedding vector_cosine_ops);
  ```
- Create `migrations/001_initial.down.sql` (reverse)

### 1.4 — PostgreSQL Connection
- `internal/db/postgres.go`:
  - `NewDB(config)` — connection pool setup with pgx
  - `MigrateUp()` — run golang-migrate
  - `MigrateDown()` — rollback migrations
- Auto-migrate on server startup
- Connection pool settings (max open, max idle, timeout)

### 1.5 — Data Models
- `internal/db/models.go`:
  - `Space` struct with fields
  - `Page` struct with fields
  - `PageEmbedding` struct with vector field
  - Query functions: `CreateSpace`, `GetSpace`, `UpsertPage`, `GetPage`, `ListPages`, `CreateEmbedding`, `SearchEmbeddings`

### 1.6 — File Storage Layer
- `internal/storage/writer.go`:
  - `SavePage(spaceKey, pageTitle, html, rawHTML, metadata)` — creates directory structure
  - Directory layout: `saved/<space-key>/<sanitized-path>/`
  - File naming: `index.html`, `raw.html`, `metadata.json`
- `internal/storage/asset.go`:
  - `DownloadAsset(url, destDir)` — HTTP download with retry
  - URL rewriting for saved HTML (local asset paths)

### 1.7 — CLI Entrypoint
- `cmd/cli/main.go`:
  - Command: `space-mosquito save <url>`
  - Loads config, connects to DB
  - For now: fetch page HTML (placeholder for scraper), save to disk with metadata
  - Command: `space-mosquito init` — run migrations

### 1.8 — Docker Compose for PostgreSQL
- `docker-compose.yml` (initial):
  ```yaml
  services:
    db:
      image: pgvector/pgvector:pg16
      environment:
        POSTGRES_USER: spacemosquito
        POSTGRES_PASSWORD: spacemosquito
        POSTGRES_DB: spacemosquito
      ports:
        - "5432:5432"
      volumes:
        - pgdata:/var/lib/postgresql/data

  volumes:
    pgdata:
  ```

## Acceptance Criteria
- `go run ./cmd/cli init` creates the database schema
- `go run ./cmd/cli save <url>` saves a page to disk with metadata.json
- PostgreSQL runs in Docker with pgvector extension
- Config loaded from YAML with env var overrides
