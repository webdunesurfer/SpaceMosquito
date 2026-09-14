# Extension — light/dark themes & icon

- **Task ID:** `extension-themes`
- **Status:** done
- **Parent:** —
- **Blocked by:** —

## Goal

Ship **light** and **dark** popup themes that **auto-follow** the browser color
scheme (fallback **light** if undetectable), plus a **single** theme-agnostic
extension icon derived from the redesign mockup mosquito. No in-UI theme picker
in v1.

## Decisions

| # | Topic | Status | Decision |
|---|--------|--------|----------|
| 1 | Themes | locked | Exactly **two**: light and dark. Color tokens designed **from scratch** (do not preserve current teal/navy palette). |
| 2 | Selection | locked | Autodetect browser/OS preferred color scheme (`prefers-color-scheme`). If unknown / unsupported → **light**. |
| 3 | Manual switch | locked | **None** in v1 (may add later). |
| 4 | Progress bar | locked | Keep a **red** family for crawl progress. **Darker** red on light theme; **lighter** red on dark theme. |
| 5 | Scope | locked | Firefox + Chrome popup CSS (+ any shared token vars). Both browsers stay duplicated (no `shared/` package). |
| 6 | Icon art | locked | Line-art mosquito from [`extension-redesign-mockups/01-page-healthy.png`](extension-redesign-mockups/01-page-healthy.png) (header mark). Redraw as SVG if extract is messy; keep the same silhouette spirit. |
| 7 | Icon variants | locked | **One** icon for both browsers, **theme-agnostic**. Stroke `#9b4d4d` (muted brick red). Same asset for toolbar + manifest `icons` + popup header. |

## Shipped

- CSS tokens on `:root` (light default) + `@media (prefers-color-scheme: dark)`.
- Progress / sync / stop use `--progress-fill` (`#6b1c1c` light, `#c45c5c` dark).
- Temporary mosquito SVG in `assets/icon.svg`; **pick a final mark** from [`icons/`](icons/) (6 variants + preview).
- Chosen icon: [`icons/06-side-flying.svg`](icons/06-side-flying.svg) (red) + [`icons/06-side-flying-inactive.svg`](icons/06-side-flying-inactive.svg) (gray `#8a93a0`). Manifest default = inactive PNGs; Confluence tabs use active PNGs via `setIcon`. Regenerate with `./scripts/render-extension-icons.sh`.
- No Settings theme toggle.

## Done when

- [x] Light + dark popup themes auto-follow `prefers-color-scheme`; unknown → light
- [x] No manual theme UI
- [x] Progress bar stays red (darker on light, lighter on dark)
- [x] Single muted-red mosquito icon shipped for Firefox + Chrome (theme-agnostic)
- [x] Firefox + Chrome updated; CHANGELOG note

## Non-goals

- User-facing theme picker / “system | light | dark” setting
- Dual light/dark toolbar icons (`theme_icons` / runtime `setIcon`)
- Full design-system docs site
- Merging Chrome/Firefox into a shared package
- Restyling the backend/CLI
