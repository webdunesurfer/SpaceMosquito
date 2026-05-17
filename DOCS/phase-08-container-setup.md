# Phase 8: Container Setup

## Objective
Create Docker images and docker-compose configuration for running the full SpaceMosquito stack in a container, including Firefox with noVNC for interactive authentication.

## Deliverables
- `Dockerfile` — Full stack: Go binary + Firefox + Xvfb + noVNC + PostgreSQL
- `Dockerfile.backend` — Lean: Go binary + PostgreSQL only (no browser)
- `docker-compose.yml` — Multi-service configuration
- VNC access on port 6080 for interactive browser use
- Backend API on port 8080, MCP on port 8081

## Tasks

### 8.1 — Full Dockerfile
- `docker/Dockerfile`:
  ```dockerfile
  FROM golang:1.22-alpine AS builder
  WORKDIR /app
  COPY space-mosquito/ .
  RUN go mod download && go build -o /space-mosquito ./cmd/server

  FROM mozilla/firefox:latest
  RUN apk add --no-cache \
      xvfb \
      nmap-ncat \
      curl \
      tini

  # Install Playwright Firefox dependencies
  RUN npx playwright install-deps firefox 2>/dev/null || true
  # Download nomic-embed-text model
  RUN mkdir -p /app/models && \
      curl -L https://huggingface.co/nomic-ai/nomic-embed-text-v1/resolve/main/onnx/model.onnx \
        -o /app/models/nomic-embed-text.onnx

  COPY --from=builder /space-mosquito /usr/local/bin/space-mosquito
  COPY docker/entrypoint.sh /entrypoint.sh
  RUN chmod +x /entrypoint.sh

  EXPOSE 8080 8081 6080

  ENTRYPOINT ["/usr/bin/tini", "--", "/entrypoint.sh"]
  ```

### 8.2 — Lean Backend Dockerfile
- `docker/Dockerfile.backend`:
  ```dockerfile
  FROM golang:1.22-alpine AS builder
  WORKDIR /app
  COPY space-mosquito/ .
  RUN go mod download && go build -o /space-mosquito ./cmd/server

  FROM alpine:3.19
  RUN apk add --no-cache tini curl
  COPY --from=builder /space-mosquito /usr/local/bin/space-mosquito
  EXPOSE 8080 8081
  ENTRYPOINT ["/usr/bin/tini", "--", "space-mosquito", "serve"]
  ```

### 8.3 — noVNC Setup
- `docker/entrypoint.sh`:
  ```bash
  #!/bin/sh
  # Start Xvfb virtual display
  Xvfb :99 -screen 0 1280x720x24 &

  # Start noVNC
  # (use docker-vnc-image or novnc/novnc for web-based VNC)
  # For simplicity, use a pre-built noVNC container

  # Start Go backend
  exec space-mosquito serve
  ```

### 8.4 — Docker Compose
- `docker-compose.yml`:
  ```yaml
  services:
    app:
      build:
        context: .
        dockerfile: docker/Dockerfile
      environment:
        DATABASE_HOST: db
        DATABASE_PORT: 5432
        DATABASE_USER: spacemosquito
        DATABASE_PASSWORD: spacemosquito
        DATABASE_NAME: spacemosquito
        SESSION_ENCRYPTION_KEY: ${SESSION_KEY:-changeme}
        CONFIG_PATH: /app/config.yaml
      ports:
        - "8080:8080"
        - "8081:8081"
      volumes:
        - ./config.yaml:/app/config.yaml:ro
        - saved-data:/app/saved
        - session-data:/app/session
      depends_on:
        - db

    db:
      image: pgvector/pgvector:pg16
      environment:
        POSTGRES_USER: spacemosquito
        POSTGRES_PASSWORD: spacemosquito
        POSTGRES_DB: spacemosquito
      volumes:
        - pgdata:/var/lib/postgresql/data

    vnc:
      image: consol/ubuntu-xfce-vnc:latest
      ports:
        - "6080:6080"
      environment:
        VNC_PASSWORD: vnc123
      depends_on:
        - app
      # Firefox and extension pre-installed in this image
      # Extension loaded via --profile flag

  volumes:
    pgdata:
    saved-data:
    session-data:
  ```

### 8.5 — noVNC Firefox Configuration
- Pre-configure Firefox in noVNC container:
  - Load SpaceMosquito extension as temporary (dev) or XPI (production)
  - Set default home page to Confluence URL (configurable via env var)
  - Configure auto-connect to backend API (`http://app:8080`)

### 8.6 — Config File for Container
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
    file_path: /app/session/session.enc

  mcp:
    port: 8081
    host: "0.0.0.0"
  ```

## Acceptance Criteria
- `docker compose up` starts the full stack
- PostgreSQL is accessible and migrations run
- Go backend is accessible on port 8080
- MCP server is accessible on port 8081
- noVNC is accessible on port 6080 with Firefox pre-loaded
- Extension loads in noVNC Firefox
- Saved data persists across container restarts
