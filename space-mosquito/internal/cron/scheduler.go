package cron

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/db"
	"github.com/vkh/spacemosquito/internal/scraper"
	"github.com/vkh/spacemosquito/internal/session"
	"github.com/vkh/spacemosquito/internal/storage"
	"github.com/vkh/spacemosquito/pkg/logging"
)

// Scheduler manages scheduled crawl jobs.
type Scheduler struct {
	scheduler gocron.Scheduler
	jobs      map[string]gocron.Job
	mu        sync.RWMutex
	log       logging.Sugar
	cfg       *config.Config
	man       *Manager
	db        *db.DB
	store     *session.Store
	storage   *storage.Writer
	assets    *storage.AssetDownloader
}

// JobInfo holds information about a cron job.
type JobInfo struct {
	ID       string
	NextRun  time.Time
	LastRun  time.Time
	Disabled bool
}

// NewScheduler creates a new cron scheduler.
func NewScheduler(
	cfg *config.Config,
	man *Manager,
	database *db.DB,
	store *session.Store,
	storageWriter *storage.Writer,
	assetDownloader *storage.AssetDownloader,
	log logging.Sugar,
) *Scheduler {
	return &Scheduler{
		jobs:      make(map[string]gocron.Job),
		log:       log,
		cfg:       cfg,
		man:       man,
		db:        database,
		store:     store,
		storage:   storageWriter,
		assets:    assetDownloader,
	}
}

// Start initializes and starts all configured cron jobs.
// Reads YAML config for space lists + defaults, then applies JSON overrides.
func (s *Scheduler) Start(ctx context.Context) error {
	if s.scheduler != nil {
		s.scheduler.Shutdown()
	}

	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return fmt.Errorf("create scheduler: %w", err)
	}
	s.scheduler = scheduler

	if s.cfg.Cron.FullCrawl != nil && s.cfg.Cron.FullCrawl.Enabled {
		s.log.Infow("starting full crawl scheduler",
			"default_interval", s.cfg.Cron.FullCrawl.Interval,
			"spaces", len(s.cfg.Cron.FullCrawl.Spaces))
		if err := s.startFullCrawl(ctx); err != nil {
			s.log.Errorw("failed to start full crawl scheduler", "error", err)
		}
	}
	if s.cfg.Cron.Incremental != nil && s.cfg.Cron.Incremental.Enabled {
		s.log.Infow("starting incremental scheduler",
			"default_interval", s.cfg.Cron.Incremental.Interval,
			"detection", s.cfg.Cron.Incremental.Detection,
			"spaces", len(s.cfg.Cron.Incremental.Spaces))
		if err := s.startIncremental(ctx); err != nil {
			s.log.Errorw("failed to start incremental scheduler", "error", err)
		}
	}

	s.scheduler.Start()
	s.log.Info("cron scheduler started")
	return nil
}

// Restart gracefully shuts down and re-creates the scheduler with current configs.
func (s *Scheduler) Restart(ctx context.Context) error {
	s.scheduler.Shutdown()
	s.mu.Lock()
	s.jobs = make(map[string]gocron.Job)
	s.mu.Unlock()
	return s.Start(ctx)
}

// Stop gracefully shuts down the scheduler.
func (s *Scheduler) Stop() {
	s.scheduler.Shutdown()
	s.log.Info("cron scheduler stopped")
}

// StartNow triggers all registered jobs immediately.
func (s *Scheduler) StartNow() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for id, job := range s.jobs {
		if err := job.RunNow(); err != nil {
			s.log.Warnw("failed to trigger job now", "job_id", id, "error", err)
		} else {
			s.log.Infow("triggered job now", "job_id", id)
		}
	}
}

// ListJobs returns all configured cron jobs.
func (s *Scheduler) ListJobs() []JobInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var jobs []JobInfo
	for id := range s.jobs {
		jobs = append(jobs, JobInfo{ID: id})
	}
	return jobs
}

// Config returns the per-space cron config manager.
func (s *Scheduler) Config() *Manager {
	return s.man
}

