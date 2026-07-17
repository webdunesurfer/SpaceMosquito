.PHONY: build run test migrate-up migrate-down lint dev-extension build-extension clean

BUILD_DIR=build

build:
	mkdir -p $(BUILD_DIR)
	cd space-mosquito && go build -o ../$(BUILD_DIR)/spacemosquito ./cmd/spacemosquito

run: build
	$(BUILD_DIR)/spacemosquito serve

test:
	cd space-mosquito && go test -race ./...

migrate-up:
	cd space-mosquito && go run ./cmd/spacemosquito init

migrate-down:
	cd space-mosquito && go run ./cmd/spacemosquito migrate-down

lint:
	cd space-mosquito && go vet ./...
	cd firefox-extension && npx tsc --noEmit

dev-extension:
	cd firefox-extension && npx web-ext run --source-dir ./dist --target firefox

build-extension:
	cd firefox-extension && npx webpack --mode production

clean:
	rm -rf build/
	cd space-mosquito && go clean
