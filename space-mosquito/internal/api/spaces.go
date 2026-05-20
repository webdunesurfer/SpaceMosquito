package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/vkh/spacemosquito/internal/db"
	"github.com/vkh/spacemosquito/internal/session"
	"github.com/vkh/spacemosquito/pkg/logging"
)

type spacesHandler struct {
	log logging.Sugar
	db  *db.DB
}

type addSpaceRequest struct {
	URL string `json:"url"`
}

// SpacesHandler handles space management endpoints.
func SpacesHandler(db *db.DB, log logging.Sugar) http.HandlerFunc {
	return (&spacesHandler{log: log, db: db}).handle
}

func (h *spacesHandler) handle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		h.list(w, r)
	case "POST":
		h.add(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *spacesHandler) list(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	spaces, err := h.db.ListSpaces(ctx)
	if err != nil {
		h.log.Errorw("list spaces failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list spaces"})
		return
	}

	result := make([]map[string]interface{}, 0, len(spaces))
	for _, s := range spaces {
		var pageCount int
		var lastCrawledStr string
		if s.LastCrawled != nil {
			lastCrawledStr = s.LastCrawled.Format(time.RFC3339)
		}
		err := h.db.Pool().QueryRow(ctx, "SELECT COUNT(*) FROM pages WHERE space_id = $1", s.ID).Scan(&pageCount)
		if err != nil {
			h.log.Warnw("count pages failed", "space_key", s.Key, "error", err)
			pageCount = 0
		}

		result = append(result, map[string]interface{}{
			"space_key":     s.Key,
			"space_name":    s.Name,
			"space_url":     s.URL,
			"pages_crawled": pageCount,
			"last_crawled":  lastCrawledStr,
			"created_at":    s.CreatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *spacesHandler) add(w http.ResponseWriter, r *http.Request) {
	var req addSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}

	spaceKey := session.GetSpaceKeyFromURL(req.URL)
	spaceName := session.GetSpaceNameFromURL(req.URL)

	if spaceKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid confluence URL: cannot extract space key"})
		return
	}

	ctx := context.Background()
	id, err := h.db.CreateSpace(ctx, spaceKey, spaceName, req.URL)
	if err != nil {
		h.log.Errorw("create space failed", "space_key", spaceKey, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create space"})
		return
	}

	h.log.Infow("space added", "id", id, "key", spaceKey, "name", spaceName)
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":   "space added",
		"space_key": spaceKey,
		"space_url": req.URL,
	})
}

// SpaceByIDHandler handles individual space endpoints.
func SpaceByIDHandler(db *db.DB, log logging.Sugar) http.HandlerFunc {
	return (&spaceByIDHandler{log: log, db: db}).handle
}

type spaceByIDHandler struct {
	log logging.Sugar
	db  *db.DB
}

func (h *spaceByIDHandler) handle(w http.ResponseWriter, r *http.Request) {
	spaceKey := r.PathValue("key")
	if spaceKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "space key is required"})
		return
	}

	switch r.Method {
	case "GET":
		h.getStatus(w, r, spaceKey)
	case "DELETE":
		h.delete(w, r, spaceKey)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *spaceByIDHandler) getStatus(w http.ResponseWriter, r *http.Request, spaceKey string) {
	ctx := context.Background()
	space, err := h.db.GetSpaceByKey(ctx, spaceKey)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "space not found"})
		return
	}

	pages, _ := h.db.ListPages(ctx, spaceKey, 0)

	var lastCrawledStr string
	if space.LastCrawled != nil {
		lastCrawledStr = space.LastCrawled.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"space_key":     space.Key,
		"space_name":    space.Name,
		"space_url":     space.URL,
		"pages_crawled": len(pages),
		"last_crawled":  lastCrawledStr,
	})
}

func (h *spaceByIDHandler) delete(w http.ResponseWriter, r *http.Request, spaceKey string) {
	ctx := context.Background()
	space, err := h.db.GetSpaceByKey(ctx, spaceKey)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "space not found"})
		return
	}

	// Delete all pages for this space
	_, err = h.db.Pool().Exec(ctx, "DELETE FROM pages WHERE space_id = $1", space.ID)
	if err != nil {
		h.log.Errorw("delete pages failed", "space_key", spaceKey, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete pages"})
		return
	}

	// Delete the space
	_, err = h.db.Pool().Exec(ctx, "DELETE FROM spaces WHERE id = $1", space.ID)
	if err != nil {
		h.log.Errorw("delete space failed", "space_key", spaceKey, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete space"})
		return
	}

	h.log.Infow("space deleted", "space_key", spaceKey)
	writeJSON(w, http.StatusOK, map[string]string{"message": "space deleted"})
}