func (s *Scheduler) startFullCrawl(ctx context.Context) error {
	cfg := s.cfg.Cron.FullCrawl
	if cfg == nil {
		return nil
	}

	for _, spaceURL := range cfg.Spaces {
		jobID := fmt.Sprintf("full-crawl-%s", sanitizeSpaceKey(spaceURL))
		spaceKey := session.GetSpaceKeyFromURL(spaceURL)

		// Check for per-space JSON override
		ov := s.man.GetOverride(spaceKey)
		var interval time.Duration
		var maxDur time.Duration
		var finalURL string

		if ov != nil && ov.FullCrawl {
			interval, _ = time.ParseDuration(ov.FullCrawlInterval)
			maxDur, _ = cfg.ParseMaxDuration()
			finalURL = ov.SpaceURL
			s.log.Infow("registering full crawl job (JSON override)",
				"job_id", jobID, "space", spaceKey,
				"interval", ov.FullCrawlInterval)
		} else {
			var err error
			interval, err = time.ParseDuration(cfg.Interval)
			if err != nil {
				s.log.Warnw("parse interval failed, using default", "error", err)
				interval, _ = time.ParseDuration("24h")
			}
			maxDur, _ = cfg.ParseMaxDuration()
			finalURL = spaceURL
			s.log.Infow("registering full crawl job",
				"job_id", jobID, "space", spaceKey,
				"interval", interval)
		}

		job, err := s.scheduler.NewJob(
			gocron.DurationJob(interval),
			gocron.NewTask(
				func(jID, sKey, sURL string, maxD time.Duration) func() {
					return func() {
						s.runFullCrawl(ctx, jID, sKey, sURL, maxD)
					}
				}(jobID, spaceKey, finalURL, maxDur),
			),
		)
		if err != nil {
			return fmt.Errorf("create job %s: %w", jobID, err)
		}

		s.mu.Lock()
		s.jobs[jobID] = job
		s.mu.Unlock()
	}

	return nil
}

func (s *Scheduler) startIncremental(ctx context.Context) error {
	cfg := s.cfg.Cron.Incremental
	if cfg == nil {
		return nil
	}

	for _, spaceURL := range cfg.Spaces {
		jobID := fmt.Sprintf("incremental-%s", sanitizeSpaceKey(spaceURL))
		spaceKey := session.GetSpaceKeyFromURL(spaceURL)

		// Check for per-space JSON override
		ov := s.man.GetOverride(spaceKey)
		var interval time.Duration
		var maxDur time.Duration
		var detection string
		var finalURL string

		if ov != nil && ov.IncrCrawl {
			interval, _ = time.ParseDuration(ov.IncrCrawlInterval)
			maxDur, _ = cfg.ParseMaxDuration()
			detection = ov.Detection
			finalURL = ov.SpaceURL
			s.log.Infow("registering incremental job (JSON override)",
				"job_id", jobID, "space", spaceKey,
				"interval", ov.IncrCrawlInterval, "detection", detection)
		} else {
			var err error
			interval, err = time.ParseDuration(cfg.Interval)
			if err != nil {
				s.log.Warnw("parse interval failed, using default", "error", err)
				interval, _ = time.ParseDuration("2h")
			}
			maxDur, _ = cfg.ParseMaxDuration()
			detection = cfg.Detection
			finalURL = spaceURL
			s.log.Infow("registering incremental job",
				"job_id", jobID, "space", spaceKey,
				"interval", interval, "detection", detection)
		}

		job, err := s.scheduler.NewJob(
			gocron.DurationJob(interval),
			gocron.NewTask(
				func(jID, sKey, sURL string, maxD time.Duration, det string) func() {
					return func() {
						s.runIncremental(ctx, jID, sKey, sURL, maxD, det)
					}
				}(jobID, spaceKey, finalURL, maxDur, detection),
			),
		)
		if err != nil {
			return fmt.Errorf("create job %s: %w", jobID, err)
		}

		s.mu.Lock()
		s.jobs[jobID] = job
		s.mu.Unlock()
	}

	return nil
}

func (s *Scheduler) runFullCrawl(ctx context.Context, jobID, spaceKey, spaceURL string, maxDuration time.Duration) {
	s.log.Infow("full crawl started", "job_id", jobID, "space", spaceKey)

	start := time.Now()

	encKey := s.cfg.Session.EncryptionKey
	if encKey == "" {
		s.log.Errorw("encryption key not configured, skipping", "job_id", jobID)
		return
	}

	sess, err := s.store.Load(encKey)
	if err != nil {
		s.log.Errorw("failed to load session", "job_id", jobID, "error", err)
		return
	}

	scr := scraper.New(s.cfg, s.db, s.storage, s.assets, s.log)
	if err := scr.LaunchBrowser(); err != nil {
		s.log.Errorw("failed to launch browser", "job_id", jobID, "error", err)
		return
	}
	defer scr.CloseBrowser()

	if err := scr.SetupContextWithSession(sess); err != nil {
		s.log.Errorw("failed to setup session", "job_id", jobID, "error", err)
		return
	}

	done := make(chan struct{})
	var crawlErr error

	go func() {
		crawlErr = scr.CrawlSpace(spaceURL, sess)
		close(done)
	}()

	select {
	case <-done:
		if crawlErr != nil {
			s.log.Errorw("full crawl failed", "job_id", jobID, "error", crawlErr)
		}
	case <-time.After(maxDuration):
		s.log.Errorw("full crawl timed out", "job_id", jobID, "max_duration", maxDuration)
	}

	duration := time.Since(start)
	s.log.Infow("full crawl completed",
		"job_id", jobID,
		"space", spaceKey,
		"duration", duration.Round(time.Millisecond))
}

