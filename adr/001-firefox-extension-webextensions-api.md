# ADR-001: Browser Extension Technology

- **Status**: Accepted
- **Date**: 2025-01-17
- **Context**: We need a browser extension to capture Confluence session cookies, handle authentication, and enable interactive page-by-page scraping
- **Decision**: Use TypeScript with the WebExtensions API (Manifest V3) for the Firefox extension
- **Rationale**:
  - Firefox extensions must run in a sandboxed JavaScript environment; Go is not an option
  - The WebExtensions API is the standard for Firefox extensions
  - TypeScript provides type safety for session management, scraping logic, and API communication
  - Manifest V3 with service worker background scripts is the current standard
  - The `web-ext` CLI tool provides straightforward development and testing
- **Alternatives considered**:
  - Rust-based WebExtensions SDK — heavier toolchain, not needed for this scope
  - Native Messaging host — adds unnecessary complexity for what a simple API call can solve
- **Consequences**:
  - Extension must be loaded as a temporary extension in Firefox during development
  - Will be distributed as an XPI file for production installs
  - Service worker background scripts have lifetime limits; long-running scrapes need to be chunked or delegated to the backend
