package scraper

import (
	"testing"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/pkg/logging"
)

func testJobManager(t *testing.T) *CrawlJobManager {
	t.Helper()
	return NewJobManager(&config.Config{}, nil, nil, nil, nil, logging.Sugar{})
}

func TestCrawlJobManager_CreateAndGet(t *testing.T) {
	m := testJobManager(t)
	job, err := m.CreateJob("https://example.atlassian.net/wiki/spaces/PROJ")
	if err != nil {
		t.Fatal(err)
	}
	if job.ID == "" {
		t.Fatal("expected job ID")
	}
	if job.Status != JobStatusPending {
		t.Errorf("status = %q", job.Status)
	}

	got, err := m.GetJob(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SpaceURL != job.SpaceURL {
		t.Errorf("SpaceURL = %q", got.SpaceURL)
	}
}

func TestCrawlJobManager_GetJob_notFound(t *testing.T) {
	m := testJobManager(t)
	_, err := m.GetJob("missing")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCrawlJobManager_ListJobs_counts(t *testing.T) {
	m := testJobManager(t)
	j1, _ := m.CreateJob("https://a")
	j2, _ := m.CreateJob("https://b")

	m.mu.Lock()
	m.jobs[j1.ID].Status = JobStatusRunning
	m.jobs[j2.ID].Status = JobStatusCompleted
	m.mu.Unlock()

	snap := m.ListJobs()
	if snap.Total != 2 || snap.Running != 1 || snap.Completed != 1 {
		t.Fatalf("snapshot = %+v", snap)
	}
}

func TestCrawlJobManager_CancelJob(t *testing.T) {
	m := testJobManager(t)
	job, _ := m.CreateJob("https://a")

	if err := m.CancelJob(job.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := m.GetJob(job.ID)
	if got.Status != JobStatusCancelled {
		t.Errorf("status = %q", got.Status)
	}

	if err := m.CancelJob("missing"); err == nil {
		t.Fatal("expected error for missing job")
	}

	m.jobs[job.ID].Status = JobStatusCompleted
	if err := m.CancelJob(job.ID); err == nil {
		t.Fatal("expected error cancelling completed job")
	}
}

func TestCrawlJobManager_Cleanup(t *testing.T) {
	m := testJobManager(t)
	done, _ := m.CreateJob("https://done")
	pending, _ := m.CreateJob("https://pending")

	m.mu.Lock()
	m.jobs[done.ID].Status = JobStatusCompleted
	m.mu.Unlock()

	m.Cleanup()
	if _, err := m.GetJob(done.ID); err == nil {
		t.Fatal("completed job should be removed")
	}
	if _, err := m.GetJob(pending.ID); err != nil {
		t.Fatal("pending job should remain")
	}
}

func TestCrawlJobManager_RunJob_notPending(t *testing.T) {
	m := testJobManager(t)
	job, _ := m.CreateJob("https://a")
	m.mu.Lock()
	m.jobs[job.ID].Status = JobStatusRunning
	m.mu.Unlock()

	err := m.RunJob(t.Context(), job.ID)
	if err == nil {
		t.Fatal("expected error when job is not pending")
	}
}
