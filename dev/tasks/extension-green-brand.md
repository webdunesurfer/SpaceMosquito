# Extension — green brand palette

- **Task ID:** `extension-green-brand`
- **Status:** done
- **Parent:** —
- **Blocked by:** [`extension-themes`](extension-themes.md), [`extension-toolbar-detect-icon`](extension-toolbar-detect-icon.md)

## Goal

Retheme Firefox + Chrome extensions to a **green brand**. App icon (active
Confluence / popup header) is green; gray inactive toolbar icon stays gray.
Red reserved for progress / stop / danger / errors; semantic session/backend
dots unchanged.

## Locked brand hexes

| Token / surface | Light | Dark |
|-----------------|-------|------|
| `--brand` / icon stroke | `#4a7a5c` | `#4a7a5c` (theme-agnostic mark) |
| `--primary` | `#3d6b50` | `#7aaf8e` |
| `--primary-hover` | `#325643` | `#91c4a4` |
| Inactive toolbar | `#8a93a0` | same |
| `--progress-fill` | `#6b1c1c` (unchanged) | `#c45c5c` (unchanged) |
| `--danger` / hints / banners | danger family (unchanged) | same |
| `--success` (session valid) | `#2f7d57` (unchanged) | `#4caf7a` (unchanged) |
| `--cron-enabled` | amber (unchanged) | amber (unchanged) |

Brand sage is intentionally distinct from `--success` emerald so “valid” ≠ “brand”.

## Shipped

- Popup CSS tokens (FF + Chrome): brand/primary + current-row / tab-hover greens; hint/banner stay danger.
- Page refresh (`.btn-sync`) uses `--primary` (green), not progress red.
- `06-side-flying.svg` + `assets/icon.svg` + `icon-active-*.png` → `#4a7a5c`.
- Inactive gray SVG/PNG unchanged.
- `./scripts/render-extension-icons.sh` updated for green→gray.

## Done when

- [x] Brand accents + active/app icon are green (light + dark)
- [x] Errors, session/backend status semantics, progress bar, start/stop unchanged in intent
- [x] Inactive toolbar icon still gray; Confluence tab active = green
- [x] Both browsers; CHANGELOG note
