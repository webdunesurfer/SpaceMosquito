package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/scraper"
	"github.com/vkh/spacemosquito/pkg/logging"
)

type CrawlHandler struct {
	manager *scraper.CrawlJobManager
	cfg     *config.Config
	log     logging.Sugar
}

type CreateCrawlRequest struct {
	SpaceURL string `json:"space_url"`
}

func NewCrawlHandler(
	jobManager *scraper.CrawlJobManager,
	cfg *config.Config,
	log logging.Sugar,
) *CrawlHandler {
	return &CrawlHandler{
		manager: jobManager,
		cfg:     cfg,
		log:     log,
	}
}

func (h *CrawlHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req CreateCrawlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.SpaceURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "space_url is required"})
		return
	}

	job, err := h.manager.CreateJob(req.SpaceURL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Start the job asynchronously with a background context
	go func() {
		h.manager.RunJob(context.Background(), job.ID)
	}()

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"message":  "crawl job created",
		"job_id":   job.ID,
		"status":   job.Status,
		"space_url": job.SpaceURL,
	})
}

func (h *CrawlHandler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id query parameter is required"})
		return
	}

	job, err := h.manager.GetJob(jobID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, job)
}

func (h *CrawlHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	snapshot := h.manager.ListJobs()
	writeJSON(w, http.StatusOK, snapshot)
}

func (h *CrawlHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id query parameter is required"})
		return
	}

	if err := h.manager.CancelJob(jobID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "job cancelled"})
}

func (h *CrawlHandler) Cleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	h.manager.Cleanup()
	writeJSON(w, http.StatusOK, map[string]string{"message": "completed jobs cleaned up"})
}
