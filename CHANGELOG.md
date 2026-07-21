# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Rule-based **Confluence Storage Format → Markdown** converter (`internal/contentmd/csf`): macros (code, panel/info/note/warning/tip, jira, status, expand, toc, draw.io), task lists, native tables/links/emphasis, images and draw.io diagrams extracted as assets, emoticons mapped to Unicode. Replaces the generic converter on the API path; browser-fallback rendered HTML still uses the generic converter, selected by a format sniff.
- `metadata.json` records `body_format` (`storage`/`rendered`); `reindex --content` and `bootstrap import-saved` route each page to the correct converter (sniffing `raw.html` for legacy pages).
- `metadata.json` `diagrams` field for extracted draw.io previews.
- Chrome extension port (Manifest V3).
- REST API-first crawling with headless fallback.
- Multi-flavor support (Cloud / Server / Data Center).
- Standardized MCP SSE transport.
- Variable expansion in `config.yaml`.
- `AGENTS.md` for local development.

### Changed
- Removed the 50 KB `pages.content`/`content.md` truncation cap; full Markdown is stored (disk + DB + FTS).
- Unified all HTTP services on port 8081.
- Refactored session deletion for Docker compatibility.
- Broadened URL detection for custom domains.

### Fixed
- "Page Not Found" errors by using direct API extraction.
- "Device or resource busy" mount errors.
- Standardized MCP JSON-RPC handshake.
