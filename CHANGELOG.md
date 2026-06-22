# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Chrome extension port (Manifest V3).
- REST API-first crawling with headless fallback.
- Multi-flavor support (Cloud / Server / Data Center).
- Standardized MCP SSE transport.
- Variable expansion in `config.yaml`.
- `AGENTS.md` for local development.

### Changed
- Unified all HTTP services on port 8081.
- Refactored session deletion for Docker compatibility.
- Broadened URL detection for custom domains.

### Fixed
- "Page Not Found" errors by using direct API extraction.
- "Device or resource busy" mount errors.
- Standardized MCP JSON-RPC handshake.
