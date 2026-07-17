.PHONY: build run test migrate-up migrate-down lint dev-extension build-extensions clean

build:
	cd spacemosquito && go build -o spacemosquito ./cmd/spacemosquito

run: build
	cd spacemosquito && ./spacemosquito serve

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

build-extensions:
	cd firefox-extension && npm install && npx webpack --mode production
	cd chrome-extension && npm install && npx webpack --mode production

clean:
	rm -f spacemosquito/spacemosquito
	cd spacemosquito && go clean
