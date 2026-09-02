# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- Several issues related to self-hosted wikis 

### Changed
- On-disk page directories naming switched to `{confluence-id}-{title}`

## [0.3.3] - 2026-08-21

### Changed
- Firefox releases ship a signed `.xpi` for persistent install via Install Add-on From File (replacing temporary zip + `about:debugging` for end users).
- Browser extension display name is **Space Mosquito**.

## [0.3.2] - 2026-08-04

### Fixed
- Firefox extension Capture Session fixes.

## [0.3.1] - 2026-08-04

### Fixed
- Firefox extension Capture Session fixes.

## [0.3.0] - 2026-08-04

### Added
- `spacemosquito crawl-page <space-key> <confluence-id>` refreshes one page via the REST API, always overwriting local text and assets.

### Fixed
- Session Validate fixes.
- `GET /api/cron/config` panics fixes.

## [0.2.0] - 2026-08-03

### Added
- GitHub Releases and local `build-release.sh` publish Firefox/Chrome extension zips.

### Fixed
- Crawl Cancel now really cancels the started crawl.

### Changed
- Crawl skips already-downloaded non-empty assets. `crawl --force` re-scrapes every page and re-downloads every asset.

## [0.1.0] - 2026-08-02

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
- Asset downloads (images, diagrams, attachments) now send the session cookies, so SSO-protected instances return the file instead of a login page. Responses that are HTML (a login/redirect page) are rejected as a loud error rather than saved as a corrupt image.
- "Page Not Found" errors by using direct API extraction.
- "Device or resource busy" mount errors.
- Standardized MCP JSON-RPC handshake.
