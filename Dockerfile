# syntax=docker/dockerfile:1
FROM golang:1.25-bookworm AS builder

WORKDIR /build

# Copy go mod files first for layer caching
COPY space-mosquito/go.mod space-mosquito/go.sum ./
RUN go mod download

# Copy source and build
COPY space-mosquito/ ./
RUN go build -o /server ./cmd/server && \
    go build -o /cli ./cmd/cli

# Runtime stage
FROM debian:bookworm-slim

# Install Chromium + required deps
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
    libpangocairo-1.0-0 \
    libgtk-3-0 \
    libglib2.0-0 \
    libx11-6 \
    libxcomposite1 \
    libxdamage1 \
    libxrandr2 \
    libxshmfence1 \
    libxss1 \
    libxtst6 \
    libxcb1 \
    libegl1 \
    libpci3 \
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

RUN ln -sf /usr/bin/chromium /usr/bin/chrome && \
    useradd -m -s /bin/bash appuser && \
    touch /app/session.enc && chmod 0600 /app/session.enc && \
    mkdir -p /app/saved && chmod 0755 /app/saved && \
    chown -R appuser:appuser /app
USER appuser

EXPOSE 8080 8081

ENV CONFIG_PATH=/app/config.yaml

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/start.sh"]
