# ADR-004: Headless Browser for Cron Scraper

- **Status**: Accepted
- **Date**: 2025-01-17
- **Context**: The cron job needs to scrape Confluence pages headlessly. We need a browser engine that reliably renders JavaScript-heavy pages like Confluence.
- **Decision**: Use Playwright with Firefox for headless scraping, matching the Firefox extension for rendering consistency
- **Rationale**:
  - Playwright provides a high-level, reliable API for browser automation across browsers
  - Firefox ensures the headless scraper renders Confluence pages identically to the Firefox extension
  - Playwright handles dynamic content, AJAX loading, and Confluence's JavaScript rendering
  - Confluence is heavily JavaScript-dependent; raw HTTP requests with Go would fail to capture rendered content
  - Playwright's persistent context preserves cookies from the session store, enabling authenticated scraping
- **Alternatives considered**:
  - Playwright with Chromium — more reliable headless mode and lighter, but potential rendering differences vs Firefox extension
  - Go-native HTTP + HTML parsing — would fail on JavaScript-rendered Confluence content; would require maintaining a wiki markup parser
  - Selenium — heavier, slower, and less mature Go bindings than Playwright
- **Consequences**:
  - Firefox binary must be available in the Docker image (adds ~200MB)
  - Headless Firefox can be slightly slower than Chromium in some cases
  - Playwright Go bindings are less mature than the Node.js version, but functional for our use case
