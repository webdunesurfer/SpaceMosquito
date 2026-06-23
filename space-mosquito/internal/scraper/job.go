package scraper

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/db"
	"github.com/vkh/spacemosquito/internal/session"
	"github.com/vkh/spacemosquito/internal/storage"
	"github.com/vkh/spacemosquito/pkg/logging"
)

type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusRunning    JobStatus = "running"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCancelled  JobStatus = "cancelled"
)

type CrawlJob struct {
	ID          string     `json:"id"`
	SpaceURL    string     `json:"space_url"`
	Status      JobStatus  `json:"status"`
	Progress    int        `json:"progress"`
	TotalPages  int        `json:"total_pages"`
	Completed   int        `json:"completed"`
	Failed      int        `json:"failed"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type JobSnapshot struct {
	Jobs      []CrawlJob `json:"jobs"`
	Total     int        `json:"total"`
	Running   int        `json:"running"`
	Completed int        `json:"completed"`
	Failed    int        `json:"failed"`
	Pending   int        `json:"pending"`
}

type CrawlJobManager struct {
	jobs   map[string]*CrawlJob
	mu     sync.RWMutex
	log    logging.Sugar
	cfg    *config.Config
	db     *db.DB
	store  *session.Store
	storage *storage.Writer
	assets *storage.AssetDownloader
}

type CrawlRunner struct {
	manager *CrawlJobManager
	log     logging.Sugar
}

func NewJobManager(cfg *config.Config, db *db.DB, store *session.Store, storageWriter *storage.Writer, assetDownloader *storage.AssetDownloader, log logging.Sugar) *CrawlJobManager {
	return &CrawlJobManager{
		jobs:      make(map[string]*CrawlJob),
		log:       log,
		cfg:       cfg,
		db:        db,
		store:     store,
		storage:   storageWriter,
		assets:    assetDownloader,
	}
}

func (m *CrawlJobManager) CreateJob(spaceURL string) (*CrawlJob, error) {
	job := &CrawlJob{
		ID:        uuid.New().String(),
		SpaceURL:  spaceURL,
		Status:    JobStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	if m.log.Enabled() {
		m.log.Infow("crawl job created",
			"job_id", job.ID,
			"space_url", spaceURL)
	}

	return job, nil
}

func (m *CrawlJobManager) GetJob(jobID string) (*CrawlJob, error) {
	m.mu.RLock()
	job, exists := m.jobs[jobID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	return job, nil
}

func (m *CrawlJobManager) ListJobs() *JobSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := &JobSnapshot{
		Jobs: make([]CrawlJob, 0, len(m.jobs)),
	}

	for _, job := range m.jobs {
		snapshot.Jobs = append(snapshot.Jobs, *job)
		switch job.Status {
		case JobStatusRunning:
			snapshot.Running++
		case JobStatusCompleted:
			snapshot.Completed++
		case JobStatusFailed:
			snapshot.Failed++
		case JobStatusPending:
			snapshot.Pending++
		}
	}

	snapshot.Total = len(m.jobs)

	return snapshot
}

func (m *CrawlJobManager) CancelJob(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	if job.Status != JobStatusPending && job.Status != JobStatusRunning {
		return fmt.Errorf("cannot cancel job with status: %s", job.Status)
	}

	job.Status = JobStatusCancelled
	job.UpdatedAt = time.Now()

	if m.log.Enabled() {
		m.log.Infow("crawl job cancelled", "job_id", jobID)
	}

	return nil
}

func (m *CrawlJobManager) RunJob(ctx context.Context, jobID string) error {
	m.mu.Lock()
	job, exists := m.jobs[jobID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("job not found: %s", jobID)
	}

	if job.Status != JobStatusPending {
		m.mu.Unlock()
		return fmt.Errorf("job is not pending: %s", job.Status)
	}

	job.Status = JobStatusRunning
	job.StartedAt = &time.Time{}
	now := time.Now()
	job.StartedAt = &now
	job.UpdatedAt = now
	m.mu.Unlock()

	if m.log.Enabled() {
		m.log.Infow("crawl job started",
			"job_id", jobID,
			"space_url", job.SpaceURL)
	}

	runner := &CrawlRunner{
		manager: m,
		log:     m.log,
	}

	defer func() {
		if r := recover(); r != nil {
			if m.log.Enabled() {
				m.log.Errorw("crawl panicked",
					"job_id", jobID,
					"panic", fmt.Sprintf("%v", r))
			}
		}
	}()

	err := runner.Run(ctx, job)

	m.mu.Lock()
	job.UpdatedAt = time.Now()
	if err != nil {
		job.Status = JobStatusFailed
		job.Error = err.Error()
	} else {
		job.Status = JobStatusCompleted
		completedTime := time.Now()
		job.CompletedAt = &completedTime
	}
	m.mu.Unlock()

	if m.log.Enabled() {
		if err != nil {
			m.log.Errorw("crawl job failed",
				"job_id", jobID,
				"error", err)
		} else {
			m.log.Infow("crawl job completed",
				"job_id", jobID,
				"completed", job.Completed,
				"failed", job.Failed)
		}
	}

	return err
}

func (r *CrawlRunner) Run(ctx context.Context, job *CrawlJob) error {
	// Create a new scraper instance for this job
	scraper := New(
		r.manager.cfg,
		r.manager.db,
		r.manager.storage,
		r.manager.assets,
		r.log,
	)
	
	// Initialize context for the scraper to prevent nil pointer dereferences in DB calls
	scraper.ctx = ctx

	// Load session
	encKey := r.manager.cfg.Session.EncryptionKey
	if encKey == "" {
		return fmt.Errorf("encryption key not configured")
	}

	sess, err := r.manager.store.Load(encKey)
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}

	// Discover and crawl
	pageInfo, err := scraper.discoverSpace(job.SpaceURL, sess)
	if err != nil {
		return fmt.Errorf("discover space: %w", err)
	}

	job.TotalPages = len(pageInfo.Pages)
	now := time.Now()
	job.UpdatedAt = now
	if r.manager.log.Enabled() {
		r.manager.log.Infow("discovery complete",
			"job_id", job.ID,
			"total_pages", job.TotalPages)
	}

	for i := 0; i < len(pageInfo.Pages); i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		pg := pageInfo.Pages[i]

		// Check if page needs scraping (Incremental logic)
		if pg.Version > 0 {
			existingPage, err := scraper.db.GetPage(ctx, pageInfo.SpaceKey, pg.ConfluenceID)
			if err == nil && existingPage.Version >= pg.Version {
				if r.log.Enabled() {
					r.log.Infow("skipping unchanged page", "job_id", job.ID, "page_id", pg.ConfluenceID, "version", pg.Version)
				}
				r.manager.mu.Lock()
				job.Completed++
				if job.TotalPages > 0 {
					job.Progress = int(float64(job.Completed) / float64(job.TotalPages) * 100)
				}
				job.UpdatedAt = time.Now()
				r.manager.mu.Unlock()
				continue
			}
		}
		
		// Try API scraping first
		err := scraper.ScrapePageAPI(pg, pageInfo.SpaceKey, pageInfo.SpaceURL, sess)
		if err != nil {
			if r.log.Enabled() {
				r.log.Warnw("API page scrape failed, falling back to browser", 
					"job_id", job.ID, "page_id", pg.ConfluenceID, "error", err)
			}
			
			// Fallback to browser
			if scraper.browser == nil {
				if err := scraper.LaunchBrowser(); err != nil {
					r.manager.mu.Lock()
					job.Failed++
					r.manager.mu.Unlock()
					continue
				}
				if err := scraper.SetupContextWithSession(sess); err != nil {
					r.manager.mu.Lock()
					job.Failed++
					r.manager.mu.Unlock()
					continue
				}
			}
			
			if err := scraper.ScrapePage(pg, pageInfo.SpaceKey, pageInfo.SpaceURL); err != nil {
				r.manager.mu.Lock()
				job.Failed++
				r.manager.mu.Unlock()
				if r.log.Enabled() {
					r.log.Errorw("browser page scrape failed",
						"job_id", job.ID,
						"page_id", pg.ConfluenceID,
						"error", err)
				}
				continue
			}
		}

		r.manager.mu.Lock()
		job.Completed++
		if job.TotalPages > 0 {
			job.Progress = int(float64(job.Completed) / float64(job.TotalPages) * 100)
		}
		job.UpdatedAt = time.Now()
		r.manager.mu.Unlock()

		if r.log.Enabled() {
			r.log.Infow("page crawled",
				"job_id", job.ID,
				"page", i+1,
				"total", job.TotalPages,
				"completed", job.Completed)
		}
	}

	// Update space page count
	if job.Failed == job.TotalPages && job.TotalPages > 0 {
		return fmt.Errorf("all %d pages failed to crawl", job.TotalPages)
	}

	if r.log.Enabled() {
		r.log.Infow("updating space page count",
			"job_id", job.ID,
			"total_pages", job.Completed)
	}

	if err := r.manager.db.UpdateSpaceLastCrawled(ctx, pageInfo.SpaceKey); err != nil {
		r.log.Warnw("failed to update space last crawled",
			"job_id", job.ID,
			"error", err)
	}

	return nil
}

func (m *CrawlJobManager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, job := range m.jobs {
		if job.Status == JobStatusCompleted || job.Status == JobStatusFailed || job.Status == JobStatusCancelled {
			delete(m.jobs, id)
		}
	}

	if m.log.Enabled() {
		m.log.Infow("crawl jobs cleaned up", "remaining", len(m.jobs))
	}
}
