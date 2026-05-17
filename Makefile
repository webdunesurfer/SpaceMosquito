.PHONY: build run test dev-extension build-extension docker-up docker-down migrate-up migrate-down lint

BUILD_DIR=build

build:
	cd space-mosquito && go build -o $(BUILD_DIR)/space-mosquito ./cmd/server
	cd space-mosquito && go build -o $(BUILD_DIR)/spacemosquito-cli ./cmd/cli

run: build
	$(BUILD_DIR)/space-mosquito serve

test:
	cd space-mosquito && go test ./... -v

migrate-up:
	cd space-mosquito && go run ./cmd/cli init

migrate-down:
	cd space-mosquito && go build -o $(BUILD_DIR)/cli ./cmd/cli

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

lint:
	cd space-mosquito && go vet ./...
	cd firefox-extension && npx tsc --noEmit

dev-extension:
	cd firefox-extension && npx web-ext run --source-dir ./dist --target firefox

build-extension:
	cd firefox-extension && npx webpack --mode production

config-example:
	@cat > config.yaml.example << 'EOF'
database:
  host: localhost
  port: 5432
  user: spacemosquito
  password: spacemosquito
  dbname: spacemosquito
  sslmode: disable

storage:
  base_path: ./saved

session:
  encryption_key: ""
  file_path: ~/.config/spacemosquito/session.enc

embedder:
  model: nomic-embed-text
  # openai:
  #   api_key: ""
  #   model: text-embedding-3-small

mcp:
  port: 8081
  host: "0.0.0.0"
  session_timeout: 3600

cron:
  full_crawl:
    enabled: false
    interval: "24h"
    spaces: []
  incremental:
    enabled: false
    interval: "2h"
    spaces: []
EOF
