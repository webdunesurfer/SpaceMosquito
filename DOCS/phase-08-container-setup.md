# Phase 8: Container Setup

> **Historical.** This phase document describes work from the Docker/Postgres era.
> Docker mode has been removed; SpaceMosquito is SQLite-only.
> See [`DOCS/task-remove-docker-mode.md`](./task-remove-docker-mode.md).

## Objective
Create Docker images and docker-compose configuration for running the full SpaceMosquito stack in a container.

## Deliverables
- `Dockerfile` — Lean: Go binary + Chromium (chromedp, no Xvfb, no Node.js)
- `docker-compose.yml` — Multi-service configuration
- Backend API on port 8080, MCP on port 8081

## Tasks

### 8.1 — Dockerfile
- `Dockerfile`:
  ```dockerfile
  # syntax=docker/dockerfile:1
  FROM golang:1.25-bookworm AS builder

  WORKDIR /build
  COPY space-mosquito/go.mod space-mosquito/go.sum ./
  RUN go mod download
  COPY space-mosquito/ ./
  RUN go build -o /server ./cmd/server && \
      go build -o /cli ./cmd/cli

  # Runtime stage
  FROM debian:bookworm-slim

  # Install Chromium + CDP deps + curl
  RUN apt-get update && apt-get install -y --no-install-recommends \
      chromium \
      libatk-bridge2.0-0 \
      libatk1.0-0 \
      libcups2 \
      libdrm2 \
      libgbm1 \
      libnspr4 \
      libnss3 \
      libpango-1.0-0 \
      libxcomposite1 \
      libxdamage1 \
      libxrandr2 \
      libxshmfence1 \
      libxss1 \
      libxtst6 \
      libglib2.0-0 \
      libgtk-3-0 \
      curl \
      ca-certificates \
      fonts-noto-cjk \
      fonts-dejavu-core \
      && rm -rf /var/lib/apt/lists/*

  WORKDIR /app

  # Copy binaries and assets from builder
  COPY --from=builder /server /app/server
  COPY --from=builder /cli /app/cli
  COPY --from=builder /build/migrations /app/migrations
  COPY --from=builder /build/config.yaml.example /app/config.yaml.example
  COPY app-start.sh /app/start.sh
  RUN chmod +x /app/start.sh

  RUN useradd -m -s /bin/bash appuser && \
      touch /app/session.enc && chmod 0600 /app/session.enc && \
      mkdir -p /app/saved && chmod 0755 /app/saved && \
      chown -R appuser:appuser /app
  USER appuser

  EXPOSE 8080 8081

  ENV CONFIG_PATH=/app/config.yaml

  HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

  ENTRYPOINT ["/app/start.sh"]
  ```

### 8.2 — app-start.sh (no Xvfb)
- `app-start.sh`:
  ```bash
  #!/bin/sh
  # chromedp runs Chromium headless natively — no Xvfb needed
  exec /app/server
  ```

### 8.3 — Docker Compose
- `docker-compose.yml`:
  ```yaml
  services:
    db:
      image: pgvector/pgvector:pg17
      environment:
        POSTGRES_USER: spacemosquito
        POSTGRES_PASSWORD: spacemosquito
        POSTGRES_DB: spacemosquito
      ports:
        - "5432:5432"
      volumes:
        - pgdata:/var/lib/postgresql/data
      healthcheck:
        test: ["CMD-SHELL", "pg_isready -U spacemosquito"]
        interval: 5s
        timeout: 3s
        retries: 10
        start_period: 10s

    app:
      build:
        context: .
        dockerfile: Dockerfile
      environment:
        CONFIG_PATH: /app/config.yaml
        DATABASE_URL: postgres://spacemosquito:spacemosquito@db:5432/spacemosquito?sslmode=disable
      ports:
        - "8080:8080"
        - "8081:8081"
      depends_on:
        db:
          condition: service_healthy
      volumes:
        - ./config.yaml:/app/config.yaml:ro
        - saved-data:/app/saved
        - ./session.enc:/app/session.enc:rw

  volumes:
    pgdata:
    saved-data:
  ```

### 8.4 — Config File for Container
- `config.yaml` (container):
  ```yaml
  database:
    host: db
    port: 5432
    user: spacemosquito
    password: spacemosquito
    dbname: spacemosquito
    sslmode: disable

  storage:
    base_path: /app/saved

  session:
    encryption_key: ${SESSION_ENCRYPTION_KEY}
    file_path: /app/session.enc

  mcp:
    port: 8081
    host: "0.0.0.0"
  ```

## Key Differences from Playwright Approach
- **No Node.js**: chromedp is pure Go, no npm/driver needed
- **No Xvfb/DISPLAY**: Chromium headless runs natively without a display server
- **Smaller image**: ~200MB total vs ~800MB+ with Firefox + Node.js + Xvfb
- **Faster startup**: no Xvfb init, no driver download, no version matching
- **Simpler Dockerfile**: single base image (debian-slim), no multi-stage npm installs

## Acceptance Criteria
- `docker compose up` starts the full stack
- PostgreSQL is accessible and migrations run
- Go backend is accessible on port 8080
- MCP server is accessible on port 8081
- `spacemosquito crawl` command works headlessly in the container
- Saved data persists across container restarts
