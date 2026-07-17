.PHONY: build run test migrate-up migrate-down lint dev-extension build-extension clean

BUILD_DIR=build

build:
	mkdir -p $(BUILD_DIR)
	cd spacemosquito && go build -o ../$(BUILD_DIR)/spacemosquito ./cmd/spacemosquito

run: build
	$(BUILD_DIR)/spacemosquito serve

test:
	cd spacemosquito && go test -race ./...

migrate-up:
	cd spacemosquito && go run ./cmd/spacemosquito init

migrate-down:
	cd spacemosquito && go run ./cmd/spacemosquito migrate-down

lint:
	cd spacemosquito && go vet ./...
	cd firefox-extension && npx tsc --noEmit

dev-extension:
	cd firefox-extension && npx web-ext run --source-dir ./dist --target firefox

build-extension:
	cd firefox-extension && npx webpack --mode production

clean:
	rm -rf build/
	cd spacemosquito && go clean
