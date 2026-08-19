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
| 1 | Signing vs unsigned | open | Option A: AMO self-hosted signing (`web-ext sign --channel unlisted`) — works on all Firefox channels. Option B: Unsigned XPI — requires Dev Edition / Nightly / ESR + `xpinstall.signatures.required = false`. |
| 2 | Distribution format | open | `.xpi` file in GitHub Releases (rename current `.zip` to `.xpi`, or produce both)? |

## Non-goals

- Publishing on addons.mozilla.org as a listed/public extension.
- Changing Chrome extension install flow (already persistent via "Load unpacked").

## Implementation

### Already done (no work needed)

- `manifest.json` has `browser_specific_settings.gecko.id` =
  `spacemosquito@vkh.dev` — **required** for XPI install and already present.

### Option A: AMO unlisted signing (recommended if viable)

1. Register at `addons.mozilla.org`, generate API key + secret.
2. Add a build step (CI or Makefile):

   ```sh
   npx web-ext sign \
     --source-dir ./dist \
     --channel unlisted \
     --api-key "$AMO_JWT_ISSUER" \
     --api-secret "$AMO_JWT_SECRET"
   ```

3. This produces a signed `.xpi` that installs on **any** Firefox (release,
   ESR, Dev, Nightly) without signature overrides.
4. Upload signed XPI to GitHub Releases alongside the unsigned zip.
5. Update `docs/INSTALL.md` Firefox section.

**Trade-offs:**
- Requires Mozilla developer account + stored secrets.
- AMO review (unlisted = automated scan, no human review, usually minutes).
- Must re-sign on every version bump.

### Option B: Unsigned XPI (simpler, limited audience)

1. Rename or additionally produce `.xpi` from the existing zip build:

   ```sh
   cd dist && zip -r ../spacemosquito-firefox-v$VERSION.xpi .
   ```

2. Document that users need Firefox Developer Edition / Nightly / ESR and must
   set `xpinstall.signatures.required = false` in `about:config`.
3. Update `docs/INSTALL.md` with the new install path.

**Trade-offs:**
- Does **not** work on standard Firefox release channel.
- Requires `about:config` change — extra friction, but one-time.
- No external accounts or secrets needed.

### Common changes (both options)

- **`docs/INSTALL.md`** — rewrite Firefox section: primary path = "Install
  Add-on From File"; keep `about:debugging` as fallback for development.
- **`Makefile` / CI** — add `xpi` target or rename artifact extension.
- **README** — mention persistence.

## Done when

- [ ] Firefox extension installs via `about:addons` → "Install Add-on From
      File" and survives restart.
- [ ] `docs/INSTALL.md` updated with the new primary install path.
- [ ] Release artifacts include the installable XPI (signed or unsigned per
      decision #1).
- [ ] `about:debugging` path documented as dev-only fallback.

## Notes

- The manifest already has `gecko.id` — the main technical prerequisite is met.
- `web-ext sign` docs: https://extensionworkshop.com/documentation/develop/web-ext-command-reference/#web-ext-sign
- Unsigned XPI requires: Dev Edition / Nightly / ESR + `xpinstall.signatures.required = false`.
- Standard Firefox release channel **cannot** disable signature checking (since Firefox 47).
- Chrome is unaffected — "Load unpacked" is already persistent (survives restart).