func (s *Scheduler) runIncremental(ctx context.Context, jobID, spaceKey, spaceURL string, maxDuration time.Duration, detection string) {
	s.log.Infow("incremental scan started", "job_id", jobID, "space", spaceKey, "detection", detection)

	start := time.Now()

	encKey := s.cfg.Session.EncryptionKey
	if encKey == "" {
		s.log.Errorw("encryption key not configured, skipping", "job_id", jobID)
		return
	}

	sess, err := s.store.Load(encKey)
	if err != nil {
		s.log.Errorw("failed to load session", "job_id", jobID, "error", err)
		return
	}

	if _, err := s.db.GetSpaceByKey(ctx, spaceKey); err != nil {
		s.log.Errorw("space not found", "job_id", jobID, "space_key", spaceKey)
		return
	}

	pages, err := s.db.ListPages(ctx, spaceKey, 0)
	if err != nil {
		s.log.Errorw("failed to list pages", "job_id", jobID, "error", err)
		return
	}

	if len(pages) == 0 {
		s.log.Infow("no pages to check, skipping incremental scan", "job_id", jobID)
		return
	}

	scr := scraper.New(s.cfg, s.db, s.storage, s.assets, s.log)
	if err := scr.LaunchBrowser(); err != nil {
		s.log.Errorw("failed to launch browser", "job_id", jobID, "error", err)
		return
	}
	defer scr.CloseBrowser()

	if err := scr.SetupContextWithSession(sess); err != nil {
		s.log.Errorw("failed to setup session", "job_id", jobID, "error", err)
		return
	}

	changed := 0
	skipped := 0

	for idx, page := range pages {
		select {
		case <-ctx.Done():
			s.log.Errorw("scan cancelled", "job_id", jobID, "error", ctx.Err())
			return
		default:
		}

		pageURL := fmt.Sprintf("https://teamnetconomy.atlassian.net/wiki/spaces/%s/pages/%d", spaceKey, page.ConfluenceID)

		var isChanged bool
		switch detection {
		case "api":
			isChanged = true
		case "dom":
			isChanged = s.checkPageChangedDOM(scr, pageURL, page.UpdatedAt)
		default:
			isChanged = true
		}

		if !isChanged {
			skipped++
			continue
		}

		pg := &scraper.Page{
			ConfluenceID: page.ConfluenceID,
			Title:        page.Title,
			URL:          pageURL,
			Level:        0,
		}

		// Try API scraping first
		err := scr.ScrapePageAPI(pg, spaceKey, spaceURL, sess)
		if err != nil {
			s.log.Warnw("API page scrape failed, falling back to browser", 
				"job_id", jobID, "page_id", page.ConfluenceID, "error", err)

			// Fallback to browser
			if scr.Browser() == nil {
				if err := scr.LaunchBrowser(); err != nil {
					s.log.Errorw("failed to launch browser for fallback", "error", err)
					continue
				}
				if err := scr.SetupContextWithSession(sess); err != nil {
					s.log.Errorw("failed to setup browser context", "error", err)
					continue
				}
			}

			if err := scr.ScrapePage(pg, spaceKey, spaceURL); err != nil {
				s.log.Warnw("browser page scrape failed",
					"job_id", jobID,
					"page_id", page.ConfluenceID,
					"title", page.Title,
					"error", err)
				continue
			}
		}

		changed++

		if (idx+1)%10 == 0 {
			s.log.Infow("incremental progress",
				"job_id", jobID,
				"pages_checked", idx+1,
				"total", len(pages),
				"changed", changed,
				"skipped", skipped)
		}
	}

	duration := time.Since(start)
	s.log.Infow("incremental scan completed",
		"job_id", jobID,
		"pages_checked", len(pages),
		"pages_changed", changed,
		"pages_skipped", skipped,
		"duration", duration.Round(time.Millisecond))
}

func (s *Scheduler) checkPageChangedDOM(scr *scraper.Scraper, pageURL string, updatedAt time.Time) bool {
	return true // conservative: always re-crawl
}

func sanitizeSpaceKey(url string) string {
	key := session.GetSpaceKeyFromURL(url)
	if key == "" {
		key = "unknown"
	}
	return key
}
