# Phase 3: Playwright Scraper

## Objective
Implement headless Firefox scraping via Playwright to discover and extract all pages in a Confluence space.

## Deliverables
- Playwright Go bindings with Firefox
- Space traversal: sidebar parsing → page discovery
- Page scraping: content extraction (trafilatura-style), asset download
- Clean HTML generation with rewritten URLs
- Full-space crawl orchestration
- Structured logging throughout scraper lifecycle

## Logging Strategy
- Use `logging.Sugar` injected via constructors in all scraper packages
- Log at INFO for page start/end, WARN for retries, ERROR for failures
- Include `space_key`, `page_id`, `page_title` in all page-related log entries
- Include `remote_addr` in HTTP requests during asset download
- Log browser lifecycle events (launch, navigate, close)

## Tasks

### 3.1 — Playwright Setup
- `internal/scraper/scraper.go`:
  - Initialize Playwright: `playwright.New()`
  - Launch Firefox headless: `browser.NewContext()` with persistent context for cookies
  - Configure viewport, user agent (match Firefox on desktop)
  - Handle browser binary location (system install or bundled)
  - **Log browser launch/close events**, log viewport config

### 3.2 — Space Page Discovery
- `internal/scraper/page.go`:
  - Navigate to space root URL
  - Parse sidebar navigation DOM to discover all page links
  - Recursive traversal: for each page, check for sub-pages in sidebar
  - Build a page tree: `{ pageId, title, url, parentId, level }`
  - Handle Confluence's dynamic sidebar (wait for JS rendering)
  - Deduplicate pages by confluence_id
  - **Log space root navigation, page discovery count, duplicates skipped**

### 3.3 — Page Content Extraction (Trafilatura-style)
- `internal/scraper/page.go`:
  - Navigate to each page, wait for full render
  - Use goquery to parse HTML in Go:
    ```go
    doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
    ```
  - Find main content: selectors for `.wiki-content`, `#content`, `#main-content`
  - Strip chrome:
    - Remove: `#header`, `#footer`, `.sidebar`, `.breadcrumbs`, `.toolbar`
    - Remove: macro wrappers (keep inner content)
    - Keep: code blocks (`<pre>`, `<code>`), tables, images
  - Extract text for embedding (plain text from cleaned HTML)
  - Preserve: headings (h1-h6), lists, links, code blocks, tables, images
  - **Log content extraction progress, stripped elements count, text length for embedding**

### 3.4 — Clean HTML Generation
- `internal/storage/writer.go`:
  - Generate `index.html` from cleaned DOM
  - Rewrite URLs:
    - Confluence internal links → local file references
    - CDN image URLs → local `assets/images/`
    - Attachment URLs → local `assets/attachments/`
  - Preserve: CSS classes that affect readability (strip layout classes)
  - Inline critical CSS for offline viewing (optional, can be separate file)
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
    2. Create Playwright Firefox context with session cookies
    3. Discover all pages in space (build page tree)
    4. For each page:
       a. Navigate and extract content
       b. Download assets
       c. Save to disk (clean HTML + raw HTML + metadata)
       d. Store in database
    5. Close browser context
  - Progress reporting: emit events/callbacks for crawl status
  - Error handling: skip failed pages, log errors, continue with next
  - **Log crawl start/end with duration, per-page progress, asset totals, per-page errors, summary stats**

### 3.7 — CLI Integration
- `cmd/cli/main.go`:
  - Command: `space-mosquito crawl <space-url>`
  - Loads config, validates session, runs full crawl
  - Progress output: "Crawling page 15/142: Page Title..."
  - Summary: "Crawl complete: 142 pages, 89 images, 12 attachments"
  - **Use structured logger instead of fmt for progress, include request_id for crawl job**

## Acceptance Criteria
- Full Confluence space can be crawled headlessly with Firefox
- Page tree is correctly discovered (including nested pages)
- Content extraction produces clean, readable HTML
- Assets (images, attachments) are downloaded and linked
- Raw HTML is preserved as fallback
- CLI command `crawl` completes successfully on a test space
- All scraper events are logged with structured fields (page_id, space_key, duration, bytes)
