# Install from source

Build the `spacemosquito` binary and browser extensions from a git clone.

For pre-built binaries and extension zips, see [Install](INSTALL.md).

## Requirements

- Go 1.25+
- npm (and npx) for building extensions
- Firefox or Chrome (for the Pirate Mosquito extension)

## Binary

```sh
git clone https://github.com/webdunesurfer/SpaceMosquito.git
cd SpaceMosquito
make build
# binary: spacemosquito/spacemosquito
```

Cross-platform release artifacts (binaries + extension zips, same layout as CI):

```sh
cd spacemosquito
./scripts/build-release.sh v0.2.0
ls dist/
```

Extension zips only:

```sh
./scripts/build-extension-zips.sh v0.2.0
```

## Browser extensions

```sh
make build-extensions
# or:
#   cd firefox-extension && npm ci && npm run build
#   cd chrome-extension && npm ci && npm run build
```

### Firefox

Open `about:debugging` → **This Firefox** → **Load Temporary Add-on…** → select
`firefox-extension/dist/manifest.json`.

### Chrome

Open `chrome://extensions` → **Developer mode** → **Load unpacked** → select
`chrome-extension/dist/`.

## Next steps

```sh
./spacemosquito/spacemosquito init
./spacemosquito/spacemosquito serve
```

Then capture a Confluence session with the extension (see [README](../README.md#capture-a-confluence-session)).

Contributor workflows, tests, and tooling: [DEVELOPMENT.md](DEVELOPMENT.md).
