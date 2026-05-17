# ADR-010: HTML Content Extraction — Trafilatura-style in Go

- **Status**: Accepted
- **Date**: 2025-01-17
- **Context**: Confluence pages contain significant HTML "chrome" (navigation, sidebar, footer, macros) that must be stripped to extract the main content for embedding and clean HTML generation.
- **Decision**: Implement trafilatura-style extraction in Go using goquery for DOM parsing with heuristic-based content identification
- **Rationale**:
  - Trafilatura's approach (find the main content node, strip surrounding noise) is well-proven for wiki/encyclopedia content
  - Confluence's HTML structure is relatively predictable: content is typically in `<article>`, `<main>`, or a specific `<div>` with a content class
  - Go's `goquery` (jQuery-like selectors) provides efficient DOM traversal
  - Heuristic rules can handle Confluence-specific markup (wiki markup blocks, macros, info panels)
  - No external dependencies — pure Go implementation
- **Extraction approach**:
  1. Use goquery to parse the rendered HTML
  2. Find the main content container using CSS selectors (`.wiki-content`, `#content`, `<article>`)
  3. Strip navigation elements (sidebar, breadcrumbs, footer, header)
  4. Strip macro containers (info panels, code blocks should be preserved)
  5. Extract text content for embedding
  6. Generate clean HTML with rewritten URLs for offline navigation
- **Alternatives considered**:
  - Import a Python trafilatura library via subprocess — adds Python dependency, slows execution
  - Import an existing Go HTML extraction library — few mature options; most are specialized (e.g., newspaper3g)
  - Use Confluence's built-in export API — requires different auth flow and doesn't give us raw HTML control
- **Consequences**:
  - Heuristics may need tuning for different Confluence versions and themes
  - Macro content (e.g., Jira embeds, charts) may not extract cleanly as static HTML
  - Code blocks and tables should be preserved with their formatting in the clean HTML output
