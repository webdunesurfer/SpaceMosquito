# Task: Migrate from Web Scraping to Confluence REST API

## Objective
Replace the current headless browser-based scraping logic (`go-rod`) with direct Atlassian Confluence REST API calls. This will improve reliability, speed, and resolve authentication issues where the headless browser fails to render pages or maintain session state.

## Strategic Intent
The current "Space Detection" and "Page Scraping" logic relies on DOM inspection and headless rendering, which is fragile and easily blocked by CSP or authentication mismatches. By switching to the official REST API, we can:
1.  **Guarantee Data Integrity**: Fetch original storage format (XHTML) or rendered view.
2.  **Increase Speed**: Avoid the overhead of starting a browser, loading CSS/JS, and waiting for "stable" state.
3.  **Resolve Auth Failures**: Use the same captured cookies to directly authorize HTTP requests.

## Implementation Plan

### 1. Unified Authentication Layer
Modify the `session` package to provide a standard `http.Header` containing the captured cookies.
*   **Action**: Add a helper method `sess.AsHeaders()` that returns a `map[string]string` with the `Cookie` header.
*   **XSRF Protection**: Include the `X-Atlassian-Token: no-check` header to bypass XSRF checks for simple GET/POST requests.

### 2. Space Discovery via API
Replace the current recursive DOM traversal with the Confluence Content API.
*   **Cloud Endpoint**: `GET /wiki/api/v2/spaces/{id}/pages` (V2) or `GET /wiki/rest/api/space/{key}/content/page` (V1).
*   **Server/DC Endpoint**: `GET /rest/api/content?spaceKey={key}&type=page`.
*   **Logic**: Paginate through the results until all page IDs and URLs are collected.

### 3. Content Extraction via API
Replace `scraper.ScrapePage` (which uses `rod`) with a direct HTTP GET request.
*   **Endpoint**: `GET /rest/api/content/{id}?expand=body.storage,version,ancestors`.
*   **Benefit**: Storage format is much cleaner for text extraction (BM25) than rendered HTML full of scripts and CSS.
*   **Fallback**: If `body.storage` is hard to process, use `body.view` for pre-rendered HTML.

### 4. Hybrid Support
Keep the `scraper` package but refactor it to accept an "Extractor" interface.
*   `WebExtractor`: Existing `go-rod` logic (kept for non-API compatible sites).
*   `APIExtractor`: New REST API-based logic.

## Open Questions & Concerns

### 1. Cookie Scope & "HttpOnly"
*   **Concern**: Can the browser extension capture *all* necessary session cookies if they are marked `HttpOnly`? 
*   **Note**: Our current Chrome/Firefox manifest has the `cookies` permission, which *should* allow this, but some corporate security policies might rotate `JSESSIONID` or tie it to a specific User-Agent.

### 2. API Version Fragmentation
*   **Question**: How do we reliably detect if a custom domain is Cloud vs. Server?
*   **Proposed Solution**: Perform a "capability probe" during session validation to check if `/wiki/` exists or which REST path returns a 200.

### 3. Asset Downloading (Images/Attachments)
*   **Concern**: The API returns URLs for images. Will direct HTTP requests for these assets also work with the injected cookies?
*   **Risk**: Confluence sometimes serves assets from a different subdomain (e.g., `media-api.atlassian.com`) which might require different cookies or tokens.

### 4. Rate Limiting
*   **Question**: Does the REST API have stricter rate limits than the web UI?
*   **Concern**: A full crawl of 1000+ pages might trigger 429 Too Many Requests if not carefully throttled.

### 5. Permission Gaps
*   **Concern**: Are there any pages visible in the Web UI that are *not* accessible via the REST API for the same user? (Usually rare, but possible with certain macros or App-specific content).

## Expected Outcome
The "Page Not Found" errors in search results will be eliminated. The tool will transition from a "Pirate Scraper" to a "Local API Mirror," making the data extraction process robust against UI changes.
