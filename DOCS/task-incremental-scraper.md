# Task: Efficient Incremental Scraper

## Objective
Update the crawler to perform incremental updates by checking page versions before downloading content. Skip processing for pages that have not changed since the last crawl.

## Data Structure Changes
Yes, a schema change is required to track the Confluence page version.

1.  **Database Schema**:
    *   Create a new migration (`007_page_version.up.sql` / `down.sql`).
    *   Add column: `ALTER TABLE pages ADD COLUMN version INT DEFAULT 0;`.
2.  **Go Models**:
    *   `internal/db/models.go`: Add `Version int` to `type Page struct`.
    *   `internal/scraper/scraper.go`: Add `Version int` to `type Page struct`.

## Implementation Sequence

### 1. Database Layer Updates
*   **File**: `internal/db/models.go`
*   **Change**: 
    *   Update `UpsertPage` SQL to include `version`.
    *   Update `ON CONFLICT` block to `version=EXCLUDED.version`.
    *   Update `GetPage` and `ListPages` SQL queries and `Scan()` calls to include the new `version` column.

### 2. Discovery API Update
*   **File**: `internal/scraper/discovery.go`
*   **Change in `fetchPageListAPI`**:
    *   Modify the `apiURL` to include the `expand=version` parameter.
        *   Cloud: `.../content/page?expand=version&limit...`
        *   Server/DC: `.../content?spaceKey=X&type=page&expand=version&limit...`
    *   Update the `result` struct to parse the version:
        ```go
        Version struct {
            Number int `json:"number"`
        } `json:"version"`
        ```
    *   Assign the parsed `r.Version.Number` to the constructed `scraper.Page`.

### 3. Crawl Logic Update
*   **File**: `internal/scraper/scraper.go`
*   **Change in `CrawlSpace` loop**:
    *   Before calling `ScrapePageAPI`, query the database for the existing page version: `existingPage, err := s.db.GetPage(s.ctx, pageInfo.SpaceKey, pg.ConfluenceID)`.
    *   If `err == nil` AND `existingPage.Version >= pg.Version`:
        *   Skip extraction (`continue`).
        *   Increment a new `SkippedUnchanged` counter in `CrawlStats`.
    *   Otherwise, proceed with `ScrapePageAPI`.

### 4. Forcing a Full Re-Crawl (Erasing Data)
*   To force the scraper to download all pages again, the database must be wiped for that space.
*   **File**: `internal/api/crawl.go` or `internal/api/spaces.go`
*   **Change**: Add a new API endpoint `DELETE /api/spaces/{key}/data` (or update existing `DELETE /api/spaces/{key}`).
*   **Implementation**: Execute `DELETE FROM pages WHERE space_id = (SELECT id FROM spaces WHERE key = $1)`.
*   **Alternative (CLI)**: Provide a command: `docker compose exec app /app/cli crawl --force "URL"`. The CLI would hit an endpoint that clears the DB before initiating the crawl. 
*   **Current immediate workaround**: Use docker exec directly:
    `docker compose exec -it db psql -U spacemosquito -d spacemosquito -c "DELETE FROM pages WHERE space_id = (SELECT id FROM spaces WHERE key = 'SK');"`

## Open Questions & Resolutions

1.  **Headless Browser Fallback Compatibility**:
    *   *Conflict*: If the system falls back to `discoverSpaceWeb` (DOM parsing), it cannot extract the `Version` number from the sidebar UI without downloading the page.
    *   *Resolution*: DOM parsing fallback will always re-download the full page. Any `Page` struct with `Version = 0` (unknown) must be crawled and treated as changed.
2.  **Asset Handling**:
    *   *Concern*: If a page text hasn't changed, but an attached image was replaced, the `version.number` usually increments. Relying *only* on `version.number` is safe for 99% of Confluence updates.
    *   *Resolution*: Approved. Relying on the page version number is sufficient for asset invalidation.
3.  **Deleted Pages**:
    *   *Concern*: If a page is deleted on Confluence, it disappears from the discovery list, leaving a stale orphan in our local database.
    *   *Resolution*: Implement a "mark as deleted" or sweep logic at the end of the crawl. **CRITICAL Rule**: This logic must *only* delete records from our local PostgreSQL database (e.g., `DELETE FROM pages WHERE space_id = X AND updated_at < crawlStart`). It must **never** call the Confluence API to delete a page on the remote server.
