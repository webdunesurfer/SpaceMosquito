# Phase 9: Cron Scheduler

## Objective
Implement scheduled full-space and incremental scans using Go's gocron package for automated periodic re-crawling.

## Deliverables
- Go-based cron scheduler with gocron
- Full-space scan job: re-crawl entire space
- Incremental scan job: only fetch changed pages
- Job configuration via YAML config
- Comprehensive logging for all scheduled operations

## Logging Strategy
- Use `logging.Sugar` injected via scheduler constructor
- Log all job lifecycle events: schedule, start, pause, complete, error, skip
- Include job_id, space_key, job_type in all log entries
- Log detailed per-job statistics: pages crawled, errors, duration, bytes transferred
- Use `zapexp.OtelObserver()` for future OpenTelemetry integration (already configured in pkg/logger)

## Tasks

### 9.1 — Cron Scheduler Setup
- `internal/cron/scheduler.go`:
  ```go
  type Scheduler struct {
      scheduler *gocron.Scheduler
      jobs      map[string]*Job
      log       logging.Sugar
  }

  func NewScheduler(config CronConfig, log logging.Sugar) *Scheduler
  func (s *Scheduler) Start()
  func (s *Scheduler) Stop()
  func (s *Scheduler) AddFullCrawlJob(spaceURL, interval string) error
  func (s *Scheduler) AddIncrementalJob(spaceURL, interval string) error
  ```
- gocron integration:
  - `github.com/go-co-op/gocron` for scheduling
  - Jobs run in goroutines
  - Graceful shutdown on SIGTERM
  - **Log scheduler start/stop, job registration, interval configuration**

### 9.2 — Full-Space Scan Job
- `internal/cron/job.go`:
  - `FullCrawlJob`:
    - Runs the complete scraper (Phase 3) from scratch
    - Discovers all pages, extracts content, downloads assets
    - Overwrites existing local files with updated content
    - Updates database records
    - Triggers re-embedding of all pages
  - Trigger: user-specified interval (e.g., "every 6 hours", "daily at 2am")
  - Config:
    ```yaml
    cron:
      full_crawl:
        enabled: true
        interval: "24h"
        spaces:
          - "https://company.atlassian.net/wiki/spaces/PROJ"
    ```
  - **Log job start with timestamp, space_key, pages discovered, assets downloaded, embeddings generated**

### 9.3 — Incremental Scan Job
- `internal/cron/job.go`:
  - `IncrementalJob`:
    - For each tracked space, query Confluence for pages modified after `last_crawled`
    - Use Confluence REST API or page metadata (modified date in DOM) to detect changes
    - Only scrape pages that have been updated since last crawl
    - Update embeddings only for changed pages
    - Significantly faster than full crawl for large spaces
  - Detection methods:
    1. Confluence REST API `/rest/api/content/search` with `expand=version`
    2. DOM-based: parse `#page-info` or similar element for "Last modified" date
    3. Fallback: scan all pages if change detection unavailable
  - Config:
    ```yaml
    incremental:
      enabled: true
      interval: "2h"
      detection: "api"  # "api" | "dom" | "disabled"
    ```
  - **Log change detection: pages checked, pages changed, skipped unchanged pages**

### 9.4 — Job State Management
- `internal/cron/scheduler.go`:
  - Persist job state to prevent duplicate runs:
    - Track `last_run` timestamp per job
    - Skip if previous run hasn't completed
    - Configurable: `max_run_duration` (cancel if stuck)
  - Job logging:
    - Log each run start/end
    - Log pages crawled, errors, time taken
    - Append to log file or structured logs
    - **Log skip reasons (previous run in progress, max duration exceeded, interval not elapsed)**

### 9.5 — CLI Commands
- `cmd/cli/main.go`:
  - `space-mosquito cron list` — list configured cron jobs
  - `space-mosquito cron add --space <url> --interval 24h --type full`
  - `space-mosquito cron remove <job-id>`
  - `space-mosquito cron run --now` — trigger a specific job immediately
  - **Log CLI cron operations with job_id, space_key, action (list/add/remove/run)**

### 9.6 — Notifications
- Job completion logging:
  ```
  [cron] Full crawl started for PROJ: https://company.atlassian.net/wiki/spaces/PROJ
  [cron] Full crawl completed for PROJ: 142 pages, 89 images, 2m34s
  [cron] Incremental scan for PROJ: 3 pages updated, 1m12s
  [cron] Error in full crawl for OPS: failed to authenticate — check session
  ```
- Optional: email/webhook notification on completion (future enhancement)
  - **Log notification events, delivery status**

## Acceptance Criteria
- Scheduler starts with the backend and runs on configured intervals
- Full crawl re-scans the entire space and updates all files
- Incremental scan detects and updates only changed pages
- Jobs log start/end with summary statistics
- CLI commands manage cron configuration
- Scheduler handles errors gracefully (logs, continues)
- All cron operations logged with structured fields (job_id, job_type, space_key, duration, pages_crawled, errors)
