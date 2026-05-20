package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/cron"
	"github.com/vkh/spacemosquito/pkg/logging"
)

type cronConfigHandler struct {
	log     logging.Sugar
	cfg     *config.Config
	manager *cron.Manager
	sched   *cron.Scheduler
}

// CronConfigHandler handles cron configuration endpoints.
func CronConfigHandler(cfg *config.Config, log logging.Sugar) http.HandlerFunc {
	// We'll wire the manager and scheduler separately
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"message": "placeholder"})
	}
}

// CronAPIHandler handles cron config + reload endpoints.
type CronAPIHandler struct {
	cfg     *config.Config
	log     logging.Sugar
	manager *cron.Manager
	sched   *cron.Scheduler
}

// NewCronAPIHandler creates a cron config handler.
func NewCronAPIHandler(cfg *config.Config, manager *cron.Manager, sched *cron.Scheduler, log logging.Sugar) *CronAPIHandler {
	return &CronAPIHandler{
		cfg:     cfg,
		log:     log,
		manager: manager,
		sched:   sched,
	}
}

// List handles GET /api/cron — list cron jobs.
func (h *CronAPIHandler) List(w http.ResponseWriter, r *http.Request) {
	jobs := h.sched.ListJobs()
	writeJSON(w, http.StatusOK, jobs)
}

// StartNow handles POST /api/cron/start — trigger all jobs immediately.
func (h *CronAPIHandler) StartNow(w http.ResponseWriter, r *http.Request) {
	h.sched.StartNow()
	writeJSON(w, http.StatusOK, map[string]string{"status": "jobs triggered"})
}

// ConfigGet handles GET /api/cron/config — return current cron configuration.
func (h *CronAPIHandler) ConfigGet(w http.ResponseWriter, r *http.Request) {
	overrides := h.manager.List()

	result := map[string]interface{}{
		"yaml_full_crawl": map[string]interface{}{
			"enabled":    h.cfg.Cron.FullCrawl != nil && h.cfg.Cron.FullCrawl.Enabled,
			"interval":   h.cfg.Cron.FullCrawl.Interval,
			"spaces":     h.cfg.Cron.FullCrawl.Spaces,
			"max_duration": h.cfg.Cron.FullCrawl.MaxDuration,
		},
		"yaml_incremental": map[string]interface{}{
			"enabled":    h.cfg.Cron.Incremental != nil && h.cfg.Cron.Incremental.Enabled,
			"interval":   h.cfg.Cron.Incremental.Interval,
			"detection":  h.cfg.Cron.Incremental.Detection,
			"spaces":     h.cfg.Cron.Incremental.Spaces,
			"max_duration": h.cfg.Cron.Incremental.MaxDuration,
		},
		"per_space_overrides": overrides,
	}

	writeJSON(w, http.StatusOK, result)
}

// ConfigUpdate handles POST /api/cron/config — update per-space cron config.
func (h *CronAPIHandler) ConfigUpdate(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	spaceKey, ok := req["space_key"].(string)
	if !ok || spaceKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "space_key is required"})
		return
	}

	// Build the per-space config
	ov := &cron.PerSpaceCronConfig{
		SpaceKey: spaceKey,
	}

	if v, ok := req["space_url"].(string); ok {
		ov.SpaceURL = v
	}
	if v, ok := req["full_crawl_enabled"].(bool); ok {
		ov.FullCrawl = v
	}
	if v, ok := req["full_crawl_interval"].(string); ok {
		ov.FullCrawlInterval = v
	}
	if v, ok := req["incr_crawl_enabled"].(bool); ok {
		ov.IncrCrawl = v
	}
	if v, ok := req["incr_crawl_interval"].(string); ok {
		ov.IncrCrawlInterval = v
	}
	if v, ok := req["detection"].(string); ok {
		ov.Detection = v
	}

	if err := h.manager.Upsert(*ov); err != nil {
		h.log.Errorw("failed to update cron config", "space_key", spaceKey, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save config"})
		return
	}

	h.log.Infow("cron config updated", "space_key", spaceKey)
	writeJSON(w, http.StatusOK, map[string]string{"message": "cron config updated", "space_key": spaceKey})
}

