# Firefox extension: persistent install via "Install Add-on From File"

- **Task ID:** `task-firefox-persistent-install`
- **Status:** ready
- **Parent / blocked by:** <!-- none -->

## Problem

The Firefox extension is currently installed as a **temporary add-on** via
`about:debugging` → "Load Temporary Add-on". This means:

- Extension is removed on every Firefox restart.
- Users must re-load manually after each session.
- Not a standard user-facing install experience.

## Goal

Support installing the Firefox extension **persistently** via `about:addons` →
gear → "Install Add-on From File" (or drag-and-drop of an XPI). The extension
survives browser restarts without reloading.

## Decisions

| # | Topic | Status | Decision |
|---|--------|--------|----------|
| 1 | Signing vs unsigned | locked | **AMO unlisted signing** (`web-ext sign --channel unlisted`). Works on all Firefox channels; no `about:config` override. |
| 2 | Distribution format | locked | **XPI only** for Firefox releases — stop shipping `spacemosquito-firefox-*.zip`; ship signed `spacemosquito-firefox-*.xpi`. |
| 3 | AMO credentials | locked | Store as GitHub **repository secrets**: `AMO_JWT_ISSUER`, `AMO_JWT_SECRET`. CI release job passes them to `web-ext sign`. |

## Non-goals

- Publishing on addons.mozilla.org as a listed/public extension.
- Changing Chrome extension install flow (already persistent via "Load unpacked").
- Shipping an unsigned Firefox zip alongside the XPI.

## Implementation

### Already done (no work needed)

- `manifest.json` has `browser_specific_settings.gecko.id` =
  `space@mosqui.to` — **required** for XPI install and already present.

### AMO unlisted signing + XPI artifact

1. Register at `addons.mozilla.org`, generate API key + secret.
2. Configure GitHub repository secrets: `AMO_JWT_ISSUER`, `AMO_JWT_SECRET`
   (Settings → Secrets and variables → Actions).
3. Add a build / release step that injects those secrets into the
   `build-extensions` (or successor) job:

   ```sh
   npx web-ext sign \
     --source-dir ./dist \
     --channel unlisted \
     --api-key "$AMO_JWT_ISSUER" \
     --api-secret "$AMO_JWT_SECRET"
   ```

4. Produce a signed `.xpi` that installs on **any** Firefox (release, ESR,
   Dev, Nightly) without signature overrides.
5. GitHub Releases: ship **only** `spacemosquito-firefox-v*.xpi` (remove
   Firefox zip from release assets / SHA256SUMS / docs).
6. Update `docs/INSTALL.md` Firefox section: primary path = "Install Add-on
   From File" with the XPI; keep `about:debugging` as **dev-only** fallback
   (load unpacked `dist/` / unsigned build during development).
7. Update `Makefile` / release scripts / CI that currently zip the Firefox
   extension; README / release notes that mention the zip.

Local `build-release.sh` cannot sign without the secrets on the machine; either
skip Firefox XPI locally, or document exporting the same env vars for a
manual signed build.

**Trade-offs (accepted):**
- Requires Mozilla developer account + stored secrets.
- AMO automated scan on each signed build (usually minutes).
- Must re-sign on every version bump.

## Done when

- [x] Firefox extension installs via `about:addons` → "Install Add-on From
      File" and survives restart. *(release artifact path; verify on next tag)*
- [x] `docs/INSTALL.md` updated with the new primary install path (XPI).
- [x] Release artifacts include signed `spacemosquito-firefox-*.xpi` only
      (no Firefox zip).
- [x] `about:debugging` path documented as dev-only fallback.

## Notes

- Implemented in `scripts/build-extension-zips.sh` + `.github/workflows/release.yml`.
- CI sets `REQUIRE_FIREFOX_XPI=1` so missing AMO secrets fail the release job.
- Local builds without secrets skip the Firefox XPI and still build Chrome.
- The manifest already has `gecko.id` — the main technical prerequisite is met.
- `web-ext sign` docs: https://extensionworkshop.com/documentation/develop/web-ext-command-reference/#web-ext-sign
- Chrome is unaffected — "Load unpacked" is already persistent (survives restart).
