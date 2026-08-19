# ADR-004: Headless Browser for Cron Scraper

- **Status:** Accepted (updated with go-rod)
- **Date:** 2025-01-17
- **Updated:** 2026-05-17
- **Related:** ADR-013 (go-rod over chromedp)

## Context

The cron job needs to scrape Confluence pages headlessly. We need a browser engine that reliably renders JavaScript-heavy pages like Confluence, with minimal host dependencies.

## Decision

Use go-rod (Go-native Chrome DevTools Protocol) with Chromium for headless scraping.

- go-rod is a pure-Go library — zero Node.js dependency, no driver downloads, no version pinning
- Uses Chrome DevTools Protocol directly; no Xvfb or DISPLAY required for headless mode
- Chromium via rod download or CHROMIUM_PATH
- Well-maintained, good API ergonomics with type-safe element queries and fluent waits
- Confluence is heavily JavaScript-dependent; raw HTTP requests with Go would fail to capture rendered content
- go-rod supports cookie injection via CDP `Storage.setCookies`, enabling authenticated scraping
- Chromium headless is the de facto standard for server-side scraping; rendering consistency across platforms
- go-rod's `MustWaitStable()` provides robust page load detection for JS-heavy pages

## Alternatives considered

| Option | Why not |
|--------|---------|
| Playwright with Firefox | Sandbox/Xvfb requirements and driver version mismatches |
| Playwright with Chromium | Requires Node.js bridge, driver downloads, and version management |
| Selenium | Heavier, slower, and less mature Go bindings |
| chromedp | Sandbox namespace failures (EPERM); see ADR-013 |
| Go-native HTTP + HTML parsing | Would fail on JavaScript-rendered Confluence content |

## Consequences

- Chromium must be available on the host (rod auto-download under the data dir, or CHROMIUM_PATH)
- Uses Chrome DevTools Protocol — may need attention if Confluence uses non-standard Chrome features
- Pure-Go stack means fewer moving parts and easier debugging
- No Xvfb needed; Chromium runs headless natively without a display server
