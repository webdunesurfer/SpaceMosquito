# ADR-012: Browser Automation Library — Superseded

- **Status**: Superseded by ADR-013
- **Date**: 2026-05-17
- **Updated**: 2026-05-17
- **Context**: This ADR recommended chromedp over Playwright for headless browser automation.
- **Decision**: chromedp was recommended but later replaced by go-rod (see ADR-013)
- **Rationale (historical)**:
  - chromedp is a pure Go library communicating via Chrome DevTools Protocol
  - No Node.js, no Xvfb, simpler Docker image than Playwright
  - Faster startup than Playwright (seconds vs 30+ seconds)
- **Why superseded**: During implementation in Docker (Colima with vz driver), chromedp's `NoSandbox` flag caused sandbox namespace failures (EPERM). go-rod's `launcher.NoSandbox(true)` with explicit `Bin("/usr/bin/chromium")` works reliably in the same environment. See ADR-013 for the go-rod decision.
- **See also**: ADR-004 (headless browser selection, updated for go-rod), ADR-013 (go-rod over chromedp)
