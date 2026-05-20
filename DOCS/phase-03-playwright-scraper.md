# Phase 3: go-rod Scraper

> **Status**: Completed. This phase originally planned chromedp (ADR-012), but was implemented with go-rod due to chromedp sandbox failures in Colima vz driver (ADR-013).

## Objective
Implement headless Chromium scraping via go-rod to discover and extract all pages in a Confluence space.

## Logging Strategy
- Use `logging.Sugar` injected via constructors in all scraper packages
- Log at INFO for page start/end, WARN for retries, ERROR for failures
- Include `space_key`, `page_id`, `page_title` in all page-related log entries
- Include `remote_addr` in HTTP requests during asset download
- Log browser lifecycle events (context creation, navigation, close)

## Tasks

### 3.1 — go-rod Setup
- `internal/scraper/scraper.go`:
  - Launch browser via `rod.New().Headless(true).BrowserBinary("/usr/bin/chromium").NoSandbox(true).MustStart()`
  - Handle browser lifecycle: `Browser.MustClose()` on shutdown
  - **Log browser start/close events**
  - No Xvfb or DISPLAY required — Chromium headless is native
  - See ADR-013 for why go-rod was chosen over chromedp

### 3.2 — Space Page Discovery
- `internal/scraper/discovery.go`:
  - Navigate to space root URL via `page.MustNavigate(url)`
  - Wait for sidebar to render: `page.MustWaitStable()` then query `#sidebar` / `.page-tree`
  - Parse sidebar navigation DOM to discover all page links
  - Recursive traversal: for each page, check for sub-pages in sidebar
  - Build a page tree: `{ pageId, title, url, parentId, level }`
  - Handle Confluence's dynamic sidebar (wait for JS rendering)
  - Deduplicate pages by confluence_id
  - **Log space root navigation, page discovery count, duplicates skipped**

### 3.3 — Page Content Extraction (Trafilatura-style)
- `internal/scraper/page.go`:
  - Navigate to each page, wait for full render via `page.MustWaitStable()`
  - Extract raw HTML via `page.MustElement("#page-content").MustHTML()`
  - Use goquery to parse HTML in Go:
    ```go
    doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
    ```
  - Find main content: selectors for `#page-content`, `.wiki-content`
  - Strip chrome:
    - Remove: `#header`, `#footer`, `.sidebar`, `.breadcrumbs`, `.toolbar`
    - Remove: macro wrappers (keep inner content)
    - Keep: code blocks (`<pre>`, `<code>`), tables, images
  - Extract text (plain text from cleaned HTML)
  - Preserve: headings (h1-h6), lists, links, code blocks, tables, images
  - **Log content extraction progress, stripped elements count, text length**

### 3.4 — Clean HTML Generation
- `internal/storage/writer.go`:
  - Generate clean HTML from extracted DOM
  - Rewrite URLs:
    - Confluence internal links → local file references
    - CDN image URLs → local `assets/images/`
    - Attachment URLs → local `assets/attachments/`
  - Preserve: CSS classes that affect readability (strip layout classes)
  - **Log HTML generation with byte size, URL rewrites count**

### 3.5 — Asset Download
- `internal/storage/asset.go`:
  - `DownloadImage(url, destPath)` — download image, save with hash-based filename
  - `DownloadAttachment(url, destPath)` — download file attachment
  - Track downloaded assets in metadata.json
  - Rate limiting: respect Confluence server, add delays between requests
  - Retry logic with exponential backoff
  - **Log each asset download (URL, bytes, status), rate limit wait times, retry attempts**

### 3.6 — Crawl Orchestration
- `internal/scraper/scraper.go`:
  - `CrawlSpace(url string, session *session.Session) error`
  - Flow:
    1. Validate session
    2. Set session cookies via `Browser.MustSetCookies()` (CDP `Storage.setCookies`)
    3. Discover all pages in space (build page tree)
    4. For each page:
       a. Navigate and extract content
       b. Download assets
       c. Save to disk (clean HTML + raw HTML + metadata)
       d. Store in database
    5. Close browser
  - Progress reporting: emit events/callbacks for crawl status
  - Error handling: skip failed pages, log errors, continue with next
  - **Log crawl start/end with duration, per-page progress, asset totals, per-page errors, summary stats**

### 3.7 — CLI Integration
- `cmd/cli/main.go`:
  - Command: `spacemosquito crawl <space-url>`
  - Loads config, validates session, runs full crawl
  - Progress output: "Crawling page 15/142: Page Title..."
  - Summary: "Crawl complete: 142 pages, 89 images, 12 attachments"
  - **Use structured logger instead of fmt for progress, include request_id for crawl job**

## Acceptance Criteria
- Full Confluence space can be crawled headlessly with Chromium
- Page tree is correctly discovered (including nested pages)
- Content extraction produces clean, readable HTML
- Assets (images, attachments) are downloaded and linked
- Raw HTML is preserved as fallback
- CLI command `crawl` completes successfully on a test space
- All scraper events are logged with structured fields (page_id, space_key, duration, bytes)
