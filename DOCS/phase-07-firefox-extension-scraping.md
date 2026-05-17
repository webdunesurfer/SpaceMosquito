# Phase 7: Firefox Extension — Scraping

## Objective
Extend the Firefox extension with page-by-page scraping capabilities, crawl progress tracking, and backend API integration.

## Deliverables
- Crawl initiation from extension popup
- Page-by-page scraping with progress display
- Crawl status monitoring
- Error handling and retry

## Tasks

### 7.1 — Crawl Initiation
- `popup/popup.ts`:
  - "Start Crawl" button triggers crawl of current space
  - Detect current space URL from `window.location`
  - Send `POST /api/crawl` with `{ space_url, depth: 'all' }`
  - Receive job ID, start polling for progress

### 7.2 — Scraping Orchestration
- `background.ts`:
  - `startCrawl(spaceUrl)` — creates Playwright Firefox context
  - Uses stored session cookies (from Phase 2)
  - Discovers pages via sidebar (same logic as Phase 3)
  - For each page:
    1. Navigate and wait for render
    2. Extract content (trafilatura-style via goquery on backend — extension sends HTML to backend)
    3. Send extracted data to backend API
    4. Update progress in `chrome.storage.local`
  - Report completion to user

### 7.3 — Content Extraction (Extension Side)
- `content.ts`:
  - `extractPageContent()` — extract visible content from current page
  - Use DOM queries to find main content area
  - Return `{ title, html, text, images: [...], attachments: [...] }`
  - Called by background script for each crawled page
  - Extension-side extraction is a fallback; primary scraping is backend-driven via Playwright

### 7.4 — Progress Tracking
- `chrome.storage.local`:
  - `crawl_state: { jobId, status: 'running'|'paused'|'completed'|'failed', current: number, total: number, currentPage: string, errors: [...] }`
  - Background updates state after each page
  - Popup polls every 2 seconds: `fetch('/api/crawl/status')` (Phase 2 API endpoint)
- Popup displays:
  - Progress bar: "Page 23/142: API Documentation"
  - Stats: "Images downloaded: 8, Attachments: 2"
  - Error count and last error message
  - Pause/Resume button
  - "View saved files" link

### 7.5 — Backend Crawl API
- `internal/api/handler.go`:
  - `POST /api/crawl` — start crawl job
    - Body: `{ "space_url": string, "options": { "depth": "all"|"shallow" } }`
    - Returns: `{ "job_id": string }`
    - Starts Playwright scraper (Phase 3) in background goroutine
  - `GET /api/crawl/status/<job_id>` — get job status
    - Returns: `{ "status": "running", "current": 23, "total": 142, "page": "API Documentation" }`
  - `POST /api/crawl/pause/<job_id>` — pause current crawl
  - `POST /api/crawl/resume/<job_id>` — resume paused crawl
  - `DELETE /api/crawl/<job_id>` — cancel crawl

### 7.6 — Crawl Job Management
- `internal/scraper/scraper.go`:
  - `CrawlJob` struct: `{ ID, SpaceURL, Status, Current, Total, Pages, Errors, Context }`
  - Job registry: `map[string]*CrawlJob`
  - Jobs run in goroutines
  - Pause: close context, save progress
  - Resume: restore context from saved state
  - Completion: trigger embedding generation (Phase 4)

### 7.7 — Error Handling
- Extension-side:
  - Network errors → show toast, log to console
  - Auth expiry → show "Session expired, re-authenticate" message
  - Crawl failure → show error details, allow retry
- Backend-side:
  - Per-page error logging with retry (3 attempts)
  - Failed pages listed in job status
  - Crawl continues with next page on error (non-fatal)

### 7.8 — Settings
- `popup/popup.html` — Settings panel:
  - Output directory (for local file storage)
  - Embedding model selection (nomic-embed-text, openai)
  - Auto-embed after crawl (toggle)
  - Session management (view/expire status, re-authenticate)

## Acceptance Criteria
- User can click "Start Crawl" in extension popup
- Crawl progresses page by page with visible progress
- Popup shows current page, progress bar, and stats
- Crawl can be paused and resumed
- Errors are reported without stopping the crawl
- Backend API handles crawl jobs with status tracking
