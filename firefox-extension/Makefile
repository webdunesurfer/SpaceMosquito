.PHONY: build dev dev:watch dev:firefox lint clean

build:
	npx webpack --mode production

dev:
	npx webpack --mode development

dev:watch:
	npx webpack --mode development --watch

dev:firefox:
	npx web-ext run --source-dir ./dist --target firefox

lint:
	npx tsc --noEmit

clean:
	rm -rf dist
