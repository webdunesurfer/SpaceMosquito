# Icon variants (side profile family)

Base pick: [`02-side-profile.svg`](02-side-profile.svg). Four siblings share the same side-mosquito pose.

| File | Style |
|------|--------|
| [`02-side-profile.svg`](02-side-profile.svg) | **Base** - classic line side view |
| [`03-side-filled.svg`](03-side-filled.svg) | Solid fill of the same pose |
| [`04-side-bold.svg`](04-side-bold.svg) | Thicker strokes, fewer details |
| [`05-side-detailed.svg`](05-side-detailed.svg) | Extra wing veins / legs / segments |
| [`06-side-flying.svg`](06-side-flying.svg) | Same mark, slight upward flight tilt |

## Spec (ready to drop in)

- Source art: SVG (`06-side-flying.svg` → `assets/icon.svg`)
- **Shipped toolbar/manifest icons:** PNG 16 / 32 / 48 / 128 (`assets/icon-{size}.png`) — required for Chrome
- Color: muted sage green `#4a7a5c` (active); inactive gray `#8a93a0`
- Regenerate: `./scripts/render-extension-icons.sh` (optional; needs **librsvg** `rsvg-convert`). Committed PNGs are used if the tool is absent.

## Apply a pick

Update the SVG source (or copy a candidate over `06-side-flying.svg` / re-point the render script), then:

```sh
./scripts/render-extension-icons.sh
make build-extensions
```

Preview: [`preview.html`](preview.html).
