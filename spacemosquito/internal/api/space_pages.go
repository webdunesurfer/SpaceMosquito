package api

import (
	"context"
	"net/http"

	"github.com/vkh/spacemosquito/internal/config"
	"github.com/vkh/spacemosquito/internal/search"
	"github.com/vkh/spacemosquito/internal/store"
	"github.com/vkh/spacemosquito/pkg/logging"
)

type spacePagesHandler struct {
	log logging.Sugar
	db  store.Store
	cfg *config.Config
}

// SpacePagesHandler lists pages in a space with cursor pagination.
func SpacePagesHandler(database store.Store, cfg *config.Config, log logging.Sugar) http.HandlerFunc {
	return (&spacePagesHandler{log: log, db: database, cfg: cfg}).handle
}

func (h *spacePagesHandler) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	spaceKey := r.PathValue("key")
	if spaceKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "space key is required"})
		return
	}

	opts, err := search.ParseListSpaceQuery(
		r.URL.Query().Get("limit"),
		r.URL.Query().Get("after_confluence_id"),
		r.URL.Query().Get("include_content"),
	)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	ctx := context.Background()
	if _, err := h.db.GetSpaceByKey(ctx, spaceKey); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "space not found"})
		return
	}

	exposeInternalIDs := false
	if h.cfg != nil {
		exposeInternalIDs = h.cfg.MCP.ExposeInternalIDs
	}

	if opts.IncludeContent {
		pages, err := h.db.ListPages(ctx, spaceKey, opts.Limit+1, opts.After)
		if err != nil {
			h.log.Errorw("list space pages failed", "space_key", spaceKey, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list pages"})
			return
		}
		writeJSON(w, http.StatusOK, search.BuildListSpaceResultFromPages(spaceKey, pages, opts.Limit, exposeInternalIDs))
		return
	}

	summaries, err := h.db.ListPageSummaries(ctx, spaceKey, opts.Limit+1, opts.After)
	if err != nil {
		h.log.Errorw("list space pages failed", "space_key", spaceKey, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list pages"})
		return
	}
	writeJSON(w, http.StatusOK, search.BuildListSpaceResultFromSummaries(spaceKey, summaries, opts.Limit, exposeInternalIDs))
}
