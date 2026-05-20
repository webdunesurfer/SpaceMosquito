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

	if err := scraper.LaunchBrowser(); err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}
	defer scraper.CloseBrowser()

	// Load session
	encKey := r.manager.cfg.Session.EncryptionKey
	if encKey == "" {
		return fmt.Errorf("encryption key not configured")
	}

	sess, err := r.manager.store.Load(encKey)
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}

	if err := scraper.SetupContextWithSession(sess); err != nil {
		return fmt.Errorf("setup session: %w", err)
	}

	// Discover and crawl
	pageInfo, err := scraper.discoverSpace(job.SpaceURL)
	if err != nil {
		return fmt.Errorf("discover space: %w", err)
	}

	job.TotalPages = len(pageInfo.Pages)
	now := time.Now()
	job.UpdatedAt = now

	for i, pg := range pageInfo.Pages {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := scraper.ScrapePage(pg, pageInfo.SpaceKey, pageInfo.SpaceURL); err != nil {
			job.Failed++
			if r.log.Enabled() {
				r.log.Errorw("page scrape failed",
					"job_id", job.ID,
					"page_id", pg.ConfluenceID,
					"error", err)
			}
			continue
		}

		job.Completed++
		job.Progress = int(float64(job.Completed) / float64(job.TotalPages) * 100)
		now := time.Now()
		job.UpdatedAt = now

		if r.log.Enabled() {
			r.log.Infow("page crawled",
				"job_id", job.ID,
				"page", i+1,
				"total", job.TotalPages,
				"completed", job.Completed)
		}
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
