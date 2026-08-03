# Install

Install SpaceMosquito from a [GitHub Release](https://github.com/webdunesurfer/SpaceMosquito/releases).

## Requirements

- A release asset for your OS/arch (`spacemosquito-*`)
- Firefox or Chrome (for the Pirate Mosquito extension)

## Binary

1. Download the binary for your platform and optionally `SHA256SUMS` from the release.
2. Verify the checksum against `SHA256SUMS`.
3. Install onto your `PATH` (example for macOS/Linux):

```sh
chmod +x spacemosquito-darwin-arm64
sudo mv spacemosquito-darwin-arm64 /usr/local/bin/spacemosquito
```

## Browser extension

Each release also includes:

- `spacemosquito-firefox-*.zip`
- `spacemosquito-chrome-*.zip`

Unzip so `manifest.json` is at the top of the folder (do not load a parent directory that only contains another folder).

### Firefox

1. Unzip `spacemosquito-firefox-*.zip` into a folder.
2. Open `about:debugging` → **This Firefox** → **Load Temporary Add-on…**
3. Select that folder’s `manifest.json`.

Temporary add-ons are removed when Firefox restarts; load again after a restart.

### Chrome

1. Unzip `spacemosquito-chrome-*.zip` into a folder.
2. Open `chrome://extensions` → enable **Developer mode** → **Load unpacked**
3. Select that folder (the one that contains `manifest.json`).

## Next steps

```sh
spacemosquito init
spacemosquito serve
```

Then capture a Confluence session with the extension (see [README](../README.md#capture-a-confluence-session)).
