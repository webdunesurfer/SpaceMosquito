# Extension — animated crawl toolbar icon (mosquito sucking blood)

- **Task ID:** `extension-crawl-icon-animation`
- **Status:** done
- **Parent:** —
- **Blocked by:** [`extension-green-brand`](extension-green-brand.md)

## Goal

While **any crawl is running**, show a **looping** toolbar icon of the mosquito sucking blood on **every tab**. Overrides green/gray; restores when crawl ends or backend is down.

## Shipped

- 6-frame PNG loop (`icon-crawl-f0`…`f5` @ 16/32) in FF + Chrome assets; sources under [`icons/`](icons/).
- Background: poll `GET /api/crawl` every 2s; any `running`/`pending` → global + per-tab frame cycle @ ~6 fps; start immediately on extension crawl; stop on idle or fetch failure → green/gray.
- `scripts/render-extension-icons.sh` regenerates crawl frames from SVG.

## Decisions (locked)

| # | Decision |
|---|----------|
| 1–5 | Any backend crawl; all tabs; end on complete/fail/cancel/down; loop ≠ progress |
| A–E | 6 frames, green+blood `#c45c5c`, ~6 fps, global setIcon, agent art |

## Done when

- [x] Any active crawl → looping sucking-blood toolbar icon on all tabs
- [x] Crawl ends or backend unreachable → green/gray restored
- [x] Animation loops independently of progress
- [x] Firefox + Chrome; CHANGELOG
- [x] Art accepted and shipped as toolbar PNGs