// ConfigReload handles POST /api/cron/reload — restart scheduler with new config.
func (h *CronAPIHandler) ConfigReload(w http.ResponseWriter, r *http.Request) {
	h.log.Info("reloading cron scheduler")

	ctx := r.Context()
	if err := h.sched.Restart(ctx); err != nil {
		h.log.Errorw("failed to restart scheduler", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to restart scheduler"})
		return
	}

	h.log.Info("cron scheduler restarted")
	writeJSON(w, http.StatusOK, map[string]string{"message": "scheduler restarted with new config"})
}

// CronSpaceHandler handles per-space cron config endpoints.
type CronSpaceHandler struct {
	log     logging.Sugar
	manager *cron.Manager
	sched   *cron.Scheduler
}

// NewCronSpaceHandler creates a per-space cron handler.
func NewCronSpaceHandler(manager *cron.Manager, sched *cron.Scheduler, log logging.Sugar) *CronSpaceHandler {
	return &CronSpaceHandler{
		log:     log,
		manager: manager,
		sched:   sched,
	}
}

// Update handles POST /api/cron/space/:key — update a specific space's cron config.
func (h *CronSpaceHandler) Update(w http.ResponseWriter, r *http.Request) {
	spaceKey := r.PathValue("key")
	if spaceKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "space key is required"})
		return
	}

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Get existing config
	existing := h.manager.GetOverride(spaceKey)
	ov := cron.PerSpaceCronConfig{SpaceKey: spaceKey}
	if existing != nil {
		ov = *existing
	}

	if v, ok := req["full_crawl_enabled"].(bool); ok {
		ov.FullCrawl = v
	}
	if v, ok := req["full_crawl_interval"].(string); ok {
		ov.FullCrawlInterval = v
	}
	if v, ok := req["incr_crawl_enabled"].(bool); ok {
		ov.IncrCrawl = v
	}
	if v, ok := req["incr_crawl_interval"].(string); ok {
		ov.IncrCrawlInterval = v
	}
	if v, ok := req["detection"].(string); ok {
		ov.Detection = v
	}

	if err := h.manager.Upsert(ov); err != nil {
		h.log.Errorw("failed to update space cron config", "space_key", spaceKey, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save config"})
		return
	}

	h.log.Infow("space cron config updated", "space_key", spaceKey)
	writeJSON(w, http.StatusOK, map[string]string{"message": "cron config updated"})
}

// Delete handles DELETE /api/cron/space/:key — remove a space's cron config.
func (h *CronSpaceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	spaceKey := r.PathValue("key")
	if spaceKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "space key is required"})
		return
	}

	if err := h.manager.Delete(spaceKey); err != nil {
		h.log.Errorw("failed to delete space cron config", "space_key", spaceKey, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete config"})
		return
	}

	h.log.Infow("space cron config deleted", "space_key", spaceKey)
	writeJSON(w, http.StatusOK, map[string]string{"message": "cron config deleted"})
}

// SpaceCronConfig returns the per-space cron config.
type SpaceCronConfig struct {
	SpaceKey        string    `json:"space_key"`
	SpaceURL        string    `json:"space_url"`
	FullCrawl       bool      `json:"full_crawl_enabled"`
	FullCrawlInterval string  `json:"full_crawl_interval"`
	IncrCrawl       bool      `json:"incr_crawl_enabled"`
	IncrCrawlInterval string  `json:"incr_crawl_interval"`
	Detection       string    `json:"detection"`
	Updated         time.Time `json:"updated"`
}

// ConfigGet returns the per-space cron config for a given space.
func (h *CronSpaceHandler) ConfigGet(w http.ResponseWriter, r *http.Request) {
	spaceKey := r.PathValue("key")
	if spaceKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "space key is required"})
		return
	}

	ov := h.manager.GetOverride(spaceKey)
	if ov == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"space_key":           spaceKey,
			"full_crawl_enabled":  false,
			"incr_crawl_enabled":  false,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"space_key":           ov.SpaceKey,
		"space_url":           ov.SpaceURL,
		"full_crawl_enabled":  ov.FullCrawl,
		"full_crawl_interval": ov.FullCrawlInterval,
		"incr_crawl_enabled":  ov.IncrCrawl,
		"incr_crawl_interval": ov.IncrCrawlInterval,
		"detection":           ov.Detection,
	})
}
