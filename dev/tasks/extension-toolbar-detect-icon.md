# Extension — Confluence-aware toolbar icon (red / gray)

- **Task ID:** `extension-toolbar-detect-icon`
- **Status:** done
- **Parent:** —
- **Blocked by:** [`extension-themes`](extension-themes.md) (PNG toolbar icons shipped)

## Goal

Show a **red** toolbar icon on Confluence tabs and a **gray** icon otherwise, so
detection is visible without opening the popup. Firefox + Chrome.

## Decisions

| # | Topic | Status | Decision |
|---|--------|--------|----------|
| 1 | Signal | locked | **Red** = current tab looks like Confluence; **gray** = not (or URL unknown). |
| 2 | Detection | locked | Reuse existing `isConfluenceUrl` (background). |
| 3 | Scope of icon | locked | **Toolbar / action icon only** (per tab). Popup header stays brand red (`icon.svg`). |
| 4 | API | locked | `action.setIcon({ tabId, path })` on activate + URL/complete update. Manifest default = gray. |
| 5 | Assets | locked | `icon-active-*.png` (red `#9b4d4d`) / `icon-inactive-*.png` (gray `#8a93a0`); render via `scripts/render-extension-icons.sh`. |
| 6 | Gray tone | locked | `#8a93a0`. |
| 7 | Restricted pages | locked | Missing URL → gray. |

## Shipped

- Gray SVG source: [`extension-themes/icons/06-side-flying-inactive.svg`](extension-themes/icons/06-side-flying-inactive.svg).
- Background listeners: `tabs.onActivated`, `tabs.onUpdated`; refresh on install/startup.
- Firefox + Chrome manifests use inactive PNGs as `default_icon` / `icons`.

## Done when

- [x] Confluence tab → red toolbar icon; non-Confluence / unknown → gray
- [x] Updates on tab switch and in-tab navigation (Firefox + Chrome)
- [x] PNG assets for both colors; popup header unchanged (brand red)
- [x] CHANGELOG note

## Non-goals

- Encoding session validity in the toolbar icon
- Badge text / count on the action
- Per-host color coding beyond Confluence vs not
- theme_icons / OS light-dark toolbar pairs
